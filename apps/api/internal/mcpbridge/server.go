// Package mcpbridge exposes the bounded agent HTTP contract over stdio MCP.
package mcpbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/OstKost/avari-p3-express/apps/api/internal/domain"
	"github.com/OstKost/avari-p3-express/apps/api/internal/service"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Input struct {
	ProjectID string                  `json:"project_id,omitempty"`
	TaskID    string                  `json:"task_id,omitempty"`
	Status    string                  `json:"status,omitempty"`
	StageID   string                  `json:"stage_id,omitempty"`
	StepID    string                  `json:"step_id,omitempty"`
	Assignee  string                  `json:"assignee,omitempty"`
	Limit     int                     `json:"limit,omitempty"`
	Offset    int                     `json:"offset,omitempty"`
	Version   int                     `json:"version,omitempty"`
	RunID     string                  `json:"run_id,omitempty"`
	Key       string                  `json:"idempotency_key,omitempty"`
	Text      string                  `json:"text,omitempty"`
	Result    *domain.TaskResult      `json:"result,omitempty"`
	Checks    []domain.CriterionCheck `json:"checks,omitempty"`
}

func New(base, token string) (*mcp.Server, error) {
	u, e := url.Parse(base)
	if e != nil || u.Scheme != "http" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" && u.Hostname() != "::1") {
		return nil, fmt.Errorf("AVARI_API_URL must be a loopback HTTP URL")
	}
	if token == "" {
		return nil, fmt.Errorf("AVARI_AGENT_TOKEN required")
	}
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	server := mcp.NewServer(&mcp.Implementation{Name: "avari-agent", Version: "1.0.0"}, nil)
	for _, name := range []string{"list_projects", "get_project_context", "list_tasks", "get_task", "start_task", "report_progress", "report_blocker", "add_comment", "add_result", "submit_task", "finish_run"} {
		mcp.AddTool(server, &mcp.Tool{Name: name, Description: description(name)}, func(ctx context.Context, _ *mcp.CallToolRequest, in Input) (*mcp.CallToolResult, any, error) {
			method, path := "GET", ""
			var body any
			switch name {
			case "list_projects":
				path = "/work-projects"
			case "get_project_context":
				if in.ProjectID == "" {
					return nil, nil, fmt.Errorf("project_id required")
				}
				path = "/work-projects/" + url.PathEscape(in.ProjectID)
			case "list_tasks":
				path = "/work-tasks"
			case "get_task":
				if in.TaskID == "" {
					return nil, nil, fmt.Errorf("task_id required")
				}
				path = "/work-tasks/" + url.PathEscape(in.TaskID)
			default:
				if in.TaskID == "" {
					return nil, nil, fmt.Errorf("task_id required")
				}
				ops := map[string]string{"start_task": "start", "report_progress": "progress", "report_blocker": "blocker", "add_comment": "comment", "add_result": "result", "submit_task": "submit", "finish_run": "finish"}
				method = "POST"
				path = "/work-tasks/" + url.PathEscape(in.TaskID) + "/" + ops[name]
				body = service.WorkCommand{Version: in.Version, RunID: in.RunID, Key: in.Key, Text: in.Text, Result: in.Result, Checks: in.Checks}
			}
			if name == "list_tasks" || name == "list_projects" {
				q := url.Values{}
				for k, v := range map[string]string{"project_id": in.ProjectID, "status": in.Status, "stage_id": in.StageID, "step_id": in.StepID, "assignee": in.Assignee} {
					if v != "" {
						q.Set(k, v)
					}
				}
				if in.Limit != 0 {
					q.Set("limit", fmt.Sprint(in.Limit))
				}
				q.Set("offset", fmt.Sprint(in.Offset))
				path += "?" + q.Encode()
			}
			var buf bytes.Buffer
			if body != nil {
				if e := json.NewEncoder(&buf).Encode(body); e != nil {
					return nil, nil, e
				}
			}
			req, e := http.NewRequestWithContext(ctx, method, strings.TrimRight(base, "/")+"/api/v1"+path, &buf)
			if e != nil {
				return nil, nil, e
			}
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")
			resp, e := client.Do(req)
			if e != nil {
				return nil, nil, fmt.Errorf("Avari API unavailable")
			}
			defer resp.Body.Close()
			raw, e := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
			if e != nil {
				return nil, nil, e
			}
			if resp.StatusCode >= 300 {
				return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, raw)}}}, nil, nil
			}
			var out any
			if e = json.Unmarshal(raw, &out); e != nil {
				return nil, nil, e
			}
			return nil, out, nil
		})
	}
	mcp.AddTool(server, &mcp.Tool{Name: "report_verification", Description: "Reviewer-only: append independent checks for an exact submitted run and submission_digest. Does not accept work. Stable idempotency_key required."}, func(ctx context.Context, _ *mcp.CallToolRequest, in struct {
		RunID string `json:"run_id"`
		service.VerificationInput
	}) (*mcp.CallToolResult, any, error) {
		if in.RunID == "" {
			return nil, nil, fmt.Errorf("run_id required")
		}
		b, e := json.Marshal(in.VerificationInput)
		if e != nil {
			return nil, nil, e
		}
		req, e := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(base, "/")+"/api/v1/agent-runs/"+url.PathEscape(in.RunID)+"/verification-reports", bytes.NewReader(b))
		if e != nil {
			return nil, nil, e
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		resp, e := client.Do(req)
		if e != nil {
			return nil, nil, fmt.Errorf("Avari API unavailable")
		}
		defer resp.Body.Close()
		raw, e := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		if e != nil {
			return nil, nil, e
		}
		if resp.StatusCode >= 300 {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, raw)}}}, nil, nil
		}
		var out any
		if e = json.Unmarshal(raw, &out); e != nil {
			return nil, nil, e
		}
		return nil, out, nil
	})
	return server, nil
}
func description(name string) string {
	switch name {
	case "start_task":
		return "Atomically start ready task. Supply current version and stable idempotency_key. Identity comes from token."
	case "add_result":
		return "Attach text and HTTP(S) artifact links to active run. Stable idempotency_key required."
	case "submit_task":
		return "Submit report, results and one check per zero-based criterion. Checks are agent reports; manager acceptance is separate. Stable idempotency_key required."
	case "finish_run":
		return "Stop failed or cancelled active run with text reason, releasing task; does not accept work."
	case "get_task":
		return "Read task, project, exact step IDs, dependencies, previous attempts, results and review comments."
	default:
		return name + ": scoped to token projects. Mutations require version and run_id; report_blocker with empty text clears blocker."
	}
}
