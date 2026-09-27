package handler

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/OstKost/avari-p3-express/apps/api/internal/config"
	"github.com/OstKost/avari-p3-express/apps/api/internal/domain"
	"github.com/OstKost/avari-p3-express/apps/api/internal/service"
	"github.com/go-chi/chi/v5"
)

type actorKey struct{}
type WorkHandler struct {
	service      *service.WorkService
	p3           *service.P3Service
	origins      []string
	mu           sync.Mutex
	sessions     map[string]time.Time
	managerKey   string
	publicOrigin string
}

func NewWorkHandler(s *service.WorkService, p *service.P3Service, origins []string, managerKey string, publicOrigins ...string) *WorkHandler {
	h := &WorkHandler{service: s, p3: p, origins: origins, sessions: map[string]time.Time{}, managerKey: managerKey}
	if len(publicOrigins) == 1 {
		h.publicOrigin, _ = config.NormalizePublicManagerOrigin(publicOrigins[0])
	}
	return h
}
func (h *WorkHandler) publicRequest(r *http.Request) bool {
	if h.publicOrigin == "" || r.Header.Get("Origin") != h.publicOrigin {
		return false
	}
	u, err := url.Parse(h.publicOrigin)
	return err == nil && r.Host == u.Host
}

// Public browser reads may omit Origin; writes always require the exact origin.
func (h *WorkHandler) trustedManagerRequest(r *http.Request) bool {
	if localHost(r.Host) {
		return h.trusted(r)
	}
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		if h.publicOrigin == "" {
			return false
		}
		u, err := url.Parse(h.publicOrigin)
		origin := r.Header.Get("Origin")
		return err == nil && r.Host == u.Host && (origin == "" || origin == h.publicOrigin)
	}
	return h.publicRequest(r)
}
func actor(r *http.Request) domain.WorkActor {
	a, _ := r.Context().Value(actorKey{}).(domain.WorkActor)
	return a
}
func workError(w http.ResponseWriter, e error) {
	switch {
	case errors.Is(e, domain.ErrWorkInput):
		respondError(w, 400, e.Error())
	case errors.Is(e, domain.ErrForbidden):
		respondError(w, 403, "Access denied")
	case errors.Is(e, domain.ErrNotFound):
		respondError(w, 404, "Not found")
	case errors.Is(e, domain.ErrConflict):
		respondError(w, 409, e.Error())
	default:
		respondError(w, 500, "Internal server error")
	}
}
func workBody(w http.ResponseWriter, r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if e := dec.Decode(v); e != nil {
		return domain.ErrWorkInput
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return domain.ErrWorkInput
	}
	return nil
}
func localHost(raw string) bool {
	host := raw
	if h, _, e := net.SplitHostPort(raw); e == nil {
		host = h
	}
	return host == "localhost" || net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback()
}
func (h *WorkHandler) trusted(r *http.Request) bool {
	if !localHost(r.Host) {
		return h.publicRequest(r)
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, e := url.Parse(origin)
	if e != nil || !localHost(u.Host) {
		return false
	}
	for _, o := range h.origins {
		if o == origin {
			return true
		}
	}
	return origin == "http://"+r.Host || origin == "https://"+r.Host
}
func (h *WorkHandler) Session(w http.ResponseWriter, r *http.Request) {
	if !h.trusted(r) || r.Header.Get("X-Avari-Local") != "manager" || r.Header.Get("Authorization") != "" {
		respondError(w, 403, "Local manager request required")
		return
	}
	var credentials struct {
		Key string `json:"key"`
	}
	if e := workBody(w, r, &credentials); e != nil {
		workError(w, e)
		return
	}
	if len(h.managerKey) < 32 || subtle.ConstantTimeCompare([]byte(credentials.Key), []byte(h.managerKey)) != 1 {
		respondError(w, 401, "Invalid manager key")
		return
	}
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		workError(w, e)
		return
	}
	token := hex.EncodeToString(b)
	h.mu.Lock()
	for k, expiry := range h.sessions {
		if time.Now().After(expiry) {
			delete(h.sessions, k)
		}
	}
	h.sessions[token] = time.Now().Add(24 * time.Hour)
	h.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "avari_manager", Value: token, Path: "/api", HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil || h.publicRequest(r), MaxAge: 86400})
	respondJSON(w, 200, map[string]string{"role": "manager"})
}
func (h *WorkHandler) Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var a domain.WorkActor
		auth := r.Header.Get("Authorization")
		if auth != "" {
			if !strings.HasPrefix(auth, "Bearer ") {
				respondError(w, 401, "Authentication required")
				return
			}
			var e error
			a, e = h.service.Authenticate(r.Context(), strings.TrimPrefix(auth, "Bearer "))
			if e != nil {
				respondError(w, 401, "Invalid or revoked token")
				return
			}
		} else {
			cookie, e := r.Cookie("avari_manager")
			h.mu.Lock()
			expiry := time.Time{}
			if e == nil {
				expiry = h.sessions[cookie.Value]
			}
			h.mu.Unlock()
			if e != nil || time.Now().After(expiry) {
				respondError(w, 401, "Manager session required")
				return
			}
			if !h.trustedManagerRequest(r) {
				respondError(w, 403, "Untrusted local origin")
				return
			}
			a = domain.WorkActor{ID: "manager", Name: "Менеджер", Manager: true}
		}
		if !a.Manager && strings.HasPrefix(r.URL.Path, "/api/v1/p3") {
			respondError(w, 403, "Use scoped project context endpoint")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), actorKey{}, a)))
	})
}
func page(r *http.Request, length int) (int, int, error) {
	limit := 50
	offset := 0
	var e error
	if x := r.URL.Query().Get("limit"); x != "" {
		limit, e = strconv.Atoi(x)
		if e != nil || limit < 1 || limit > 100 {
			return 0, 0, domain.ErrWorkInput
		}
	}
	if x := r.URL.Query().Get("offset"); x != "" {
		offset, e = strconv.Atoi(x)
		if e != nil || offset < 0 {
			return 0, 0, domain.ErrWorkInput
		}
	}
	if offset > length {
		offset = length
	}
	end := offset + limit
	if end > length {
		end = length
	}
	return offset, end, nil
}

