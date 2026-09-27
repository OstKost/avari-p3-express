package domain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
)

var ErrWorkInput = errors.New("invalid work input")
var ErrForbidden = errors.New("access denied")

type WorkSpec struct {
	RequiresIndependentReview bool     `json:"requires_independent_review"`
	Goal                      string   `json:"goal"`
	ExpectedResult            string   `json:"expected_result"`
	Criteria                  []string `json:"criteria"`
	Priority                  string   `json:"priority"`
	DueDate                   string   `json:"due_date"`
	Assignee                  string   `json:"assignee"`
	StepIDs                   []string `json:"step_ids"`
	StageID                   string   `json:"stage_id"`
	Dependencies              []string `json:"dependencies"`
}
type WorkTask struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	WorkSpec
	Status  string      `json:"status"`
	Version int         `json:"version"`
	Blocker string      `json:"blocker"`
	Runs    []AgentRun  `json:"runs"`
	Events  []WorkEvent `json:"events"`
}
type AgentRun struct {
	SubmissionDigest    string               `json:"submission_digest"`
	VerificationReports []VerificationReport `json:"verification_reports"`
	ActorName           string               `json:"actor_name"`
	ID                  string               `json:"id"`
	TaskID              string               `json:"task_id"`
	Actor               string               `json:"actor"`
	State               string               `json:"state"`
	StartedAt           string               `json:"started_at"`
	EndedAt             string               `json:"ended_at"`
	LastMessageAt       string               `json:"last_message_at"`
	Snapshot            WorkSpec             `json:"snapshot"`
	Report              string               `json:"report"`
	Results             []TaskResult         `json:"results"`
	Checks              []CriterionCheck     `json:"checks"`
	Review              *ReviewDecision      `json:"review"`
	StartKey            string               `json:"start_key"`
	SubmitKey           string               `json:"submit_key"`
	SubmitFingerprint   string               `json:"submit_fingerprint"`
}
type TaskResult struct {
	ID              string   `json:"id,omitempty"`
	TaskID          string   `json:"task_id,omitempty"`
	RunID           string   `json:"run_id,omitempty"`
	Description     string   `json:"description"`
	Kind            string   `json:"kind"`
	URLs            []string `json:"urls"`
	ArtifactVersion string   `json:"artifact_version"`
	Key             string   `json:"idempotency_key,omitempty"`
	CreatedAt       string   `json:"created_at,omitempty"`
}
type CriterionCheck struct {
	Criterion int    `json:"criterion"`
	Outcome   string `json:"outcome"`
	Method    string `json:"method"`
	Source    string `json:"source"`
	Evidence  string `json:"evidence"`
}
type ReviewDecision struct {
	Decision  string `json:"decision"`
	Comment   string `json:"comment"`
	Actor     string `json:"actor"`
	CreatedAt string `json:"created_at"`
}
type WorkEvent struct {
	VerificationReportID string `json:"verification_report_id,omitempty"`
	ActorName            string `json:"actor_name"`
	ID                   string `json:"id"`
	TaskID               string `json:"task_id"`
	RunID                string `json:"run_id"`
	ResultID             string `json:"result_id"`
	Actor                string `json:"actor"`
	Kind                 string `json:"kind"`
	Text                 string `json:"text"`
	CreatedAt            string `json:"created_at"`
}
type WorkActor struct {
	Role     string
	Name     string
	ID       string
	Manager  bool
	Projects []string
}

func (a WorkActor) Allows(id string) bool {
	if a.Manager {
		return true
	}
	for _, p := range a.Projects {
		if p == id {
			return true
		}
	}
	return false
}

type AgentToken struct {
	Role     string   `json:"role"`
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Projects []string `json:"projects"`
	Hash     string   `json:"-"`
	Revoked  bool     `json:"revoked"`
}
type WorkStepContext struct {
	ID          string `json:"id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	CycleID     string `json:"cycle_id"`
	CycleNumber int    `json:"cycle_number"`
	PhaseCode   string `json:"phase_code"`
}
type WorkData struct {
	Tasks        []WorkTask
	Projects     []string
	Steps        map[string]string
	Stages       map[string]string
	StepContexts map[string]WorkStepContext
}

// Transact serializes use cases and persists changed aggregates with their events atomically.
type WorkRepository interface {
	Transact(context.Context, func(*WorkData) error) error
	Tokens(context.Context) ([]AgentToken, error)
	SaveToken(context.Context, AgentToken) error
}

type VerificationReport struct {
	ID               string           `json:"id"`
	TaskID           string           `json:"task_id"`
	RunID            string           `json:"run_id"`
	ReviewerID       string           `json:"reviewer_id"`
	ReviewerName     string           `json:"reviewer_name"`
	SubmissionDigest string           `json:"submission_digest"`
	Kind             string           `json:"kind"`
	Summary          string           `json:"summary"`
	Checks           []CriterionCheck `json:"checks"`
	CreatedAt        string           `json:"created_at"`
	Key              string           `json:"idempotency_key"`
}

// SubmissionDigest deliberately excludes review decisions and subsequent events.
func SubmissionDigest(r AgentRun) string {
	b, _ := json.Marshal(struct {
		Snapshot WorkSpec
		Results  []TaskResult
		Report   string
		Checks   []CriterionCheck
	}{r.Snapshot, r.Results, r.Report, r.Checks})
	digest := sha256.Sum256(b)
	return hex.EncodeToString(digest[:])
}
