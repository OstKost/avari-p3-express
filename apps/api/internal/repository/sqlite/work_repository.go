package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"sync"

	"github.com/OstKost/avari-p3-express/apps/api/internal/domain"
)

type WorkRepository struct {
	db *sql.DB
	mu sync.Mutex
}

func NewWorkRepository(db *sql.DB) *WorkRepository { return &WorkRepository{db: db} }
func (r *WorkRepository) Transact(ctx context.Context, fn func(*domain.WorkData) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	d := domain.WorkData{Tasks: []domain.WorkTask{}, Steps: map[string]string{}, Stages: map[string]string{}}
	rows, err := tx.QueryContext(ctx, "SELECT body FROM work_tasks ORDER BY rowid")
	if err != nil {
		return err
	}
	old := map[string]string{}
	for rows.Next() {
		var body string
		if err = rows.Scan(&body); err != nil {
			rows.Close()
			return err
		}
		var t domain.WorkTask
		if err = json.Unmarshal([]byte(body), &t); err != nil {
			rows.Close()
			return err
		}
		d.Tasks = append(d.Tasks, t)
		old[t.ID] = body
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, q := range []struct {
		sql  string
		dest map[string]string
	}{{"SELECT s.id,c.project_id FROM p3_steps s JOIN p3_cycles c ON c.id=s.cycle_id", d.Steps}, {"SELECT id,project_id FROM p3_sdlc_stages", d.Stages}, {"SELECT id,id FROM p3_projects", nil}} {
		rows, err = tx.QueryContext(ctx, q.sql)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id, p string
			if err = rows.Scan(&id, &p); err != nil {
				rows.Close()
				return err
			}
			if q.dest == nil {
				d.Projects = append(d.Projects, id)
			} else {
				q.dest[id] = p
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	d.StepContexts = map[string]domain.WorkStepContext{}
	rows, err = tx.QueryContext(ctx, "SELECT s.id,s.code,s.name,c.id,c.number,c.phase_code FROM p3_steps s JOIN p3_cycles c ON c.id=s.cycle_id")
	if err != nil {
		return err
	}
	for rows.Next() {
		var ref domain.WorkStepContext
		if err = rows.Scan(&ref.ID, &ref.Code, &ref.Name, &ref.CycleID, &ref.CycleNumber, &ref.PhaseCode); err != nil {
			rows.Close()
			return err
		}
		d.StepContexts[ref.ID] = ref
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if err = fn(&d); err != nil {
		return err
	}
	for _, t := range d.Tasks {
		b, e := json.Marshal(t)
		if e != nil {
			return e
		}
		if old[t.ID] == string(b) {
			continue
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO work_tasks(id,project_id,version,body) VALUES(?,?,?,?) ON CONFLICT(id) DO UPDATE SET version=excluded.version,body=excluded.body", t.ID, t.ProjectID, t.Version, string(b)); err != nil {
			return err
		}
		for _, ev := range t.Events {
			eb, _ := json.Marshal(ev)
			if _, err = tx.ExecContext(ctx, "INSERT OR IGNORE INTO work_events VALUES(?,?,?,?,?,?,?,?)", ev.ID, t.ID, ev.Actor, ev.RunID, ev.ResultID, ev.Kind, ev.CreatedAt, string(eb)); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}
func (r *WorkRepository) Tokens(ctx context.Context) ([]domain.AgentToken, error) {
	rows, e := r.db.QueryContext(ctx, "SELECT id,name,projects,hash,revoked,role FROM agent_tokens ORDER BY rowid")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []domain.AgentToken{}
	for rows.Next() {
		var t domain.AgentToken
		var p string
		if e = rows.Scan(&t.ID, &t.Name, &p, &t.Hash, &t.Revoked, &t.Role); e != nil {
			return nil, e
		}
		if e = json.Unmarshal([]byte(p), &t.Projects); e != nil {
			return nil, e
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
func (r *WorkRepository) SaveToken(ctx context.Context, t domain.AgentToken) error {
	p, _ := json.Marshal(t.Projects)
	_, e := r.db.ExecContext(ctx, "INSERT INTO agent_tokens(id,name,projects,hash,revoked,role) VALUES(?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET revoked=excluded.revoked", t.ID, t.Name, string(p), t.Hash, t.Revoked, t.Role)
	return e
}
