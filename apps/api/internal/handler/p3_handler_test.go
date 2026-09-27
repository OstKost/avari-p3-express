package handler

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OstKost/avari-p3-express/apps/api/internal/database"
	"github.com/OstKost/avari-p3-express/apps/api/internal/domain"
	"github.com/OstKost/avari-p3-express/apps/api/internal/repository/sqlite"
	"github.com/OstKost/avari-p3-express/apps/api/internal/service"
	"github.com/go-chi/chi/v5"
)

func TestP3HTTPContract(t *testing.T) {
	db, err := database.NewConnection(filepath.Join(t.TempDir(), "http.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	router.Route("/api/v1", func(r chi.Router) { p3Routes(r, NewP3Handler(service.NewP3Service(sqlite.NewP3Repository(db)))) })
	request := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
		return w
	}
	if w := request("GET", "/api/v1/p3/projects", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"data":[]`) {
		t.Fatalf("empty projects: %d %s", w.Code, w.Body.String())
	}
	w := request("POST", "/api/v1/p3/projects", `{"name":"Atlas"}`)
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var p domain.Project
	if err = json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if len(p.Phases) != 7 {
		t.Fatalf("phases = %d", len(p.Phases))
	}
	step := p.Phases[0].Steps[0].ID
	w = request("POST", "/api/v1/p3/steps/"+step+"/links", `{"title":"Bad","url":"file:///etc/passwd"}`)
	if w.Code != 400 {
		t.Fatalf("invalid URL: %d %s", w.Code, w.Body.String())
	}
	w = request("PATCH", "/api/v1/p3/steps/"+step, `{"status":"blocked"}`)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"status":"blocked"`) {
		t.Fatalf("blocked status: %d %s", w.Code, w.Body.String())
	}
	w = request("GET", "/api/v1/p3/projects/missing", "")
	if w.Code != 404 {
		t.Fatalf("missing project: %d %s", w.Code, w.Body.String())
	}
}
