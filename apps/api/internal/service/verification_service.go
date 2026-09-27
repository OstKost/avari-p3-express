package service

import (
	"context"
	"fmt"
	"github.com/OstKost/avari-p3-express/apps/api/internal/domain"
	"github.com/google/uuid"
	"strings"
)

// VerificationInput contains only reviewer-supplied fields; identity and time are server-owned.
type VerificationInput struct {
	SubmissionDigest string                  `json:"submission_digest"`
	Kind             string                  `json:"kind"`
	Summary          string                  `json:"summary"`
	Checks           []domain.CriterionCheck `json:"checks"`
	Key              string                  `json:"idempotency_key"`
}

func acceptanceGate(run domain.AgentRun, comment string) error {
	fullQA := false
	issues := false
	for _, report := range run.VerificationReports {
		if report.SubmissionDigest != run.SubmissionDigest {
			continue
		}
		if report.Kind == "qa" && len(report.Checks) == len(run.Snapshot.Criteria) {
			fullQA = true
		}
		for _, c := range report.Checks {
			if c.Outcome != "pass" {
				issues = true
			}
		}
	}
	if run.Snapshot.RequiresIndependentReview && !fullQA {
		return fmtConflict("independent QA report required")
	}
	if issues && strings.TrimSpace(comment) == "" {
		return workInput("accepting failed or unverified independent checks requires manager comment")
	}
	return nil
}
func fmtConflict(message string) error { return fmt.Errorf("%w: %s", domain.ErrConflict, message) }
func (s *WorkService) AddVerification(ctx context.Context, a domain.WorkActor, runID string, in VerificationInput) (*domain.WorkTask, error) {
	if a.Manager || a.Role != "reviewer" {
		return nil, domain.ErrForbidden
	}
	var out *domain.WorkTask
	err := s.repo.Transact(ctx, func(d *domain.WorkData) error {
		var task *domain.WorkTask
		var run *domain.AgentRun
		for i := range d.Tasks {
			for j := range d.Tasks[i].Runs {
				if d.Tasks[i].Runs[j].ID == runID {
					task = &d.Tasks[i]
					run = &task.Runs[j]
					break
				}
			}
			if run != nil {
				break
			}
		}
		if run == nil {
			return domain.ErrNotFound
		}
		if !a.Allows(task.ProjectID) || a.ID == run.Actor {
			return domain.ErrForbidden
		}
		if strings.TrimSpace(in.Key) == "" {
			return workInput("idempotency_key required")
		}
		for _, report := range run.VerificationReports {
			if report.Key == in.Key {
				if report.ReviewerID != a.ID || fingerprint(VerificationInput{SubmissionDigest: report.SubmissionDigest, Kind: report.Kind, Summary: report.Summary, Checks: report.Checks, Key: report.Key}) != fingerprint(in) {
					return domain.ErrConflict
				}
				out = task
				return nil
			}
		}
		if task.Status != "in_review" || run.State != "submitted" || run.Review != nil || in.SubmissionDigest == "" || in.SubmissionDigest != run.SubmissionDigest {
			return domain.ErrConflict
		}
		if (in.Kind != "qa" && in.Kind != "security") || strings.TrimSpace(in.Summary) == "" || len(in.Checks) == 0 {
			return workInput("kind, summary and criterion checks required")
		}
		seen := map[int]bool{}
		for _, c := range in.Checks {
			if c.Criterion < 0 || c.Criterion >= len(run.Snapshot.Criteria) || seen[c.Criterion] || strings.TrimSpace(c.Method) == "" || strings.TrimSpace(c.Source) == "" || strings.TrimSpace(c.Evidence) == "" || (c.Outcome != "pass" && c.Outcome != "fail" && c.Outcome != "unknown") {
				return workInput("invalid verification criterion")
			}
			seen[c.Criterion] = true
		}
		if in.Kind == "qa" && len(in.Checks) != len(run.Snapshot.Criteria) {
			return workInput("QA must record every criterion, using unknown for unverified criteria")
		}
		report := domain.VerificationReport{ID: uuid.NewString(), TaskID: task.ID, RunID: run.ID, ReviewerID: a.ID, ReviewerName: a.Name, SubmissionDigest: in.SubmissionDigest, Kind: in.Kind, Summary: in.Summary, Checks: in.Checks, Key: in.Key, CreatedAt: workNow()}
		run.VerificationReports = append(run.VerificationReports, report)
		event(task, a, "verification", in.Summary, run.ID, "")
		task.Events[len(task.Events)-1].VerificationReportID = report.ID
		out = task
		return nil
	})
	return out, err
}
