package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/OstKost/avari-p3-express/apps/api/internal/domain"
	"github.com/google/uuid"
)

type WorkService struct{ repo domain.WorkRepository }

func NewWorkService(r domain.WorkRepository) *WorkService { return &WorkService{r} }
func workNow() string                                     { return time.Now().UTC().Format(time.RFC3339Nano) }
func workInput(s string) error                            { return fmt.Errorf("%w: %s", domain.ErrWorkInput, s) }
func has(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func taskIn(d *domain.WorkData, id string) *domain.WorkTask {
	for i := range d.Tasks {
		if d.Tasks[i].ID == id {
			return &d.Tasks[i]
		}
	}
	return nil
}
func event(t *domain.WorkTask, a domain.WorkActor, kind, text, run, result string) {
	t.Version++
	t.Events = append(t.Events, domain.WorkEvent{ID: uuid.NewString(), TaskID: t.ID, Actor: a.ID, ActorName: a.Name, Kind: kind, Text: text, RunID: run, ResultID: result, CreatedAt: workNow()})
}
func normalizeSpec(spec *domain.WorkSpec) {
	if spec.Criteria == nil {
		spec.Criteria = []string{}
	}
	if spec.StepIDs == nil {
		spec.StepIDs = []string{}
	}
	if spec.Dependencies == nil {
		spec.Dependencies = []string{}
	}
}
func validateSpec(d *domain.WorkData, t *domain.WorkTask) error {
	if !has(d.Projects, t.ProjectID) {
		return domain.ErrNotFound
	}
	if t.Priority != "low" && t.Priority != "medium" && t.Priority != "high" {
		return workInput("priority must be low, medium or high")
	}
	if t.DueDate != "" {
		if _, e := time.Parse("2006-01-02", t.DueDate); e != nil {
			return workInput("due_date must be YYYY-MM-DD")
		}
	}
	for _, s := range t.StepIDs {
		if d.Steps[s] != t.ProjectID {
			return workInput("step belongs to another project or does not exist")
		}
	}
	if t.StageID != "" && d.Stages[t.StageID] != t.ProjectID {
		return workInput("stage belongs to another project or does not exist")
	}
	seen := map[string]bool{}
	for _, dep := range t.Dependencies {
		other := taskIn(d, dep)
		if dep == t.ID || seen[dep] || other == nil || other.ProjectID != t.ProjectID {
			return workInput("invalid dependency")
		}
		seen[dep] = true
	}
	var visit func(string, map[string]bool) bool
	visit = func(id string, path map[string]bool) bool {
		if id == t.ID {
			return true
		}
		if path[id] {
			return false
		}
		path[id] = true
		other := taskIn(d, id)
		if other != nil {
			for _, dep := range other.Dependencies {
				if visit(dep, path) {
					return true
				}
			}
		}
		return false
	}
	for _, id := range t.Dependencies {
		if visit(id, map[string]bool{}) {
			return workInput("dependency cycle")
		}
	}
	return nil
}
func ready(t *domain.WorkTask) error {
	if strings.TrimSpace(t.Goal) == "" || strings.TrimSpace(t.ExpectedResult) == "" || len(t.Criteria) == 0 {
		return workInput("goal, expected_result and criteria required")
	}
	for _, c := range t.Criteria {
		if strings.TrimSpace(c) == "" {
			return workInput("empty criterion")
		}
	}
	return nil
}
func (s *WorkService) Create(ctx context.Context, a domain.WorkActor, p string, spec domain.WorkSpec) (*domain.WorkTask, error) {
	if !a.Manager {
		return nil, domain.ErrForbidden
	}
	normalizeSpec(&spec)
	if spec.Priority == "" {
		spec.Priority = "medium"
	}
	t := domain.WorkTask{ID: uuid.NewString(), ProjectID: p, WorkSpec: spec, Status: "draft", Runs: []domain.AgentRun{}, Events: []domain.WorkEvent{}}
	err := s.repo.Transact(ctx, func(d *domain.WorkData) error {
		if e := validateSpec(d, &t); e != nil {
			return e
		}
		event(&t, a, "created", "", "", "")
		d.Tasks = append(d.Tasks, t)
		return nil
	})
	return &t, err
}
func (s *WorkService) List(ctx context.Context, a domain.WorkActor) ([]domain.WorkTask, error) {
	out := []domain.WorkTask{}
	e := s.repo.Transact(ctx, func(d *domain.WorkData) error {
		for _, t := range d.Tasks {
			if a.Allows(t.ProjectID) {
				out = append(out, t)
			}
		}
		return nil
	})
	return out, e
}
func (s *WorkService) Get(ctx context.Context, a domain.WorkActor, id string) (*domain.WorkTask, error) {
	var out *domain.WorkTask
	e := s.repo.Transact(ctx, func(d *domain.WorkData) error {
		t := taskIn(d, id)
		if t == nil {
			return domain.ErrNotFound
		}
		if !a.Allows(t.ProjectID) {
			return domain.ErrForbidden
		}
		out = t
		return nil
	})
	return out, e
}

type WorkCommand struct {
	Version  int                     `json:"version"`
	Spec     *domain.WorkSpec        `json:"spec,omitempty"`
	RunID    string                  `json:"run_id"`
	Key      string                  `json:"idempotency_key"`
	Text     string                  `json:"text"`
	Result   *domain.TaskResult      `json:"result,omitempty"`
	Checks   []domain.CriterionCheck `json:"checks,omitempty"`
	Decision string                  `json:"decision"`
}

func fingerprint(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func (s *WorkService) Command(ctx context.Context, a domain.WorkActor, id, op string, c WorkCommand) (*domain.WorkTask, error) {
	var out *domain.WorkTask
	e := s.repo.Transact(ctx, func(d *domain.WorkData) error {
		t := taskIn(d, id)
		if t == nil {
			return domain.ErrNotFound
		}
		if !a.Allows(t.ProjectID) {
			return domain.ErrForbidden
		}
		managerOp := op == "edit" || op == "ready" || op == "review" || op == "cancel"
		if !a.Manager && a.Role != "" && a.Role != "executor" {
			return domain.ErrForbidden
		}
		if managerOp && !a.Manager {
			return domain.ErrForbidden
		}
		var run *domain.AgentRun
		for i := range t.Runs {
			if t.Runs[i].ID == c.RunID {
				run = &t.Runs[i]
			}
		}
		if run != nil && !a.Manager && run.Actor != a.ID {
			return domain.ErrForbidden
		}
		// Successful retries do not mutate events or overwrite a newer attempt.
		if op == "start" {
			if c.Key == "" {
				return workInput("idempotency_key required")
			}
			for _, r := range t.Runs {
				if r.StartKey == c.Key {
					if r.Actor != a.ID {
						return domain.ErrConflict
					}
					out = t
					return nil
				}
			}
		}
		if op == "result" || op == "submit" {
			if c.Key == "" {
				return workInput("idempotency_key required")
			}
			if run == nil {
				return domain.ErrNotFound
			}
			if op == "result" {
				if c.Result == nil {
					return workInput("result required")
				}
				for _, res := range run.Results {
					if res.Key == c.Key {
						v := *c.Result
						v.ID = res.ID
						v.TaskID = res.TaskID
						v.RunID = res.RunID
						v.Key = res.Key
						v.CreatedAt = res.CreatedAt
						if fingerprint(v) != fingerprint(res) {
							return domain.ErrConflict
						}
						out = t
						return nil
					}
				}
			}
			if op == "submit" && run.SubmitKey == c.Key {
				if run.SubmitFingerprint != fingerprint(struct {
					Text   string
					Checks []domain.CriterionCheck
				}{c.Text, c.Checks}) {
					return domain.ErrConflict
				}
				out = t
				return nil
			}
		}
		if c.Version != t.Version {
			return domain.ErrConflict
		}
		switch op {
		case "edit":
			if t.Status != "draft" && t.Status != "ready" {
				return domain.ErrConflict
			}
			if c.Spec == nil {
				return workInput("spec required")
			}
			t.WorkSpec = *c.Spec
			normalizeSpec(&t.WorkSpec)
			if e := validateSpec(d, t); e != nil {
				return e
			}
			if t.Status == "ready" {
				if e := ready(t); e != nil {
					return e
				}
			}
		case "ready":
			if t.Status != "draft" {
				return domain.ErrConflict
			}
			if e := ready(t); e != nil {
				return e
			}
			t.Status = "ready"
		case "start":
			if t.Status != "ready" || t.Blocker != "" {
				return domain.ErrConflict
			}
			if e := ready(t); e != nil {
				return e
			}
			for _, dep := range t.Dependencies {
				other := taskIn(d, dep)
				if other == nil || other.Status != "done" {
					return domain.ErrConflict
				}
			}
			snapshot := t.WorkSpec
			b, _ := json.Marshal(snapshot)
			_ = json.Unmarshal(b, &snapshot)
			t.Runs = append(t.Runs, domain.AgentRun{ID: uuid.NewString(), TaskID: t.ID, Actor: a.ID, ActorName: a.Name, State: "active", StartedAt: workNow(), LastMessageAt: workNow(), Snapshot: snapshot, Results: []domain.TaskResult{}, Checks: []domain.CriterionCheck{}, VerificationReports: []domain.VerificationReport{}, StartKey: c.Key})
			run = &t.Runs[len(t.Runs)-1]
			t.Status = "in_progress"
		case "cancel":
			if t.Status == "done" || t.Status == "cancelled" {
				return domain.ErrConflict
			}
			for i := range t.Runs {
				if t.Runs[i].State == "active" {
					t.Runs[i].State = "cancelled"
					t.Runs[i].EndedAt = workNow()
				}
			}
			t.Status = "cancelled"
		case "review":
			if t.Status != "in_review" || run == nil || run.State != "submitted" || run.Review != nil {
				return domain.ErrConflict
			}
			if c.Decision != "accept" && c.Decision != "return" {
				return workInput("decision must be accept or return")
			}
			if c.Decision == "return" && strings.TrimSpace(c.Text) == "" {
				return workInput("return requires comment")
			}
			if c.Decision == "accept" {
				if e := acceptanceGate(*run, c.Text); e != nil {
					return e
				}
			}
			run.Review = &domain.ReviewDecision{Decision: c.Decision, Comment: c.Text, Actor: a.ID, CreatedAt: workNow()}
			if c.Decision == "accept" {
				t.Status = "done"
			} else {
				t.Status = "ready"
			}
			t.Blocker = ""
		default:
			if run == nil {
				return domain.ErrNotFound
			}
			if run.State != "active" || t.Status != "in_progress" {
				return domain.ErrConflict
			}
			switch op {
			case "progress", "comment":
				if strings.TrimSpace(c.Text) == "" {
					return workInput("text required")
				}
				run.Report = c.Text
			case "blocker":
				t.Blocker = c.Text
			case "finish":
				if strings.TrimSpace(c.Text) == "" {
					return workInput("finish requires reason")
				}
				run.State = "stopped"
				run.EndedAt = workNow()
				run.Report = c.Text
				t.Status = "ready"
				t.Blocker = ""
			case "result":
				if c.Result == nil {
					return workInput("result required")
				}
				res := *c.Result
				if strings.TrimSpace(res.Description) == "" || strings.TrimSpace(res.Kind) == "" {
					return workInput("result description and kind required")
				}
				for _, raw := range res.URLs {
					u, e := url.Parse(raw)
					if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
						return workInput("artifact URL must be HTTP(S)")
					}
				}
				res.ID = uuid.NewString()
				res.TaskID = t.ID
				res.RunID = run.ID
				res.Key = c.Key
				res.CreatedAt = workNow()
				run.Results = append(run.Results, res)
			case "submit":
				if len(run.Results) == 0 || strings.TrimSpace(c.Text) == "" || len(c.Checks) != len(run.Snapshot.Criteria) {
					return workInput("report, results and check for every criterion required")
				}
				seen := map[int]bool{}
				for _, v := range c.Checks {
					if v.Criterion < 0 || v.Criterion >= len(run.Snapshot.Criteria) || seen[v.Criterion] || strings.TrimSpace(v.Method) == "" || strings.TrimSpace(v.Source) == "" || strings.TrimSpace(v.Evidence) == "" || (v.Outcome != "pass" && v.Outcome != "fail" && v.Outcome != "unknown") {
						return workInput("invalid criterion check")
					}
					seen[v.Criterion] = true
				}
				run.Checks = c.Checks
				run.Report = c.Text
				run.State = "submitted"
				run.EndedAt = workNow()
				run.SubmitKey = c.Key
				run.SubmitFingerprint = fingerprint(struct {
					Text   string
					Checks []domain.CriterionCheck
				}{c.Text, c.Checks})
				run.SubmissionDigest = domain.SubmissionDigest(*run)
				t.Status = "in_review"
			default:
				return workInput("unknown operation")
			}
			run.LastMessageAt = workNow()
		}
		rid, resid := "", ""
		if run != nil {
			rid = run.ID
			if op == "result" {
				resid = run.Results[len(run.Results)-1].ID
			}
		}
		event(t, a, op, c.Text, rid, resid)
		out = t
		return nil
	})
	return out, e
}
func (s *WorkService) Tokens(ctx context.Context, a domain.WorkActor) ([]domain.AgentToken, error) {
	if !a.Manager {
		return nil, domain.ErrForbidden
	}
	return s.repo.Tokens(ctx)
}
func (s *WorkService) CreateToken(ctx context.Context, a domain.WorkActor, name string, projects []string, roles ...string) (domain.AgentToken, string, error) {
	if !a.Manager {
		return domain.AgentToken{}, "", domain.ErrForbidden
	}
	role := "executor"
	if len(roles) > 0 && roles[0] != "" {
		role = roles[0]
	}
	if role != "executor" && role != "reviewer" {
		return domain.AgentToken{}, "", workInput("role must be executor or reviewer")
	}
	if strings.TrimSpace(name) == "" || len(projects) == 0 {
		return domain.AgentToken{}, "", workInput("name and projects required")
	}
	e := s.repo.Transact(ctx, func(d *domain.WorkData) error {
		for _, p := range projects {
			if !has(d.Projects, p) {
				return domain.ErrNotFound
			}
		}
		return nil
	})
	if e != nil {
		return domain.AgentToken{}, "", e
	}
	secret := uuid.NewString() + uuid.NewString()
	t := domain.AgentToken{ID: uuid.NewString(), Name: name, Role: role, Projects: projects, Hash: fingerprint(secret)}
	return t, secret, s.repo.SaveToken(ctx, t)
}
func (s *WorkService) RevokeToken(ctx context.Context, a domain.WorkActor, id string) error {
	ts, e := s.Tokens(ctx, a)
	if e != nil {
		return e
	}
	for _, t := range ts {
		if t.ID == id {
			t.Revoked = true
			return s.repo.SaveToken(ctx, t)
		}
	}
	return domain.ErrNotFound
}
func (s *WorkService) Authenticate(ctx context.Context, secret string) (domain.WorkActor, error) {
	ts, e := s.repo.Tokens(ctx)
	if e != nil {
		return domain.WorkActor{}, e
	}
	for _, t := range ts {
		if !t.Revoked && t.Hash == fingerprint(secret) {
			return domain.WorkActor{ID: t.ID, Name: t.Name, Role: t.Role, Projects: t.Projects}, nil
		}
	}
	return domain.WorkActor{}, domain.ErrForbidden
}

func (s *WorkService) Context(ctx context.Context, a domain.WorkActor, id string) ([]domain.WorkTask, []domain.WorkStepContext, error) {
	deps := []domain.WorkTask{}
	steps := []domain.WorkStepContext{}
	e := s.repo.Transact(ctx, func(d *domain.WorkData) error {
		t := taskIn(d, id)
		if t == nil {
			return domain.ErrNotFound
		}
		if !a.Allows(t.ProjectID) {
			return domain.ErrForbidden
		}
		for _, id := range t.Dependencies {
			if dep := taskIn(d, id); dep != nil {
				deps = append(deps, *dep)
			}
		}
		ids := append([]string{}, t.StepIDs...)
		for _, r := range t.Runs {
			for _, id := range r.Snapshot.StepIDs {
				if !has(ids, id) {
					ids = append(ids, id)
				}
			}
		}
		for _, id := range ids {
			if ref, ok := d.StepContexts[id]; ok {
				steps = append(steps, ref)
			}
		}
		return nil
	})
	return deps, steps, e
}
