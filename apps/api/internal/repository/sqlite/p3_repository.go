package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/OstKost/avari-p3-express/apps/api/internal/domain"
	"github.com/google/uuid"
)

type P3Repository struct{ db *sql.DB }

func NewP3Repository(db *sql.DB) *P3Repository { return &P3Repository{db: db} }
func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}
func mustRows(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}
func nullable(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}
func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }
func (r *P3Repository) CreateProject(ctx context.Context, p domain.Project) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO p3_projects(id,name,description,due_date,archived,rag_status,progress,current_phase,created_at) VALUES(?,?,?,?,?,?,?,?,?)`, p.ID, p.Name, p.Description, nullable(p.DueDate), p.Archived, p.RAGStatus, p.Progress, p.CurrentPhase, now())
	if err != nil {
		return err
	}
	for _, phase := range domain.P3Template {
		if err := createCycle(ctx, tx, p.ID, phase.Code, 1); err != nil {
			return err
		}
	}
	for i, name := range domain.SDLCNames {
		_, err = tx.ExecContext(ctx, `INSERT INTO p3_sdlc_stages(id,project_id,code,name) VALUES(?,?,?,?)`, uuid.NewString(), p.ID, fmt.Sprintf("%d", i+1), name)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
func createCycle(ctx context.Context, tx *sql.Tx, projectID, code string, number int) error {
	var phase *struct {
		Code, Name string
		Steps      []string
	}
	for i := range domain.P3Template {
		if domain.P3Template[i].Code == code {
			phase = &domain.P3Template[i]
			break
		}
	}
	if phase == nil {
		return fmt.Errorf("unknown phase")
	}
	id := uuid.NewString()
	_, err := tx.ExecContext(ctx, `INSERT INTO p3_cycles(id,project_id,phase_code,number,created_at) VALUES(?,?,?,?,?)`, id, projectID, code, number, now())
	if err != nil {
		return err
	}
	for i, name := range phase.Steps {
		stepID := uuid.NewString()
		_, err = tx.ExecContext(ctx, `INSERT INTO p3_steps(id,cycle_id,code,name) VALUES(?,?,?,?)`, stepID, id, fmt.Sprintf("%s%02d", code, i+1), name)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO p3_checklist(id,step_id,text,is_template) VALUES(?,?,?,1)`, uuid.NewString(), stepID, name)
		if err != nil {
			return err
		}
	}
	return nil
}
func (r *P3Repository) CreateCycle(ctx context.Context, projectID, code string) (*domain.Cycle, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var number int
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(number),0)+1 FROM p3_cycles WHERE project_id=? AND phase_code=?`, projectID, code).Scan(&number)
	if err != nil {
		return nil, err
	}
	if number == 1 {
		return nil, domain.ErrNotFound
	}
	if err = createCycle(ctx, tx, projectID, code, number); err != nil {
		return nil, err
	}
	var c domain.Cycle
	err = tx.QueryRowContext(ctx, `SELECT id,project_id,phase_code,number,created_at FROM p3_cycles WHERE project_id=? AND phase_code=? AND number=?`, projectID, code, number).Scan(&c.ID, &c.ProjectID, &c.PhaseCode, &c.Number, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &c, nil
}
func (r *P3Repository) ListCycles(ctx context.Context, id string) ([]domain.Cycle, error) {
	var exists int
	if err := r.db.QueryRowContext(ctx, `SELECT 1 FROM p3_projects WHERE id=?`, id).Scan(&exists); err != nil {
		return nil, notFound(err)
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id,project_id,phase_code,number,created_at FROM p3_cycles WHERE project_id=? ORDER BY phase_code,number`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Cycle{}
	for rows.Next() {
		var c domain.Cycle
		if err := rows.Scan(&c.ID, &c.ProjectID, &c.PhaseCode, &c.Number, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func scanProject(row *sql.Row) (*domain.Project, error) {
	p := &domain.Project{}
	var due sql.NullString
	err := row.Scan(&p.ID, &p.Name, &p.Description, &due, &p.Archived, &p.RAGStatus, &p.Progress, &p.CurrentPhase)
	if err != nil {
		return nil, notFound(err)
	}
	if due.Valid {
		p.DueDate = &due.String
	}
	p.Phases = []domain.Phase{}
	p.SDLCStages = []domain.SDLCStage{}
	p.Blockers = []domain.Blocker{}
	p.Actions = []domain.Action{}
	p.Activity = []domain.Activity{}
	return p, nil
}
func (r *P3Repository) GetProject(ctx context.Context, id string) (*domain.Project, error) {
	p, err := scanProject(r.db.QueryRowContext(ctx, `SELECT id,name,description,due_date,archived,rag_status,progress,current_phase FROM p3_projects WHERE id=?`, id))
	if err != nil {
		return nil, err
	}
	for _, tpl := range domain.P3Template {
		phase := domain.Phase{Code: tpl.Code, Name: tpl.Name, Description: tpl.Name, Status: "todo", Steps: []domain.Step{}}
		var cycleID string
		err := r.db.QueryRowContext(ctx, `SELECT id FROM p3_cycles WHERE project_id=? AND phase_code=? ORDER BY number DESC LIMIT 1`, id, tpl.Code).Scan(&cycleID)
		if err != nil {
			return nil, err
		}
		phase.ID = cycleID
		rows, err := r.db.QueryContext(ctx, `SELECT id,code,name,description,status,due_date FROM p3_steps WHERE cycle_id=? ORDER BY code`, cycleID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			s := domain.Step{Checklist: []domain.ChecklistItem{}, Links: []domain.ArtifactLink{}, Comments: []domain.Comment{}, Blockers: []domain.Blocker{}}
			var due sql.NullString
			if err = rows.Scan(&s.ID, &s.Code, &s.Name, &s.Description, &s.Status, &due); err != nil {
				rows.Close()
				return nil, err
			}
			if due.Valid {
				s.DueDate = &due.String
			}
			phase.Steps = append(phase.Steps, s)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
		for i := range phase.Steps {
			s := &phase.Steps[i]
			if err = r.loadStep(ctx, s); err != nil {
				return nil, err
			}
			phase.Progress += s.Progress
		}
		if len(phase.Steps) > 0 {
			phase.Progress /= len(phase.Steps)
		}
		if phase.Progress == 100 {
			phase.Status = "done"
		} else {
			for _, s := range phase.Steps {
				if s.Status != "todo" || s.Progress > 0 {
					phase.Status = "in_progress"
					break
				}
			}
		}
		p.Phases = append(p.Phases, phase)
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id,code,name,status,progress,start_date,due_date FROM p3_sdlc_stages WHERE project_id=? ORDER BY code`, id)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var x domain.SDLCStage
		var start, due sql.NullString
		if err = rows.Scan(&x.ID, &x.Code, &x.Name, &x.Status, &x.Progress, &start, &due); err != nil {
			rows.Close()
			return nil, err
		}
		if start.Valid {
			x.StartDate = &start.String
		}
		if due.Valid {
			x.DueDate = &due.String
		}
		p.SDLCStages = append(p.SDLCStages, x)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	p.Blockers, err = r.projectBlockers(ctx, id)
	if err != nil {
		return nil, err
	}
	p.Actions, err = r.actions(ctx, id)
	if err != nil {
		return nil, err
	}
	p.Activity, err = r.activity(ctx, id)
	if err != nil {
		return nil, err
	}
	return p, nil
}
func (r *P3Repository) loadStep(ctx context.Context, s *domain.Step) error {
	rows, err := r.db.QueryContext(ctx, `SELECT id,step_id,text,done FROM p3_checklist WHERE step_id=? ORDER BY rowid`, s.ID)
	if err != nil {
		return err
	}
	done := 0
	for rows.Next() {
		var x domain.ChecklistItem
		if err = rows.Scan(&x.ID, &x.StepID, &x.Text, &x.Done); err != nil {
			rows.Close()
			return err
		}
		if x.Done {
			done++
		}
		s.Checklist = append(s.Checklist, x)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(s.Checklist) > 0 {
		s.Progress = done * 100 / len(s.Checklist)
	}
	rows, err = r.db.QueryContext(ctx, `SELECT id,step_id,title,url,description FROM p3_links WHERE step_id=? ORDER BY rowid`, s.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var x domain.ArtifactLink
		if err = rows.Scan(&x.ID, &x.StepID, &x.Title, &x.URL, &x.Description); err != nil {
			rows.Close()
			return err
		}
		s.Links = append(s.Links, x)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	rows, err = r.db.QueryContext(ctx, `SELECT id,step_id,text,created_at FROM p3_comments WHERE step_id=? ORDER BY created_at`, s.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var x domain.Comment
		if err = rows.Scan(&x.ID, &x.StepID, &x.Text, &x.CreatedAt); err != nil {
			rows.Close()
			return err
		}
		s.Comments = append(s.Comments, x)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	rows, err = r.db.QueryContext(ctx, `SELECT id,project_id,step_id,sdlc_stage_id,title,description,resolved FROM p3_blockers WHERE step_id=?`, s.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		x, scanErr := scanBlocker(rows)
		if scanErr != nil {
			rows.Close()
			return scanErr
		}
		s.Blockers = append(s.Blockers, x)
	}
	err = rows.Err()
	rows.Close()
	return err
}
func scanBlocker(rows *sql.Rows) (domain.Blocker, error) {
	var x domain.Blocker
	var step, stage sql.NullString
	err := rows.Scan(&x.ID, &x.ProjectID, &step, &stage, &x.Title, &x.Description, &x.Resolved)
	if step.Valid {
		x.StepID = &step.String
	}
	if stage.Valid {
		x.SDLCStageID = &stage.String
	}
	return x, err
}
func (r *P3Repository) projectBlockers(ctx context.Context, id string) ([]domain.Blocker, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,project_id,step_id,sdlc_stage_id,title,description,resolved FROM p3_blockers WHERE project_id=? ORDER BY rowid`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Blocker{}
	for rows.Next() {
		x, e := scanBlocker(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r *P3Repository) actions(ctx context.Context, id string) ([]domain.Action, error) {
	query := `SELECT id,project_id,step_id,title,due_date,priority,done FROM p3_actions`
	args := []any{}
	if id != "" {
		query += ` WHERE project_id=?`
		args = append(args, id)
	}
	query += ` ORDER BY rowid`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Action{}
	for rows.Next() {
		var x domain.Action
		var step, due sql.NullString
		if err = rows.Scan(&x.ID, &x.ProjectID, &step, &x.Title, &due, &x.Priority, &x.Done); err != nil {
			return nil, err
		}
		if step.Valid {
			x.StepID = &step.String
		}
		if due.Valid {
			x.DueDate = &due.String
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r *P3Repository) activity(ctx context.Context, id string) ([]domain.Activity, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,project_id,step_id,kind,description,created_at FROM p3_activity WHERE project_id=? ORDER BY created_at DESC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Activity{}
	for rows.Next() {
		var x domain.Activity
		var step sql.NullString
		if err = rows.Scan(&x.ID, &x.ProjectID, &step, &x.Kind, &x.Description, &x.CreatedAt); err != nil {
			return nil, err
		}
		if step.Valid {
			x.StepID = &step.String
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r *P3Repository) ListProjects(ctx context.Context) ([]domain.ProjectSummary, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT p.id,p.name,p.description,p.due_date,p.archived,p.rag_status,p.progress,p.current_phase,COALESCE((SELECT name FROM p3_sdlc_stages WHERE project_id=p.id AND status='in_progress' ORDER BY code LIMIT 1),(SELECT name FROM p3_sdlc_stages WHERE project_id=p.id AND status!='done' ORDER BY code LIMIT 1),''),(SELECT count(*) FROM p3_blockers WHERE project_id=p.id AND resolved=0) FROM p3_projects p ORDER BY p.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.ProjectSummary{}
	for rows.Next() {
		var x domain.ProjectSummary
		var due sql.NullString
		if err = rows.Scan(&x.ID, &x.Name, &x.Description, &due, &x.Archived, &x.RAGStatus, &x.Progress, &x.CurrentPhase, &x.CurrentSDLCStage, &x.OpenBlockers); err != nil {
			return nil, err
		}
		if due.Valid {
			x.DueDate = &due.String
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r *P3Repository) ListActions(ctx context.Context) ([]domain.Action, error) {
	return r.actions(ctx, "")
}
func (r *P3Repository) ListArticles(ctx context.Context) ([]domain.Article, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,title,category,summary,body FROM p3_articles ORDER BY rowid DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Article{}
	for rows.Next() {
		var x domain.Article
		if err = rows.Scan(&x.ID, &x.Title, &x.Category, &x.Summary, &x.Body); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r *P3Repository) FindProjectFor(ctx context.Context, kind, id string) (string, error) {
	queries := map[string]string{"project": `SELECT id FROM p3_projects WHERE id=?`, "step": `SELECT c.project_id FROM p3_steps s JOIN p3_cycles c ON c.id=s.cycle_id WHERE s.id=?`, "checklist": `SELECT c.project_id FROM p3_checklist i JOIN p3_steps s ON s.id=i.step_id JOIN p3_cycles c ON c.id=s.cycle_id WHERE i.id=?`, "link": `SELECT c.project_id FROM p3_links l JOIN p3_steps s ON s.id=l.step_id JOIN p3_cycles c ON c.id=s.cycle_id WHERE l.id=?`, "blocker": `SELECT project_id FROM p3_blockers WHERE id=?`, "sdlc_stage": `SELECT project_id FROM p3_sdlc_stages WHERE id=?`, "action": `SELECT project_id FROM p3_actions WHERE id=?`}
	q, ok := queries[kind]
	if !ok {
		return "", fmt.Errorf("invalid entity kind")
	}
	var projectID string
	err := r.db.QueryRowContext(ctx, q, id).Scan(&projectID)
	return projectID, notFound(err)
}
func (r *P3Repository) CreateChecklist(ctx context.Context, step string, x domain.ChecklistItem) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO p3_checklist(id,step_id,text,done) VALUES(?,?,?,?)`, x.ID, step, x.Text, x.Done)
	return err
}
func (r *P3Repository) DeleteChecklist(ctx context.Context, id string) error {
	return mustRows(r.db.ExecContext(ctx, `DELETE FROM p3_checklist WHERE id=?`, id))
}
func (r *P3Repository) CreateLink(ctx context.Context, step string, x domain.ArtifactLink) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO p3_links(id,step_id,title,url,description) VALUES(?,?,?,?,?)`, x.ID, step, x.Title, x.URL, x.Description)
	return err
}
func (r *P3Repository) DeleteLink(ctx context.Context, id string) error {
	return mustRows(r.db.ExecContext(ctx, `DELETE FROM p3_links WHERE id=?`, id))
}
func (r *P3Repository) CreateComment(ctx context.Context, step string, x domain.Comment) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO p3_comments(id,step_id,text,created_at) VALUES(?,?,?,?)`, x.ID, step, x.Text, x.CreatedAt)
	return err
}
func (r *P3Repository) CreateBlocker(ctx context.Context, x domain.Blocker) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO p3_blockers(id,project_id,step_id,sdlc_stage_id,title,description,resolved) VALUES(?,?,?,?,?,?,?)`, x.ID, x.ProjectID, nullable(x.StepID), nullable(x.SDLCStageID), x.Title, x.Description, x.Resolved)
	return err
}
func (r *P3Repository) CreateAction(ctx context.Context, x domain.Action) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO p3_actions(id,project_id,step_id,title,due_date,priority,done) VALUES(?,?,?,?,?,?,?)`, x.ID, x.ProjectID, nullable(x.StepID), x.Title, nullable(x.DueDate), x.Priority, x.Done)
	return err
}
func (r *P3Repository) CreateArticle(ctx context.Context, x domain.Article) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO p3_articles(id,title,category,summary,body) VALUES(?,?,?,?,?)`, x.ID, x.Title, x.Category, x.Summary, x.Body)
	return err
}
func (r *P3Repository) PatchProject(ctx context.Context, id string, m map[string]any) error {
	return r.patch(ctx, "p3_projects", id, m)
}
func (r *P3Repository) PatchStep(ctx context.Context, id string, m map[string]any) error {
	return r.patch(ctx, "p3_steps", id, m)
}
func (r *P3Repository) PatchChecklist(ctx context.Context, id string, m map[string]any) error {
	return r.patch(ctx, "p3_checklist", id, m)
}
func (r *P3Repository) PatchBlocker(ctx context.Context, id string, m map[string]any) error {
	return r.patch(ctx, "p3_blockers", id, m)
}
func (r *P3Repository) PatchSDLCStage(ctx context.Context, id string, m map[string]any) error {
	return r.patch(ctx, "p3_sdlc_stages", id, m)
}
func (r *P3Repository) PatchAction(ctx context.Context, id string, m map[string]any) error {
	return r.patch(ctx, "p3_actions", id, m)
}
func (r *P3Repository) PatchArticle(ctx context.Context, id string, m map[string]any) error {
	return r.patch(ctx, "p3_articles", id, m)
}
func (r *P3Repository) patch(ctx context.Context, table, id string, m map[string]any) error {
	if len(m) == 0 {
		return fmt.Errorf("empty patch")
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	sets := make([]string, 0, len(keys))
	args := make([]any, 0, len(keys)+1)
	for _, k := range keys {
		sets = append(sets, k+"=?")
		args = append(args, m[k])
	}
	args = append(args, id)
	return mustRows(r.db.ExecContext(ctx, "UPDATE "+table+" SET "+strings.Join(sets, ",")+" WHERE id=?", args...))
}
