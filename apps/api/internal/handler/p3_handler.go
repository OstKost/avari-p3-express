package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/OstKost/avari-p3-express/apps/api/internal/domain"
	"github.com/OstKost/avari-p3-express/apps/api/internal/service"
	"github.com/go-chi/chi/v5"
)

type P3Handler struct{ service *service.P3Service }

func NewP3Handler(s *service.P3Service) *P3Handler { return &P3Handler{service: s} }
func p3Body(r *http.Request) (map[string]any, error) {
	defer r.Body.Close()
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	m := map[string]any{}
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, service.ErrP3Input
	}
	return m, nil
}
func p3Error(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		respondError(w, http.StatusNotFound, "Not found")
	case errors.Is(err, service.ErrP3Input):
		respondError(w, http.StatusBadRequest, err.Error())
	default:
		respondError(w, http.StatusInternalServerError, "Internal server error")
	}
}
func (h *P3Handler) bodyMutation(w http.ResponseWriter, r *http.Request, fn func(map[string]any) (any, error), status int) {
	m, err := p3Body(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	v, err := fn(m)
	if err != nil {
		p3Error(w, err)
		return
	}
	respondJSON(w, status, v)
}

// ProjectList godoc
// @Summary List P3 projects
// @Tags P3
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/p3/projects [get]
func (h *P3Handler) ProjectList(w http.ResponseWriter, r *http.Request) {
	x, e := h.service.ListProjects(r.Context())
	if e != nil {
		p3Error(w, e)
		return
	}
	respondJSON(w, 200, map[string]any{"data": x})
}

// ProjectCreate godoc
// @Summary Create a P3 project with seven activity groups
// @Tags P3
// @Accept json
// @Produce json
// @Success 201 {object} domain.Project
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/p3/projects [post]
func (h *P3Handler) ProjectCreate(w http.ResponseWriter, r *http.Request) {
	h.bodyMutation(w, r, func(m map[string]any) (any, error) { return h.service.CreateProject(r.Context(), m) }, 201)
}

// ProjectGet godoc
// @Summary Get a P3 project with active cycles and activity
// @Tags P3
// @Produce json
// @Success 200 {object} domain.Project
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/p3/projects/{id} [get]
func (h *P3Handler) ProjectGet(w http.ResponseWriter, r *http.Request) {
	x, e := h.service.GetProject(r.Context(), chi.URLParam(r, "id"))
	if e != nil {
		p3Error(w, e)
		return
	}
	respondJSON(w, 200, x)
}

// ProjectPatch godoc
// @Summary Update P3 project fields
// @Tags P3
// @Accept json
// @Produce json
// @Success 200 {object} domain.Project
// @Router /api/v1/p3/projects/{id} [patch]
func (h *P3Handler) ProjectPatch(w http.ResponseWriter, r *http.Request) { h.patch(w, r, "project") }
func (h *P3Handler) StepPatch(w http.ResponseWriter, r *http.Request)    { h.patch(w, r, "step") }
func (h *P3Handler) ChecklistPatch(w http.ResponseWriter, r *http.Request) {
	h.patch(w, r, "checklist")
}
func (h *P3Handler) BlockerPatch(w http.ResponseWriter, r *http.Request) { h.patch(w, r, "blocker") }
func (h *P3Handler) SDLCPatch(w http.ResponseWriter, r *http.Request)    { h.patch(w, r, "sdlc_stage") }
func (h *P3Handler) ActionPatch(w http.ResponseWriter, r *http.Request)  { h.patch(w, r, "action") }
func (h *P3Handler) ArticlePatch(w http.ResponseWriter, r *http.Request) { h.patch(w, r, "article") }
func (h *P3Handler) patch(w http.ResponseWriter, r *http.Request, kind string) {
	id := chi.URLParam(r, "id")
	h.bodyMutation(w, r, func(m map[string]any) (any, error) { return h.service.Patch(r.Context(), kind, id, m) }, 200)
}
func (h *P3Handler) ChecklistCreate(w http.ResponseWriter, r *http.Request) {
	h.create(w, r, "checklist", chi.URLParam(r, "id"))
}
func (h *P3Handler) LinkCreate(w http.ResponseWriter, r *http.Request) {
	h.create(w, r, "link", chi.URLParam(r, "id"))
}
func (h *P3Handler) CommentCreate(w http.ResponseWriter, r *http.Request) {
	h.create(w, r, "comment", chi.URLParam(r, "id"))
}
func (h *P3Handler) BlockerCreate(w http.ResponseWriter, r *http.Request) {
	h.create(w, r, "blocker", chi.URLParam(r, "id"))
}
func (h *P3Handler) ActionCreate(w http.ResponseWriter, r *http.Request) {
	h.create(w, r, "action", "")
}
func (h *P3Handler) ArticleCreate(w http.ResponseWriter, r *http.Request) {
	h.create(w, r, "article", "")
}
func (h *P3Handler) create(w http.ResponseWriter, r *http.Request, kind, parent string) {
	h.bodyMutation(w, r, func(m map[string]any) (any, error) { return h.service.Create(r.Context(), kind, parent, m) }, 201)
}
func (h *P3Handler) ChecklistDelete(w http.ResponseWriter, r *http.Request) {
	h.delete(w, r, "checklist")
}
func (h *P3Handler) LinkDelete(w http.ResponseWriter, r *http.Request) { h.delete(w, r, "link") }
func (h *P3Handler) delete(w http.ResponseWriter, r *http.Request, kind string) {
	if e := h.service.Delete(r.Context(), kind, chi.URLParam(r, "id")); e != nil {
		p3Error(w, e)
		return
	}
	w.WriteHeader(204)
}
func (h *P3Handler) ActionList(w http.ResponseWriter, r *http.Request) {
	x, e := h.service.ListActions(r.Context())
	if e != nil {
		p3Error(w, e)
		return
	}
	respondJSON(w, 200, map[string]any{"data": x})
}
func (h *P3Handler) ArticleList(w http.ResponseWriter, r *http.Request) {
	x, e := h.service.ListArticles(r.Context())
	if e != nil {
		p3Error(w, e)
		return
	}
	respondJSON(w, 200, map[string]any{"data": x})
}
func (h *P3Handler) CycleList(w http.ResponseWriter, r *http.Request) {
	x, e := h.service.ListCycles(r.Context(), chi.URLParam(r, "id"))
	if e != nil {
		p3Error(w, e)
		return
	}
	respondJSON(w, 200, map[string]any{"data": x})
}
func (h *P3Handler) CycleCreate(w http.ResponseWriter, r *http.Request) {
	x, e := h.service.CreateCycle(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "phase_code"))
	if e != nil {
		p3Error(w, e)
		return
	}
	respondJSON(w, 201, x)
}
func p3Routes(r chi.Router, h *P3Handler) {
	r.Route("/p3", func(r chi.Router) {
		r.Get("/projects", h.ProjectList)
		r.Post("/projects", h.ProjectCreate)
		r.Get("/projects/{id}", h.ProjectGet)
		r.Patch("/projects/{id}", h.ProjectPatch)
		r.Get("/projects/{id}/cycles", h.CycleList)
		r.Post("/projects/{id}/cycles/{phase_code}", h.CycleCreate)
		r.Patch("/steps/{id}", h.StepPatch)
		r.Post("/steps/{id}/checklist", h.ChecklistCreate)
		r.Post("/steps/{id}/links", h.LinkCreate)
		r.Post("/steps/{id}/comments", h.CommentCreate)
		r.Patch("/checklist/{id}", h.ChecklistPatch)
		r.Delete("/checklist/{id}", h.ChecklistDelete)
		r.Delete("/links/{id}", h.LinkDelete)
		r.Post("/projects/{id}/blockers", h.BlockerCreate)
		r.Patch("/blockers/{id}", h.BlockerPatch)
		r.Patch("/sdlc-stages/{id}", h.SDLCPatch)
		r.Get("/actions", h.ActionList)
		r.Post("/actions", h.ActionCreate)
		r.Patch("/actions/{id}", h.ActionPatch)
		r.Get("/articles", h.ArticleList)
		r.Post("/articles", h.ArticleCreate)
		r.Patch("/articles/{id}", h.ArticlePatch)
	})
}
