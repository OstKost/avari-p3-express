package service_test

import (
	"context"
	"errors"
	"github.com/OstKost/avari-p3-express/apps/api/internal/domain"
	"github.com/OstKost/avari-p3-express/apps/api/internal/service"
	"sync"
	"testing"
)

func submitForQA(t *testing.T, s *service.WorkService, p string, required bool) *domain.WorkTask {
	t.Helper()
	v := createTask(t, s, p)
	spec := v.WorkSpec
	spec.RequiresIndependentReview = required
	v = cmd(t, s, manager, v, "edit", service.WorkCommand{Spec: &spec})
	v = cmd(t, s, manager, v, "ready", service.WorkCommand{})
	executor := domain.WorkActor{ID: "executor", Role: "executor", Projects: []string{p}}
	v = cmd(t, s, executor, v, "start", service.WorkCommand{Key: v.ID + "-start"})
	id := v.Runs[0].ID
	v = cmd(t, s, executor, v, "result", service.WorkCommand{RunID: id, Key: "result", Result: &domain.TaskResult{Description: "Output", Kind: "code", ArtifactVersion: "abc"}})
	return cmd(t, s, executor, v, "submit", service.WorkCommand{RunID: id, Key: "submit", Text: "Ready", Checks: []domain.CriterionCheck{{Criterion: 0, Outcome: "pass", Method: "reported test", Source: "executor", Evidence: "claimed log"}}})
}
func reportFor(v *domain.WorkTask) service.VerificationInput {
	return service.VerificationInput{SubmissionDigest: v.Runs[0].SubmissionDigest, Key: "verification", Kind: "qa", Summary: "Reproduced", Checks: []domain.CriterionCheck{{Criterion: 0, Outcome: "pass", Method: "independent test", Source: "reviewer", Evidence: "revision abc actual log"}}}
}
func TestIndependentVerificationGatesAndHistory(t *testing.T) {
	s, _, p, _ := setupWork(t)
	ctx := context.Background()
	v := submitForQA(t, s, p, true)
	run := v.Runs[0].ID
	digest := v.Runs[0].SubmissionDigest
	if digest == "" {
		t.Fatal("no digest")
	}
	reviewer := domain.WorkActor{ID: "reviewer", Name: "QA", Role: "reviewer", Projects: []string{p}}
	accept := service.WorkCommand{Version: v.Version, RunID: run, Decision: "accept"}
	if _, e := s.Command(ctx, manager, v.ID, "review", accept); !errors.Is(e, domain.ErrConflict) {
		t.Fatalf("missing QA: %v", e)
	}
	in := reportFor(v)
	in.Kind = "security"
	v, e := s.AddVerification(ctx, reviewer, run, in)
	if e != nil {
		t.Fatal(e)
	}
	accept.Version = v.Version
	if _, e = s.Command(ctx, manager, v.ID, "review", accept); !errors.Is(e, domain.ErrConflict) {
		t.Fatal("security substituted QA")
	}
	in = reportFor(v)
	in.Key = "qa"
	in.Checks[0].Outcome = "unknown"
	v, e = s.AddVerification(ctx, reviewer, run, in)
	if e != nil {
		t.Fatal(e)
	}
	n := len(v.Events)
	version := v.Version
	repeat, e := s.AddVerification(ctx, reviewer, run, in)
	if e != nil || repeat.Version != version || len(repeat.Events) != n {
		t.Fatal("repeat duplicated")
	}
	changed := in
	changed.Summary = "Changed"
	if _, e = s.AddVerification(ctx, reviewer, run, changed); !errors.Is(e, domain.ErrConflict) {
		t.Fatal("key payload overwritten")
	}
	accept.Version = v.Version
	if _, e = s.Command(ctx, manager, v.ID, "review", accept); !errors.Is(e, domain.ErrWorkInput) {
		t.Fatal("unknown accepted without reason")
	}
	v = cmd(t, s, manager, v, "review", service.WorkCommand{RunID: run, Decision: "return", Text: "Recheck revision"})
	if v.Runs[0].SubmissionDigest != digest || len(v.Runs[0].VerificationReports) != 2 {
		t.Fatal("history lost")
	}
	fresh := in
	fresh.Key = "late"
	if _, e = s.AddVerification(ctx, reviewer, run, fresh); !errors.Is(e, domain.ErrConflict) {
		t.Fatal("late report accepted")
	}
	spec := v.WorkSpec
	spec.ExpectedResult = "Revised output"
	v = cmd(t, s, manager, v, "edit", service.WorkCommand{Spec: &spec})
	v = cmd(t, s, manager, v, "start", service.WorkCommand{Key: "second"})
	if len(v.Runs[1].VerificationReports) != 0 {
		t.Fatal("inherited verification")
	}
	other := submitForQA(t, s, p, true)
	r := other.Runs[0].ID
	good := reportFor(other)
	other, e = s.AddVerification(ctx, reviewer, r, good)
	if e != nil {
		t.Fatal(e)
	}
	other = cmd(t, s, manager, other, "review", service.WorkCommand{RunID: r, Decision: "accept"})
	if other.Status != "done" {
		t.Fatal("not accepted")
	}
	if _, e = s.AddVerification(ctx, reviewer, r, good); e != nil {
		t.Fatal("successful retry after acceptance", e)
	}
	// Optional QA keeps v4 behavior but adverse independent evidence requires rationale.
	optional := submitForQA(t, s, p, false)
	bad := reportFor(optional)
	bad.Checks[0].Outcome = "fail"
	optional, e = s.AddVerification(ctx, reviewer, optional.Runs[0].ID, bad)
	if e != nil {
		t.Fatal(e)
	}
	optional = cmd(t, s, manager, optional, "review", service.WorkCommand{RunID: optional.Runs[0].ID, Decision: "accept", Text: "Known issue, accepted with scope limitation"})
	if optional.Status != "done" {
		t.Fatal("override failed")
	}
}
func TestVerificationRoleScopeDigestAndConcurrency(t *testing.T) {
	s, _, p, _ := setupWork(t)
	ctx := context.Background()
	v := submitForQA(t, s, p, true)
	run := v.Runs[0].ID
	in := reportFor(v)
	for _, a := range []domain.WorkActor{manager, {ID: "executor", Role: "executor", Projects: []string{p}}, {ID: "executor", Role: "reviewer", Projects: []string{p}}, {ID: "outside", Role: "reviewer", Projects: []string{"other"}}} {
		if _, e := s.AddVerification(ctx, a, run, in); !errors.Is(e, domain.ErrForbidden) {
			t.Fatalf("role scope %+v: %v", a, e)
		}
	}
	reviewer := domain.WorkActor{ID: "qa", Role: "reviewer", Projects: []string{p}}
	for _, op := range []string{"start", "progress", "result", "submit", "finish", "review", "edit", "cancel"} {
		if _, e := s.Command(ctx, reviewer, v.ID, op, service.WorkCommand{Version: v.Version, RunID: run}); !errors.Is(e, domain.ErrForbidden) {
			t.Fatalf("reviewer %s: %v", op, e)
		}
	}
	invalid := in
	invalid.SubmissionDigest = "wrong"
	if _, e := s.AddVerification(ctx, reviewer, run, invalid); !errors.Is(e, domain.ErrConflict) {
		t.Fatal("wrong digest")
	}
	invalid = in
	invalid.Checks = []domain.CriterionCheck{{Criterion: 9, Outcome: "pass", Method: "test", Source: "qa", Evidence: "log"}}
	if _, e := s.AddVerification(ctx, reviewer, run, invalid); !errors.Is(e, domain.ErrWorkInput) {
		t.Fatal("invalid criterion")
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := s.AddVerification(ctx, reviewer, run, in); errs <- e }()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	got, e := s.Get(ctx, manager, v.ID)
	if e != nil || len(got.Runs[0].VerificationReports) != 1 || got.Version != v.Version+1 {
		t.Fatal("concurrent duplicate report")
	}
	if got.Runs[0].SubmissionDigest != domain.SubmissionDigest(got.Runs[0]) {
		t.Fatal("review changed digest")
	}
}
