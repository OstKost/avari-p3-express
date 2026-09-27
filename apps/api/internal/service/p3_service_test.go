package service

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/OstKost/avari-p3-express/apps/api/internal/database"
	"github.com/OstKost/avari-p3-express/apps/api/internal/domain"
	"github.com/OstKost/avari-p3-express/apps/api/internal/repository/sqlite"
)

func TestP3ProjectChecklistCyclePersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p3.db")
	db, err := database.NewConnection(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	var foreignKeys int
	if err = db.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil || foreignKeys != 1 {
		t.Fatalf("foreign keys disabled: value=%d err=%v", foreignKeys, err)
	}
	ctx := context.Background()
	svc := NewP3Service(sqlite.NewP3Repository(db))
	p, err := svc.CreateProject(ctx, map[string]any{"name": "Atlas"})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Phases) != 7 || len(p.SDLCStages) != 6 {
		t.Fatalf("seeded groups/stages: %d/%d", len(p.Phases), len(p.SDLCStages))
	}
	if len(p.Activity) != 8 {
		t.Fatalf("project and seven initial cycles should create 8 activity records, got %d", len(p.Activity))
	}
	total := 0
	for _, ph := range p.Phases {
		total += len(ph.Steps)
	}
	if total != 33 {
		t.Fatalf("activities = %d", total)
	}
	step := p.Phases[1].Steps[0]
	if len(step.Checklist) != 1 || step.Checklist[0].Done {
		t.Fatalf("expected one incomplete template item, got %+v", step.Checklist)
	}
	created, err := svc.Create(ctx, "checklist", step.ID, map[string]any{"text": "Review plan"})
	if err != nil {
		t.Fatal(err)
	}
	item := created.(domain.ChecklistItem)
	if _, err = db.Exec(`CREATE TRIGGER test_reject_activity BEFORE INSERT ON p3_activity BEGIN SELECT RAISE(ABORT, 'simulated activity write failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Patch(ctx, "checklist", item.ID, map[string]any{"done": true}); err == nil {
		t.Fatal("expected activity trigger failure")
	}
	var itemDone int
	if err = db.QueryRow(`SELECT done FROM p3_checklist WHERE id=?`, item.ID).Scan(&itemDone); err != nil || itemDone != 0 {
		t.Fatalf("checklist update committed without its activity: done=%d err=%v", itemDone, err)
	}
	if _, err = db.Exec(`DROP TRIGGER test_reject_activity`); err != nil {
		t.Fatal(err)
	}
	_, err = svc.Patch(ctx, "checklist", item.ID, map[string]any{"done": true})
	if err != nil {
		t.Fatal(err)
	}
	p, err = svc.GetProject(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if p.Phases[1].Steps[0].Progress != 50 || p.Phases[1].Progress != 10 {
		t.Fatalf("progress step/phase = %d/%d", p.Phases[1].Steps[0].Progress, p.Phases[1].Progress)
	}
	oldID := p.Phases[1].Steps[0].ID
	_, err = svc.CreateCycle(ctx, p.ID, "B")
	if err != nil {
		t.Fatal(err)
	}
	p, err = svc.GetProject(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if p.Phases[1].Steps[0].ID == oldID || p.Phases[1].Progress != 0 {
		t.Fatal("B cycle did not reset active steps")
	}
	cycles, err := svc.ListCycles(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(cycles) != 8 {
		t.Fatalf("cycles = %d", len(cycles))
	}
	bActive := p.Phases[1].Steps[0].ID
	if _, err = svc.CreateCycle(ctx, p.ID, "C"); err != nil {
		t.Fatal(err)
	}
	p, err = svc.GetProject(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if p.Phases[1].Steps[0].ID != bActive {
		t.Fatal("C cycle changed the active B cycle")
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = database.NewConnection(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	svc = NewP3Service(sqlite.NewP3Repository(db))
	p, err = svc.GetProject(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Phases[1].Steps) != 5 || p.Phases[1].Steps[0].ID == oldID {
		t.Fatal("active cycle was not persisted")
	}
	if len(p.Activity) < 12 {
		t.Fatalf("expected project, cycle, and checklist activity, got %d records", len(p.Activity))
	}
	var count int
	if err = db.QueryRow(`SELECT count(*) FROM p3_checklist WHERE id=? AND done=1`, item.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("historical checklist missing: count=%d err=%v", count, err)
	}
}
func TestP3Validation(t *testing.T) {
	db, err := database.NewConnection(filepath.Join(t.TempDir(), "p3.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	svc := NewP3Service(sqlite.NewP3Repository(db))
	p, err := svc.CreateProject(ctx, map[string]any{"name": "Atlas"})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		run  func() error
	}{{"bad date", func() error {
		_, e := svc.Patch(ctx, "project", p.ID, map[string]any{"due_date": "2026-02-30"})
		return e
	}}, {"bad url", func() error {
		_, e := svc.Create(ctx, "link", p.Phases[0].Steps[0].ID, map[string]any{"title": "Bad", "url": "javascript:alert(1)"})
		return e
	}}, {"out of range port", func() error {
		_, e := svc.Create(ctx, "link", p.Phases[0].Steps[0].ID, map[string]any{"title": "Bad", "url": "https://example.com:99999/"})
		return e
	}}, {"empty port", func() error {
		_, e := svc.Create(ctx, "link", p.Phases[0].Steps[0].ID, map[string]any{"title": "Bad", "url": "https://example.com:/"})
		return e
	}}, {"invalid cycle", func() error { _, e := svc.CreateCycle(ctx, p.ID, "A"); return e }}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if e := tc.run(); !errors.Is(e, ErrP3Input) {
				t.Fatalf("expected input error, got %v", e)
			}
		})
	}
}
