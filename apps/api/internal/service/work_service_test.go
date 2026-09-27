package service_test

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/OstKost/avari-p3-express/apps/api/internal/database"
	"github.com/OstKost/avari-p3-express/apps/api/internal/domain"
	"github.com/OstKost/avari-p3-express/apps/api/internal/repository/sqlite"
	"github.com/OstKost/avari-p3-express/apps/api/internal/service"
)

var manager = domain.WorkActor{ID: "manager", Manager: true}

func setupWork(t *testing.T) (*service.WorkService, *service.P3Service, string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "work.db")
	db, e := database.NewConnection(path)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	if e = database.Migrate(db); e != nil {
		t.Fatal(e)
	}
	p3 := service.NewP3Service(sqlite.NewP3Repository(db))
	p, e := p3.CreateProject(context.Background(), map[string]any{"name": "Work project"})
	if e != nil {
		t.Fatal(e)
	}
	return service.NewWorkService(sqlite.NewWorkRepository(db)), p3, p.ID, path
}
func createTask(t *testing.T, s *service.WorkService, p string) *domain.WorkTask {
	t.Helper()
	v, e := s.Create(context.Background(), manager, p, domain.WorkSpec{Goal: "Create code", ExpectedResult: "Reviewed code", Criteria: []string{"Works"}})
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func cmd(t *testing.T, s *service.WorkService, a domain.WorkActor, v *domain.WorkTask, op string, c service.WorkCommand) *domain.WorkTask {
	t.Helper()
	c.Version = v.Version
	out, e := s.Command(context.Background(), a, v.ID, op, c)
	if e != nil {
		t.Fatalf("%s: %v", op, e)
	}
	return out
}
func TestWorkLifecycleAndPersistence(t *testing.T) {
	s, p3, p, path := setupWork(t)
	ctx := context.Background()
	proj, _ := p3.GetProject(ctx, p)
	v := createTask(t, s, p)
	spec := v.WorkSpec
	spec.StepIDs = []string{proj.Phases[1].Steps[0].ID}
	spec.StageID = proj.SDLCStages[0].ID
	v = cmd(t, s, manager, v, "edit", service.WorkCommand{Spec: &spec})
	v = cmd(t, s, manager, v, "ready", service.WorkCommand{})
	token, secret, e := s.CreateToken(ctx, manager, "agent", []string{p})
	if e != nil {
		t.Fatal(e)
	}
	agent, e := s.Authenticate(ctx, secret)
	if e != nil || agent.ID != token.ID {
		t.Fatal(e)
	}
	v = cmd(t, s, agent, v, "start", service.WorkCommand{Key: "start"})
	run := v.Runs[0]
	snapshot := run.Snapshot
	result := service.WorkCommand{RunID: run.ID, Key: "result", Result: &domain.TaskResult{Description: "Implemented", Kind: "code", URLs: []string{"https://example.com/commit"}, ArtifactVersion: "abc"}}
	v = cmd(t, s, agent, v, "result", result)
	n := len(v.Events)
	again := cmd(t, s, agent, v, "result", result)
	if len(again.Events) != n || len(again.Runs[0].Results) != 1 {
		t.Fatal("duplicate result/event")
	}
	changed := result
	copyResult := *result.Result
	copyResult.Description = "Changed"
	changed.Result = &copyResult
	changed.Version = v.Version
	if _, e = s.Command(ctx, agent, v.ID, "result", changed); !errors.Is(e, domain.ErrConflict) {
		t.Fatalf("key changed payload: %v", e)
	}
	submission := service.WorkCommand{RunID: run.ID, Key: "submit", Text: "Ready with limitations", Checks: []domain.CriterionCheck{{Criterion: 0, Outcome: "pass", Method: "test", Source: "agent", Evidence: "test log"}}}
	v = cmd(t, s, agent, v, "submit", submission)
	n = len(v.Events)
	v = cmd(t, s, agent, v, "submit", submission)
	if len(v.Events) != n {
		t.Fatal("duplicate submission")
	}
	if _, e = s.Command(ctx, agent, v.ID, "review", service.WorkCommand{Version: v.Version, RunID: run.ID, Decision: "accept"}); !errors.Is(e, domain.ErrForbidden) {
		t.Fatalf("agent review: %v", e)
	}
	if _, e = s.Command(ctx, manager, v.ID, "review", service.WorkCommand{Version: v.Version, RunID: run.ID, Decision: "return"}); !errors.Is(e, domain.ErrWorkInput) {
		t.Fatal("empty return accepted")
	}
	v = cmd(t, s, manager, v, "review", service.WorkCommand{RunID: run.ID, Decision: "return", Text: "Add evidence"})
	spec = v.WorkSpec
	spec.Goal = "Revised goal"
	v = cmd(t, s, manager, v, "edit", service.WorkCommand{Spec: &spec})
	if v.Runs[0].Snapshot.Goal != snapshot.Goal {
		t.Fatal("snapshot changed")
	}
	v = cmd(t, s, agent, v, "start", service.WorkCommand{Key: "second"})
	second := v.Runs[1].ID
	result.RunID = second
	result.Key = "result2"
	v = cmd(t, s, agent, v, "result", result)
	submission.RunID = second
	submission.Key = "submit2"
	v = cmd(t, s, agent, v, "submit", submission)
	v = cmd(t, s, manager, v, "review", service.WorkCommand{RunID: second, Decision: "accept"})
	if v.Status != "done" || len(v.Runs) != 2 {
		t.Fatal(v)
	}
	if _, e = p3.CreateCycle(ctx, p, "B"); e != nil {
		t.Fatal(e)
	}
	db2, e := database.NewConnection(path)
	if e != nil {
		t.Fatal(e)
	}
	defer db2.Close()
	if e = database.Migrate(db2); e != nil {
		t.Fatal(e)
	}
	s2 := service.NewWorkService(sqlite.NewWorkRepository(db2))
	got, e := s2.Get(ctx, manager, v.ID)
	if e != nil || got.Status != "done" || got.StepIDs[0] != snapshot.StepIDs[0] || got.Runs[0].Review.Comment != "Add evidence" {
		t.Fatalf("persistence: %+v %v", got, e)
	}
	fresh, _ := p3.GetProject(ctx, p)
	if fresh.Progress != proj.Progress || fresh.SDLCStages[0].Status != proj.SDLCStages[0].Status {
		t.Fatal("task changed management progress")
	}
	if e = s.RevokeToken(ctx, manager, token.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Authenticate(ctx, secret); !errors.Is(e, domain.ErrForbidden) {
		t.Fatal("revoked token works")
	}
}
func TestWorkConcurrencyStopAndDependencies(t *testing.T) {
	s, p3, p, _ := setupWork(t)
	ctx := context.Background()
	v := createTask(t, s, p)
	v = cmd(t, s, manager, v, "ready", service.WorkCommand{})
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, id := range []string{"one", "two"} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			_, e := s.Command(ctx, domain.WorkActor{ID: id, Projects: []string{p}}, v.ID, "start", service.WorkCommand{Version: v.Version, Key: id})
			errs <- e
		}(id)
	}
	wg.Wait()
	close(errs)
	wins := 0
	for e := range errs {
		if e == nil {
			wins++
		} else if !errors.Is(e, domain.ErrConflict) {
			t.Fatal(e)
		}
	}
	if wins != 1 {
		t.Fatal("double start")
	}
	v, _ = s.Get(ctx, manager, v.ID)
	run := v.Runs[0].ID
	if _, e := s.Command(ctx, manager, v.ID, "edit", service.WorkCommand{Version: v.Version, Spec: &v.WorkSpec}); !errors.Is(e, domain.ErrConflict) {
		t.Fatal("edited active task")
	}
	v = cmd(t, s, manager, v, "finish", service.WorkCommand{RunID: run, Text: "stale agent"})
	if _, e := s.Command(ctx, manager, v.ID, "progress", service.WorkCommand{Version: v.Version, RunID: run, Text: "late"}); !errors.Is(e, domain.ErrConflict) {
		t.Fatal("late message")
	}
	dep := createTask(t, s, p)
	spec := dep.WorkSpec
	spec.Dependencies = []string{v.ID}
	dep = cmd(t, s, manager, dep, "edit", service.WorkCommand{Spec: &spec})
	dep = cmd(t, s, manager, dep, "ready", service.WorkCommand{})
	if _, e := s.Command(ctx, manager, dep.ID, "start", service.WorkCommand{Version: dep.Version, Key: "blocked"}); !errors.Is(e, domain.ErrConflict) {
		t.Fatal("unfinished dep")
	}
	spec = v.WorkSpec
	spec.Dependencies = []string{dep.ID}
	if _, e := s.Command(ctx, manager, v.ID, "edit", service.WorkCommand{Version: v.Version, Spec: &spec}); !errors.Is(e, domain.ErrWorkInput) {
		t.Fatal("cycle")
	}
	other, e := p3.CreateProject(ctx, map[string]any{"name": "Other"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Get(ctx, domain.WorkActor{ID: "outside", Projects: []string{other.ID}}, v.ID); !errors.Is(e, domain.ErrForbidden) {
		t.Fatal("cross scope")
	}
	spec = v.WorkSpec
	spec.StepIDs = []string{other.Phases[0].Steps[0].ID}
	if _, e = s.Command(ctx, manager, v.ID, "edit", service.WorkCommand{Version: v.Version, Spec: &spec}); !errors.Is(e, domain.ErrWorkInput) {
		t.Fatal("cross project link")
	}
}

func TestWorkDraftValidationAndCancellation(t *testing.T) {
	s, _, p, _ := setupWork(t)
	ctx := context.Background()
	draft, e := s.Create(ctx, manager, p, domain.WorkSpec{})
	if e != nil {
		t.Fatal(e)
	}
	if draft.Criteria == nil || draft.StepIDs == nil || draft.Dependencies == nil {
		t.Fatal("null collection contract")
	}
	if _, e = s.Command(ctx, manager, draft.ID, "ready", service.WorkCommand{Version: draft.Version}); !errors.Is(e, domain.ErrWorkInput) {
		t.Fatal("empty task became ready")
	}
	task := createTask(t, s, p)
	task = cmd(t, s, manager, task, "ready", service.WorkCommand{})
	task = cmd(t, s, manager, task, "start", service.WorkCommand{Key: "start"})
	runID := task.Runs[0].ID
	task = cmd(t, s, manager, task, "cancel", service.WorkCommand{Text: "No longer needed"})
	if task.Status != "cancelled" || task.Runs[0].State != "cancelled" || task.Runs[0].EndedAt == "" {
		t.Fatal("cancel did not stop attempt")
	}
	if _, e = s.Command(ctx, manager, task.ID, "progress", service.WorkCommand{Version: task.Version, RunID: runID, Text: "late"}); !errors.Is(e, domain.ErrConflict) {
		t.Fatal("cancelled attempt updated")
	}
	if _, e = s.Command(ctx, manager, task.ID, "start", service.WorkCommand{Version: task.Version, Key: "new"}); !errors.Is(e, domain.ErrConflict) {
		t.Fatal("cancelled task restarted")
	}
}
