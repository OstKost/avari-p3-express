package mcpbridge_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OstKost/avari-p3-express/apps/api/internal/database"
	"github.com/OstKost/avari-p3-express/apps/api/internal/domain"
	"github.com/OstKost/avari-p3-express/apps/api/internal/handler"
	"github.com/OstKost/avari-p3-express/apps/api/internal/repository/sqlite"
	"github.com/OstKost/avari-p3-express/apps/api/internal/service"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestStdioAgentWorkflowAndHTTPAccess(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	db, e := database.NewConnection(filepath.Join(t.TempDir(), "test.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if e = database.Migrate(db); e != nil {
		t.Fatal(e)
	}
	p3 := service.NewP3Service(sqlite.NewP3Repository(db))
	work := service.NewWorkService(sqlite.NewWorkRepository(db))
	manager := domain.WorkActor{ID: "manager", Manager: true}
	key := strings.Repeat("m", 64)
	h := handler.NewWorkHandler(work, p3, []string{"http://localhost:4810"}, key)
	srv := httptest.NewServer(handler.NewRouter(handler.RouterConfig{DB: db, P3Handler: handler.NewP3Handler(p3), WorkHandler: h, AllowedOrigins: []string{"http://localhost:4810"}}))
	defer srv.Close()
	request := func(method, path string, body any, bearer string, cookie *http.Cookie) (int, []byte, *http.Response) {
		t.Helper()
		b, _ := json.Marshal(body)
		req, _ := http.NewRequestWithContext(ctx, method, srv.URL+path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Avari-Local", "manager")
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		if cookie != nil {
			req.AddCookie(cookie)
		}
		resp, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return resp.StatusCode, raw, resp
	}
	if code, _, _ := request("POST", "/api/session", map[string]string{}, "", nil); code != 401 {
		t.Fatalf("bootstrap bypass: %d", code)
	}
	code, _, resp := request("POST", "/api/session", map[string]string{"key": key}, "", nil)
	if code != 200 {
		t.Fatal(code)
	}
	cookie := resp.Cookies()[0]
	project, e := p3.CreateProject(ctx, map[string]any{"name": "MCP project"})
	if e != nil {
		t.Fatal(e)
	}
	other, e := p3.CreateProject(ctx, map[string]any{"name": "Forbidden"})
	if e != nil {
		t.Fatal(e)
	}
	code, raw, _ := request("POST", "/api/v1/work-tasks", map[string]any{"project_id": project.ID, "goal": "Ship code", "expected_result": "Patch", "criteria": []string{"test passes"}, "step_ids": []string{project.Phases[0].Steps[0].ID}, "stage_id": project.SDLCStages[0].ID}, "", cookie)
	if code != 201 {
		t.Fatalf("create: %d %s", code, raw)
	}
	var task domain.WorkTask
	json.Unmarshal(raw, &task)
	code, raw, _ = request("POST", "/api/v1/work-tasks/"+task.ID+"/ready", map[string]any{"version": task.Version}, "", cookie)
	if code != 200 {
		t.Fatal(string(raw))
	}
	json.Unmarshal(raw, &task)
	_, token, e := work.CreateToken(ctx, manager, "stdio", []string{project.ID})
	if e != nil {
		t.Fatal(e)
	}
	if code, _, _ := request("POST", "/api/session", map[string]string{"key": key}, token, nil); code != 403 {
		t.Fatal("agent session")
	}
	if code, _, _ := request("GET", "/api/v1/work-projects/"+other.ID, nil, token, nil); code != 403 {
		t.Fatal("scope")
	}
	if code, _, _ := request("POST", "/api/v1/p3/projects", map[string]string{"name": "hijack"}, token, nil); code != 403 {
		t.Fatal("p3 mutation")
	}
	binary := filepath.Join(t.TempDir(), "mcp")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "../../cmd/mcp")
	if b, e := build.CombinedOutput(); e != nil {
		t.Fatalf("build: %v %s", e, b)
	}
	command := exec.CommandContext(ctx, binary)
	command.Env = append(os.Environ(), "AVARI_API_URL="+srv.URL, "AVARI_AGENT_TOKEN="+token)
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1"}, nil)
	session, e := client.Connect(ctx, &mcp.CommandTransport{Command: command}, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer session.Close()
	list, e := session.ListTools(ctx, &mcp.ListToolsParams{})
	if e != nil || len(list.Tools) != 12 {
		t.Fatalf("tools: %v", e)
	}
	for _, tool := range list.Tools {
		if tool.Name == "accept_task" {
			t.Fatal("accept exposed")
		}
	}
	call := func(name string, in any) *mcp.CallToolResult {
		t.Helper()
		res, e := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: in})
		if e != nil {
			t.Fatal(e)
		}
		return res
	}
	decode := func(res *mcp.CallToolResult) {
		t.Helper()
		if res.IsError {
			b, _ := json.Marshal(res.Content)
			t.Fatalf("MCP error: %s", b)
		}
		b, e := json.Marshal(res.StructuredContent)
		if e != nil {
			t.Fatal(e)
		}
		if e = json.Unmarshal(b, &task); e != nil {
			t.Fatalf("decode %s: %v", b, e)
		}
	}
	decode(call("start_task", map[string]any{"task_id": task.ID, "version": task.Version, "idempotency_key": "start"}))
	runID := task.Runs[0].ID
	n := len(task.Events)
	decode(call("start_task", map[string]any{"task_id": task.ID, "version": 1, "idempotency_key": "start"}))
	if len(task.Events) != n {
		t.Fatal("duplicate MCP start")
	}
	result := map[string]any{"task_id": task.ID, "run_id": runID, "version": task.Version, "idempotency_key": "result", "result": map[string]any{"description": "Patch created", "kind": "code", "urls": []string{"https://example.com/commit"}, "artifact_version": "abc"}}
	decode(call("add_result", result))
	n = len(task.Events)
	decode(call("add_result", result))
	if len(task.Events) != n || len(task.Runs[0].Results) != 1 {
		t.Fatal("duplicate MCP result")
	}
	decode(call("submit_task", map[string]any{"task_id": task.ID, "run_id": runID, "version": task.Version, "idempotency_key": "submit", "text": "Ready", "checks": []domain.CriterionCheck{{Criterion: 0, Outcome: "pass", Method: "go test", Source: "agent report", Evidence: "reported log"}}}))
	if task.Status != "in_review" {
		t.Fatal(task.Status)
	}
	if code, _, _ := request("POST", "/api/v1/work-tasks/"+task.ID+"/review", map[string]any{"version": task.Version, "run_id": runID, "decision": "accept"}, token, nil); code != 403 {
		t.Fatal("agent accepted")
	}
	code, raw, _ = request("POST", "/api/v1/work-tasks/"+task.ID+"/review", map[string]any{"version": task.Version, "run_id": runID, "decision": "accept"}, "", cookie)
	if code != 200 {
		t.Fatalf("review: %d %s", code, raw)
	}
	json.Unmarshal(raw, &task)
	if task.Status != "done" {
		t.Fatal(task.Status)
	}
	if !call("report_progress", map[string]any{"task_id": task.ID, "run_id": runID, "version": task.Version, "text": "late"}).IsError {
		t.Fatal("late accepted")
	}
	code, raw, _ = request("GET", "/api/v1/task-results?project_id="+project.ID+"&state=accept", nil, "", cookie)
	if code != 200 || !bytes.Contains(raw, []byte("Patch created")) {
		t.Fatal("accepted result missing")
	}
	// A server restart requires manager re-entry, while the persistent agent token remains valid.
	restarted := handler.NewWorkHandler(work, p3, nil, key)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "http://localhost/api/session", nil)
	req.AddCookie(cookie)
	restarted.Auth(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("old cookie valid") })).ServeHTTP(rr, req)
	if rr.Code != 401 {
		t.Fatal(rr.Code)
	}
	// Cross-origin bootstrap must allow the new custom header.
	req, _ = http.NewRequestWithContext(ctx, "OPTIONS", srv.URL+"/api/session", nil)
	req.Header.Set("Origin", "http://localhost:4810")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "content-type,x-avari-local")
	response, e := http.DefaultClient.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	response.Body.Close()
	if !strings.Contains(strings.ToLower(response.Header.Get("Access-Control-Allow-Headers")), "x-avari-local") {
		t.Fatal("preflight blocked")
	}
}
