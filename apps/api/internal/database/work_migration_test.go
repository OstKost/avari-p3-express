package database

import (
	"github.com/pressly/goose/v3"
	"path/filepath"
	"testing"
)

func TestWorkMigrationPreservesLegacy(t *testing.T) {
	db, e := NewConnection(filepath.Join(t.TempDir(), "legacy.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	goose.SetBaseFS(embedMigrations)
	goose.SetDialect("sqlite3")
	if e = goose.UpTo(db, "migrations", 3); e != nil {
		t.Fatal(e)
	}
	for _, query := range []string{
		`INSERT INTO p3_projects(id,name,progress,created_at) VALUES('p','Legacy',37,'2026-01-01')`,
		`INSERT INTO p3_cycles VALUES('c','p','B',1,'2026-01-01')`,
		`INSERT INTO p3_steps(id,cycle_id,code,name,status) VALUES('s','c','B1','Legacy step','done')`,
		`INSERT INTO p3_checklist(id,step_id,text,done) VALUES('check','s','Legacy criterion',1)`,
		`INSERT INTO p3_links VALUES('link','s','Legacy link','https://example.com','Keep')`,
		`INSERT INTO p3_actions VALUES('a','p','s','Legacy action',NULL,'high',1)`,
		`INSERT INTO p3_sdlc_stages(id,project_id,code,name,status,progress) VALUES('stage','p','dev','Develop','in_progress',60)`,
	} {
		if _, e = db.Exec(query); e != nil {
			t.Fatal(e)
		}
	}
	if e = Migrate(db); e != nil {
		t.Fatal(e)
	}
	for _, check := range []struct {
		query string
		want  int
	}{{"SELECT progress FROM p3_projects WHERE id='p'", 37}, {"SELECT done FROM p3_actions WHERE id='a'", 1}, {"SELECT done FROM p3_checklist WHERE id='check'", 1}, {"SELECT count(*) FROM p3_links", 1}, {"SELECT progress FROM p3_sdlc_stages", 60}, {"SELECT count(*) FROM work_tasks", 0}, {"SELECT count(*) FROM work_events", 0}} {
		var got int
		if e = db.QueryRow(check.query).Scan(&got); e != nil || got != check.want {
			t.Fatalf("%s got %d want %d: %v", check.query, got, check.want, e)
		}
	}
}
