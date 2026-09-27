package domain

import "context"

type Project struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	Description  string      `json:"description"`
	DueDate      *string     `json:"due_date"`
	Archived     bool        `json:"archived"`
	RAGStatus    string      `json:"rag_status"`
	Progress     int         `json:"progress"`
	CurrentPhase string      `json:"current_phase"`
	Phases       []Phase     `json:"phases"`
	SDLCStages   []SDLCStage `json:"sdlc_stages"`
	Blockers     []Blocker   `json:"blockers"`
	Actions      []Action    `json:"actions"`
	Activity     []Activity  `json:"activity"`
}
type ProjectSummary struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Description      string  `json:"description"`
	DueDate          *string `json:"due_date"`
	Archived         bool    `json:"archived"`
	RAGStatus        string  `json:"rag_status"`
	Progress         int     `json:"progress"`
	CurrentPhase     string  `json:"current_phase"`
	CurrentSDLCStage string  `json:"current_sdlc_stage"`
	OpenBlockers     int     `json:"open_blockers"`
}
type Phase struct {
	ID          string  `json:"id"`
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Progress    int     `json:"progress"`
	DueDate     *string `json:"due_date"`
	Status      string  `json:"status"`
	Steps       []Step  `json:"steps"`
}
type Step struct {
	ID          string          `json:"id"`
	Code        string          `json:"code"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Status      string          `json:"status"`
	DueDate     *string         `json:"due_date"`
	Progress    int             `json:"progress"`
	Checklist   []ChecklistItem `json:"checklist"`
	Links       []ArtifactLink  `json:"links"`
	Comments    []Comment       `json:"comments"`
	Blockers    []Blocker       `json:"blockers"`
}
type ChecklistItem struct {
	ID     string `json:"id"`
	StepID string `json:"step_id"`
	Text   string `json:"text"`
	Done   bool   `json:"done"`
}
type ArtifactLink struct {
	ID          string `json:"id"`
	StepID      string `json:"step_id"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
}
type Comment struct {
	ID        string `json:"id"`
	StepID    string `json:"step_id"`
	Text      string `json:"text"`
	CreatedAt string `json:"created_at"`
}
type SDLCStage struct {
	ID        string  `json:"id"`
	Code      string  `json:"code"`
	Name      string  `json:"name"`
	Status    string  `json:"status"`
	Progress  int     `json:"progress"`
	StartDate *string `json:"start_date"`
	DueDate   *string `json:"due_date"`
}
type Blocker struct {
	ID          string  `json:"id"`
	ProjectID   string  `json:"project_id"`
	StepID      *string `json:"step_id"`
	SDLCStageID *string `json:"sdlc_stage_id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Resolved    bool    `json:"resolved"`
}
type Action struct {
	ID        string  `json:"id"`
	ProjectID string  `json:"project_id"`
	StepID    *string `json:"step_id"`
	Title     string  `json:"title"`
	DueDate   *string `json:"due_date"`
	Priority  string  `json:"priority"`
	Done      bool    `json:"done"`
}
type Article struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Category string `json:"category"`
	Summary  string `json:"summary"`
	Body     string `json:"body"`
}
type Activity struct {
	ID          string  `json:"id"`
	ProjectID   string  `json:"project_id"`
	StepID      *string `json:"step_id"`
	Kind        string  `json:"kind"`
	Description string  `json:"description"`
	CreatedAt   string  `json:"created_at"`
}
type Cycle struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	PhaseCode string `json:"phase_code"`
	Number    int    `json:"number"`
	CreatedAt string `json:"created_at"`
}

type P3Repository interface {
	CreateProject(context.Context, Project) error
	GetProject(context.Context, string) (*Project, error)
	ListProjects(context.Context) ([]ProjectSummary, error)
	PatchProject(context.Context, string, map[string]any) error
	CreateCycle(context.Context, string, string) (*Cycle, error)
	ListCycles(context.Context, string) ([]Cycle, error)
	PatchStep(context.Context, string, map[string]any) error
	CreateChecklist(context.Context, string, ChecklistItem) error
	PatchChecklist(context.Context, string, map[string]any) error
	DeleteChecklist(context.Context, string) error
	CreateLink(context.Context, string, ArtifactLink) error
	DeleteLink(context.Context, string) error
	CreateComment(context.Context, string, Comment) error
	CreateBlocker(context.Context, Blocker) error
	PatchBlocker(context.Context, string, map[string]any) error
	PatchSDLCStage(context.Context, string, map[string]any) error
	ListActions(context.Context) ([]Action, error)
	CreateAction(context.Context, Action) error
	PatchAction(context.Context, string, map[string]any) error
	ListArticles(context.Context) ([]Article, error)
	CreateArticle(context.Context, Article) error
	PatchArticle(context.Context, string, map[string]any) error
	FindProjectFor(context.Context, string, string) (string, error)
}
