package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/OstKost/avari-p3-express/apps/api/internal/domain"
	"github.com/google/uuid"
)

var ErrP3Input = errors.New("invalid input")

type P3Service struct{ repo domain.P3Repository }

func NewP3Service(repo domain.P3Repository) *P3Service { return &P3Service{repo: repo} }
func input(msg string) error                           { return fmt.Errorf("%w: %s", ErrP3Input, msg) }
func str(m map[string]any, key string, required bool) (string, error) {
	v, ok := m[key]
	if !ok {
		if required {
			return "", input(key + " is required")
		}
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", input(key + " must be a string")
	}
	s = strings.TrimSpace(s)
	if required && s == "" {
		return "", input(key + " is required")
	}
	return s, nil
}
func optionalString(m map[string]any, key string) (*string, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return nil, nil
	}
	s, ok := v.(string)
	if !ok {
		return nil, input(key + " must be a string or null")
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	return &s, nil
}
func date(m map[string]any, key string) (*string, error) {
	s, err := optionalString(m, key)
	if err != nil || s == nil {
		return s, err
	}
	d, e := time.Parse("2006-01-02", *s)
	if e != nil || d.Format("2006-01-02") != *s {
		return nil, input(key + " must be YYYY-MM-DD")
	}
	return s, nil
}
func validURL(s string) bool {
	u, e := url.ParseRequestURI(s)
	if e != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || strings.HasSuffix(u.Host, ":") {
		return false
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return false
		}
	}
	return true
}
func allowed(m map[string]any, keys ...string) error {
	set := map[string]bool{}
	for _, k := range keys {
		set[k] = true
	}
	for k := range m {
		if !set[k] {
			return input("unknown field " + k)
		}
	}
	return nil
}
func status(s string) bool {
	return s == "todo" || s == "not_started" || s == "in_progress" || s == "blocked" || s == "done"
}
func (s *P3Service) CreateProject(ctx context.Context, m map[string]any) (*domain.Project, error) {
	if err := allowed(m, "name", "description", "due_date"); err != nil {
		return nil, err
	}
	name, err := str(m, "name", true)
	if err != nil {
		return nil, err
	}
	description, err := str(m, "description", false)
	if err != nil {
		return nil, err
	}
	due, err := date(m, "due_date")
	if err != nil {
		return nil, err
	}
	p := domain.Project{ID: uuid.NewString(), Name: name, Description: description, DueDate: due, RAGStatus: "green", CurrentPhase: "A"}
	if err = s.repo.CreateProject(ctx, p); err != nil {
		return nil, err
	}
	return s.repo.GetProject(ctx, p.ID)
}
func (s *P3Service) GetProject(ctx context.Context, id string) (*domain.Project, error) {
	return s.repo.GetProject(ctx, id)
}
func (s *P3Service) ListProjects(ctx context.Context) ([]domain.ProjectSummary, error) {
	return s.repo.ListProjects(ctx)
}
func (s *P3Service) ListCycles(ctx context.Context, id string) ([]domain.Cycle, error) {
	return s.repo.ListCycles(ctx, id)
}
func (s *P3Service) CreateCycle(ctx context.Context, id, code string) (*domain.Cycle, error) {
	if code != "B" && code != "C" && code != "D" && code != "E" {
		return nil, input("only B, C, D, E are repeatable")
	}
	c, err := s.repo.CreateCycle(ctx, id, code)
	if err != nil {
		return nil, err
	}
	return c, nil
}
func (s *P3Service) ListActions(ctx context.Context) ([]domain.Action, error) {
	return s.repo.ListActions(ctx)
}
func (s *P3Service) ListArticles(ctx context.Context) ([]domain.Article, error) {
	return s.repo.ListArticles(ctx)
}
func (s *P3Service) Patch(ctx context.Context, kind, id string, m map[string]any) (any, error) {
	if len(m) == 0 {
		return nil, input("empty patch")
	}
	rules := map[string][]string{"project": {"name", "description", "due_date", "archived", "rag_status", "progress", "current_phase"}, "step": {"status", "due_date"}, "checklist": {"text", "done"}, "blocker": {"resolved"}, "sdlc_stage": {"status", "progress", "start_date", "due_date"}, "action": {"title", "due_date", "priority", "done"}, "article": {"title", "category", "summary", "body"}}
	fields, ok := rules[kind]
	if !ok {
		return nil, input("invalid resource")
	}
	if err := allowed(m, fields...); err != nil {
		return nil, err
	}
	for k, v := range m {
		switch k {
		case "name", "title", "text", "category", "body":
			x, ok := v.(string)
			if !ok || strings.TrimSpace(x) == "" {
				return nil, input(k + " is required")
			}
			m[k] = strings.TrimSpace(x)
		case "description", "summary":
			x, ok := v.(string)
			if !ok {
				return nil, input(k + " must be a string")
			}
			m[k] = strings.TrimSpace(x)
		case "due_date", "start_date":
			d, err := date(m, k)
			if err != nil {
				return nil, err
			}
			m[k] = d
		case "progress":
			x, ok := v.(float64)
			if !ok || x < 0 || x > 100 || x != float64(int(x)) {
				return nil, input("progress must be 0-100")
			}
			m[k] = int(x)
		case "archived", "done", "resolved":
			if _, ok := v.(bool); !ok {
				return nil, input(k + " must be boolean")
			}
		case "rag_status":
			if v != "green" && v != "yellow" && v != "red" {
				return nil, input("invalid rag_status")
			}
		case "current_phase":
			valid := false
			for _, p := range domain.P3Template {
				if v == p.Code {
					valid = true
				}
			}
			if !valid {
				return nil, input("invalid current_phase")
			}
		case "status":
			x, ok := v.(string)
			if !ok || !status(x) {
				return nil, input("invalid status")
			}
		case "priority":
			if v != "low" && v != "medium" && v != "high" {
				return nil, input("invalid priority")
			}
		}
	}
	projectID := ""
	if kind != "article" {
		var err error
		projectID, err = s.repo.FindProjectFor(ctx, kind, id)
		if err != nil {
			return nil, err
		}
	}
	var err error
	switch kind {
	case "project":
		err = s.repo.PatchProject(ctx, id, m)
	case "step":
		err = s.repo.PatchStep(ctx, id, m)
	case "checklist":
		err = s.repo.PatchChecklist(ctx, id, m)
	case "blocker":
		err = s.repo.PatchBlocker(ctx, id, m)
	case "sdlc_stage":
		err = s.repo.PatchSDLCStage(ctx, id, m)
	case "action":
		err = s.repo.PatchAction(ctx, id, m)
	case "article":
		err = s.repo.PatchArticle(ctx, id, m)
	}
	if err != nil {
		return nil, err
	}
	return s.entity(ctx, kind, id, projectID)
}
func (s *P3Service) entity(ctx context.Context, kind, id, projectID string) (any, error) {
	if kind == "article" {
		items, err := s.repo.ListArticles(ctx)
		if err != nil {
			return nil, err
		}
		for _, x := range items {
			if x.ID == id {
				return x, nil
			}
		}
		return nil, domain.ErrNotFound
	}
	p, err := s.repo.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if kind == "project" {
		return p, nil
	}
	for _, ph := range p.Phases {
		for _, st := range ph.Steps {
			if kind == "step" && st.ID == id {
				return st, nil
			}
			if kind == "checklist" {
				for _, x := range st.Checklist {
					if x.ID == id {
						return x, nil
					}
				}
			}
		}
	}
	for _, x := range p.Blockers {
		if kind == "blocker" && x.ID == id {
			return x, nil
		}
	}
	for _, x := range p.SDLCStages {
		if kind == "sdlc_stage" && x.ID == id {
			return x, nil
		}
	}
	for _, x := range p.Actions {
		if kind == "action" && x.ID == id {
			return x, nil
		}
	}
	return nil, domain.ErrNotFound
}
func (s *P3Service) Create(ctx context.Context, kind, parent string, m map[string]any) (any, error) {
	id := uuid.NewString()
	switch kind {
	case "checklist":
		if err := allowed(m, "text"); err != nil {
			return nil, err
		}
		text, err := str(m, "text", true)
		if err != nil {
			return nil, err
		}
		if _, err := s.repo.FindProjectFor(ctx, "step", parent); err != nil {
			return nil, err
		}
		x := domain.ChecklistItem{ID: id, StepID: parent, Text: text}
		if err = s.repo.CreateChecklist(ctx, parent, x); err != nil {
			return nil, err
		}
		return x, nil
	case "link":
		if err := allowed(m, "title", "url", "description"); err != nil {
			return nil, err
		}
		title, err := str(m, "title", true)
		if err != nil {
			return nil, err
		}
		u, err := str(m, "url", true)
		if err != nil {
			return nil, err
		}
		if !validURL(u) {
			return nil, input("url must be http or https")
		}
		desc, err := str(m, "description", false)
		if err != nil {
			return nil, err
		}
		if _, err := s.repo.FindProjectFor(ctx, "step", parent); err != nil {
			return nil, err
		}
		x := domain.ArtifactLink{ID: id, StepID: parent, Title: title, URL: u, Description: desc}
		if err = s.repo.CreateLink(ctx, parent, x); err != nil {
			return nil, err
		}
		return x, nil
	case "comment":
		if err := allowed(m, "text"); err != nil {
			return nil, err
		}
		text, err := str(m, "text", true)
		if err != nil {
			return nil, err
		}
		if _, err := s.repo.FindProjectFor(ctx, "step", parent); err != nil {
			return nil, err
		}
		x := domain.Comment{ID: id, StepID: parent, Text: text, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
		if err = s.repo.CreateComment(ctx, parent, x); err != nil {
			return nil, err
		}
		return x, nil
	case "blocker":
		if err := allowed(m, "title", "description", "step_id", "sdlc_stage_id"); err != nil {
			return nil, err
		}
		title, err := str(m, "title", true)
		if err != nil {
			return nil, err
		}
		desc, err := str(m, "description", false)
		if err != nil {
			return nil, err
		}
		if _, err = s.repo.FindProjectFor(ctx, "project", parent); err != nil {
			return nil, err
		}
		step, err := optionalString(m, "step_id")
		if err != nil {
			return nil, err
		}
		stage, err := optionalString(m, "sdlc_stage_id")
		if err != nil {
			return nil, err
		}
		if step != nil {
			p, e := s.repo.FindProjectFor(ctx, "step", *step)
			if e != nil {
				return nil, e
			}
			if p != parent {
				return nil, input("step belongs to another project")
			}
		}
		if stage != nil {
			p, e := s.repo.FindProjectFor(ctx, "sdlc_stage", *stage)
			if e != nil {
				return nil, e
			}
			if p != parent {
				return nil, input("stage belongs to another project")
			}
		}
		x := domain.Blocker{ID: id, ProjectID: parent, StepID: step, SDLCStageID: stage, Title: title, Description: desc}
		if err = s.repo.CreateBlocker(ctx, x); err != nil {
			return nil, err
		}
		return x, nil
	case "action":
		if err := allowed(m, "project_id", "title", "due_date", "priority", "step_id"); err != nil {
			return nil, err
		}
		project, err := str(m, "project_id", true)
		if err != nil {
			return nil, err
		}
		if _, err = s.repo.FindProjectFor(ctx, "project", project); err != nil {
			return nil, err
		}
		title, err := str(m, "title", true)
		if err != nil {
			return nil, err
		}
		due, err := date(m, "due_date")
		if err != nil {
			return nil, err
		}
		step, err := optionalString(m, "step_id")
		if err != nil {
			return nil, err
		}
		if step != nil {
			p, e := s.repo.FindProjectFor(ctx, "step", *step)
			if e != nil {
				return nil, e
			}
			if p != project {
				return nil, input("step belongs to another project")
			}
		}
		priority, err := str(m, "priority", false)
		if err != nil {
			return nil, err
		}
		if priority == "" {
			priority = "medium"
		}
		if priority != "low" && priority != "medium" && priority != "high" {
			return nil, input("invalid priority")
		}
		x := domain.Action{ID: id, ProjectID: project, StepID: step, Title: title, DueDate: due, Priority: priority}
		if err = s.repo.CreateAction(ctx, x); err != nil {
			return nil, err
		}
		return x, nil
	case "article":
		if err := allowed(m, "title", "category", "summary", "body"); err != nil {
			return nil, err
		}
		title, err := str(m, "title", true)
		if err != nil {
			return nil, err
		}
		category, err := str(m, "category", true)
		if err != nil {
			return nil, err
		}
		summary, err := str(m, "summary", false)
		if err != nil {
			return nil, err
		}
		body, err := str(m, "body", true)
		if err != nil {
			return nil, err
		}
		x := domain.Article{ID: id, Title: title, Category: category, Summary: summary, Body: body}
		return x, s.repo.CreateArticle(ctx, x)
	}
	return nil, input("invalid resource")
}
func (s *P3Service) Delete(ctx context.Context, kind, id string) error {
	if kind != "checklist" && kind != "link" {
		return input("invalid resource")
	}
	if _, err := s.repo.FindProjectFor(ctx, kind, id); err != nil {
		return err
	}
	var err error
	if kind == "checklist" {
		err = s.repo.DeleteChecklist(ctx, id)
	} else {
		err = s.repo.DeleteLink(ctx, id)
	}
	if err != nil {
		return err
	}
	return nil
}
