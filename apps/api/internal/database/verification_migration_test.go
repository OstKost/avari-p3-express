package database

import (
	"encoding/json"
	"github.com/OstKost/avari-p3-express/apps/api/internal/domain"
	"github.com/pressly/goose/v3"
	"path/filepath"
	"testing"
)

func TestVerificationMigrationPreservesV4(t *testing.T) {
	db, e := NewConnection(filepath.Join(t.TempDir(), "v4.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	goose.SetBaseFS(embedMigrations)
	goose.SetDialect("sqlite3")
	if e = goose.UpTo(db, "migrations", 4); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(`INSERT INTO p3_projects(id,name,created_at) VALUES('p','Existing','now')`); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(`INSERT INTO agent_tokens VALUES('token','Legacy','["p"]','hash',0)`); e != nil {
		t.Fatal(e)
	}
	raw := `{"id":"task","project_id":"p","version":7,"goal":"Old goal","expected_result":"Old result","criteria":["Works"],"status":"in_review","runs":[{"id":"run","task_id":"task","actor":"old-executor","state":"submitted","snapshot":{"goal":"Old goal","expected_result":"Old result","criteria":["Works"]},"results":[{"id":"result","description":"Saved output","kind":"code","artifact_version":"abc"}],"report":"Original report","checks":[{"criterion":0,"outcome":"pass","method":"test","source":"executor","evidence":"old log"}],"submit_key":"key","submit_fingerprint":"old-fingerprint"}],"events":[]}`
	if _, e = db.Exec("INSERT INTO work_tasks VALUES('task','p',7,?)", raw); e != nil {
		t.Fatal(e)
	}
	if e = Migrate(db); e != nil {
		t.Fatal(e)
	}
	var role string
	if e = db.QueryRow("SELECT role FROM agent_tokens WHERE id='token'").Scan(&role); e != nil || role != "executor" {
		t.Fatal("token compatibility")
	}
	var body string
	db.QueryRow("SELECT body FROM work_tasks WHERE id='task'").Scan(&body)
	var task domain.WorkTask
	if e = json.Unmarshal([]byte(body), &task); e != nil {
		t.Fatal(e)
	}
	run := task.Runs[0]
	if task.Version != 7 || task.RequiresIndependentReview || run.SubmitFingerprint != "old-fingerprint" || run.Report != "Original report" || run.Results[0].ArtifactVersion != "abc" || len(run.VerificationReports) != 0 || run.SubmissionDigest != domain.SubmissionDigest(run) {
		t.Fatal("legacy data changed")
	}
	if e = Migrate(db); e != nil {
		t.Fatal(e)
	}
	var again string
	db.QueryRow("SELECT body FROM work_tasks WHERE id='task'").Scan(&again)
	if again != body {
		t.Fatal("backfill not idempotent")
	}
	var events int
	db.QueryRow("SELECT count(*) FROM work_events").Scan(&events)
	if events != 0 {
		t.Fatal("migration invented events")
	}
}