// WorkList godoc
// @Summary List scoped tasks with filters and pagination
// @Tags Work
// @Produce json
// @Param project_id query string false "Project ID"
// @Param status query string false "Task status"
// @Param limit query int false "Page size (1-100)"
// @Param offset query int false "Offset"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/work-tasks [get]
func (h *WorkHandler) list(w http.ResponseWriter, r *http.Request) {
	ts, e := h.service.List(r.Context(), actor(r))
	if e != nil {
		workError(w, e)
		return
	}
	out := []domain.WorkTask{}
	q := r.URL.Query()
	for _, t := range ts {
		if p := q.Get("project_id"); p != "" && p != t.ProjectID {
			continue
		}
		if s := q.Get("status"); s != "" && s != t.Status {
			continue
		}
		if s := q.Get("stage_id"); s != "" && s != t.StageID {
			continue
		}
		if s := q.Get("step_id"); s != "" && !contains(t.StepIDs, s) {
			continue
		}
		if s := q.Get("assignee"); s != "" && s != t.Assignee {
			continue
		}
		out = append(out, t)
	}
	start, end, e := page(r, len(out))
	if e != nil {
		workError(w, e)
		return
	}
	respondJSON(w, 200, map[string]any{"data": out[start:end], "total": len(out)})
}
func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// WorkCreate godoc
// @Summary Create a draft production task (manager session)
// @Tags Work
// @Accept json
// @Produce json
// @Success 201 {object} domain.WorkTask
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /api/v1/work-tasks [post]
func (h *WorkHandler) create(w http.ResponseWriter, r *http.Request) {
	var v struct {
		ProjectID string `json:"project_id"`
		domain.WorkSpec
	}
	if e := workBody(w, r, &v); e != nil {
		workError(w, e)
		return
	}
	t, e := h.service.Create(r.Context(), actor(r), v.ProjectID, v.WorkSpec)
	if e != nil {
		workError(w, e)
		return
	}
	respondJSON(w, 201, t)
}

