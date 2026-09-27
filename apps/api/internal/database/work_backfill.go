package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/OstKost/avari-p3-express/apps/api/internal/domain"
)

// backfillWorkMetadata preserves legacy payloads and versions while assigning
// deterministic digests to v4 submissions. No work decision or event is invented.
func backfillWorkMetadata(ctx context.Context, db *sql.DB) error {
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	rows, e := tx.QueryContext(ctx, "SELECT id,body FROM work_tasks")
	if e != nil {
		return e
	}
	updates := map[string]string{}
	for rows.Next() {
		var id, raw string
		if e = rows.Scan(&id, &raw); e != nil {
			rows.Close()
			return e
		}
		var task domain.WorkTask
		if e = json.Unmarshal([]byte(raw), &task); e != nil {
			rows.Close()
			return e
		}
		changed := false
		for i := range task.Runs {
			r := &task.Runs[i]
			if r.VerificationReports == nil {
				r.VerificationReports = []domain.VerificationReport{}
				changed = true
			}
			if r.State == "submitted" && r.SubmissionDigest == "" {
				r.SubmissionDigest = domain.SubmissionDigest(*r)
				changed = true
			}
		}
		if changed {
			b, e := json.Marshal(task)
			if e != nil {
				rows.Close()
				return e
			}
			updates[id] = string(b)
		}
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for id, body := range updates {
		if _, e = tx.ExecContext(ctx, "UPDATE work_tasks SET body=? WHERE id=?", body, id); e != nil {
			return e
		}
	}
	return tx.Commit()
}