// WorkGet godoc
// @Summary Task, project, dependencies and exact cycle context
// @Tags Work
// @Produce json
// @Param id path string true "Task ID"
// @Success 200 {object} map[string]interface{}
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/work-tasks/{id} [get]
func (h *WorkHandler) get(w http.ResponseWriter, r *http.Request) {
	t, e := h.service.Get(r.Context(), actor(r), chi.URLParam(r, "id"))
	if e != nil {
		workError(w, e)
		return
	}
	p, e := h.p3.GetProject(r.Context(), t.ProjectID)
	if e != nil {
		workError(w, e)
		return
	}
	deps, steps, e := h.service.Context(r.Context(), actor(r), t.ID)
	if e != nil {
		workError(w, e)
		return
	}
	respondJSON(w, 200, map[string]any{"task": t, "project": p, "dependencies": deps, "linked_steps": steps})
}

// WorkCommand godoc
// @Summary Apply versioned task operation (role and state checked by service)
// @Tags Work
// @Accept json
// @Produce json
// @Param id path string true "Task ID"
// @Param op path string true "Operation" Enums(edit,ready,start,progress,comment,blocker,result,submit,finish,review,cancel)
// @Param body body service.WorkCommand true "Version, attempt and operation payload"
// @Success 200 {object} domain.WorkTask
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /api/v1/work-tasks/{id}/{op} [post]
func (h *WorkHandler) command(w http.ResponseWriter, r *http.Request) {
	var c service.WorkCommand
	if e := workBody(w, r, &c); e != nil {
		workError(w, e)
		return
	}
	t, e := h.service.Command(r.Context(), actor(r), chi.URLParam(r, "id"), chi.URLParam(r, "op"), c)
	if e != nil {
		workError(w, e)
		return
	}
	respondJSON(w, 200, t)
}
func (h *WorkHandler) projects(w http.ResponseWriter, r *http.Request) {
	ps, e := h.p3.ListProjects(r.Context())
	if e != nil {
		workError(w, e)
		return
	}
	out := []domain.ProjectSummary{}
	for _, p := range ps {
		if actor(r).Allows(p.ID) {
			out = append(out, p)
		}
	}
	start, end, e := page(r, len(out))
	if e != nil {
		workError(w, e)
		return
	}
	respondJSON(w, 200, map[string]any{"data": out[start:end], "total": len(out)})
}
func (h *WorkHandler) context(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !actor(r).Allows(id) {
		workError(w, domain.ErrForbidden)
		return
	}
	p, e := h.p3.GetProject(r.Context(), id)
	if e != nil {
		workError(w, e)
		return
	}
	respondJSON(w, 200, p)
}

// AgentRuns godoc
// @Summary List scoped execution attempts with pagination
// @Tags Work
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/agent-runs [get]
func (h *WorkHandler) runs(w http.ResponseWriter, r *http.Request) {
	ts, e := h.service.List(r.Context(), actor(r))
	if e != nil {
		workError(w, e)
		return
	}
	out := []domain.AgentRun{}
	for _, t := range ts {
		if p := r.URL.Query().Get("project_id"); p != "" && p != t.ProjectID {
			continue
		}
		for _, run := range t.Runs {
			if id := r.URL.Query().Get("task_id"); id != "" && t.ID != id {
				continue
			}
			out = append(out, run)
		}
	}
	start, end, e := page(r, len(out))
	if e != nil {
		workError(w, e)
		return
	}
	respondJSON(w, 200, map[string]any{"data": out[start:end], "total": len(out)})
}

// TaskResults godoc
// @Summary List submissions filtered by snapshot links and review state
// @Tags Work
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/task-results [get]
func (h *WorkHandler) results(w http.ResponseWriter, r *http.Request) {
	ts, e := h.service.List(r.Context(), actor(r))
	if e != nil {
		workError(w, e)
		return
	}
	out := []map[string]any{}
	q := r.URL.Query()
	for _, t := range ts {
		if p := q.Get("project_id"); p != "" && p != t.ProjectID {
			continue
		}
		if p := q.Get("task_id"); p != "" && p != t.ID {
			continue
		}

		for _, run := range t.Runs {
			if p := q.Get("stage_id"); p != "" && p != run.Snapshot.StageID {
				continue
			}
			if p := q.Get("step_id"); p != "" && !contains(run.Snapshot.StepIDs, p) {
				continue
			}
			if p := q.Get("actor"); p != "" && p != run.Actor {
				continue
			}
			if run.State != "submitted" {
				continue
			}
			state := "pending"
			if t.Status == "cancelled" {
				state = "cancelled"
			}
			if run.Review != nil {
				state = run.Review.Decision
			}
			if p := q.Get("state"); p != "" && p != state {
				continue
			}
			for _, res := range run.Results {
				out = append(out, map[string]any{"result": res, "state": state, "actor": run.Actor, "actor_name": run.ActorName, "review": run.Review, "step_ids": run.Snapshot.StepIDs, "stage_id": run.Snapshot.StageID, "verification_reports": run.VerificationReports, "submission_digest": run.SubmissionDigest})
			}
		}
	}
	start, end, e := page(r, len(out))
	if e != nil {
		workError(w, e)
		return
	}
	respondJSON(w, 200, map[string]any{"data": out[start:end], "total": len(out)})
}
func (h *WorkHandler) tokens(w http.ResponseWriter, r *http.Request) {
	ts, e := h.service.Tokens(r.Context(), actor(r))
	if e != nil {
		workError(w, e)
		return
	}
	respondJSON(w, 200, map[string]any{"data": ts})
}
func (h *WorkHandler) tokenCreate(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Name     string   `json:"name"`
		Projects []string `json:"projects"`
		Role     string   `json:"role"`
	}
	if e := workBody(w, r, &v); e != nil {
		workError(w, e)
		return
	}
	t, secret, e := h.service.CreateToken(r.Context(), actor(r), v.Name, v.Projects, v.Role)
	if e != nil {
		workError(w, e)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, 201, map[string]any{"token": t, "secret": secret})
}
func (h *WorkHandler) tokenRevoke(w http.ResponseWriter, r *http.Request) {
	if e := h.service.RevokeToken(r.Context(), actor(r), chi.URLParam(r, "id")); e != nil {
		workError(w, e)
		return
	}
	w.WriteHeader(204)
}
func workRoutes(r chi.Router, h *WorkHandler) {
	r.Get("/work-projects", h.projects)
	r.Get("/work-projects/{id}", h.context)
	r.Get("/work-tasks", h.list)
	r.Post("/work-tasks", h.create)
	r.Get("/work-tasks/{id}", h.get)
	r.Post("/work-tasks/{id}/{op}", h.command)
	r.Get("/agent-runs", h.runs)
	r.Post("/agent-runs/{run_id}/verification-reports", h.verification)
	r.Get("/task-results", h.results)
	r.Get("/agent-tokens", h.tokens)
	r.Post("/agent-tokens", h.tokenCreate)
	r.Delete("/agent-tokens/{id}", h.tokenRevoke)
}

// VerificationCreate godoc
// @Summary Append independent verification of an exact open submission (reviewer token)
// @Tags Work
// @Accept json
// @Produce json
// @Param run_id path string true "Attempt ID"
// @Param body body service.VerificationInput true "Digest, checks and stable idempotency key"
// @Success 201 {object} domain.WorkTask
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Router /api/v1/agent-runs/{run_id}/verification-reports [post]
func (h *WorkHandler) verification(w http.ResponseWriter, r *http.Request) {
	var in service.VerificationInput
	if e := workBody(w, r, &in); e != nil {
		workError(w, e)
		return
	}
	task, e := h.service.AddVerification(r.Context(), actor(r), chi.URLParam(r, "run_id"), in)
	if e != nil {
		workError(w, e)
		return
	}
	respondJSON(w, 201, task)
}
