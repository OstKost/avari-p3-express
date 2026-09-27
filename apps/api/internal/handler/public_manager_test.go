package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPublicManagerSessionAndAuth(t *testing.T) {
	const origin = "https://p3express.avari.dev"
	key := strings.Repeat("k", 32)
	h := NewWorkHandler(nil, nil, nil, key, origin)
	for _, tt := range []struct {
		name, host, origin, header string
		want                       int
	}{
		{"correct", "p3express.avari.dev", origin, "manager", 200},
		{"foreign", "p3express.avari.dev", "https://evil.dev", "manager", 403},
		{"missing", "p3express.avari.dev", "", "manager", 403},
		{"host spoof", "evil.dev", origin, "manager", 403},
		{"sibling", "p3express.avari.dev", "https://other.avari.dev", "manager", 403},
		{"invalid", "p3express.avari.dev", origin + "/path", "manager", 403},
		{"missing header", "p3express.avari.dev", origin, "", 403},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "http://"+tt.host+"/api/v1/work/session", strings.NewReader(`{"key":"`+key+`"}`))
			r.Header.Set("Origin", tt.origin)
			r.Header.Set("X-Avari-Local", tt.header)
			r.Header.Set("X-Forwarded-Host", "p3express.avari.dev")
			w := httptest.NewRecorder()
			h.Session(w, r)
			if w.Code != tt.want {
				t.Fatalf("status %d: %s", w.Code, w.Body)
			}
			if w.Code != 200 {
				return
			}
			cookies := w.Result().Cookies()
			if len(cookies) != 1 || !cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
				t.Fatalf("insecure cookie: %+v", cookies)
			}
			for _, authCase := range []struct {
				method, origin, header string
				want                   int
			}{{"PATCH", origin, "manager", 204}, {"PATCH", origin, "", 204}, {"PATCH", "", "manager", 403}, {"PATCH", "https://evil.dev", "manager", 403}, {"GET", "", "", 204}, {"GET", "https://evil.dev", "", 403}} {
				req := httptest.NewRequest(authCase.method, "http://p3express.avari.dev/api/v1/p3/projects/demo", nil)
				req.Header.Set("Origin", authCase.origin)
				req.Header.Set("X-Avari-Local", authCase.header)
				req.AddCookie(cookies[0])
				out := httptest.NewRecorder()
				h.Auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if !actor(r).Manager {
						t.Error("not manager")
					}
					w.WriteHeader(204)
				})).ServeHTTP(out, req)
				if out.Code != authCase.want {
					t.Errorf("auth %q header %q: %d", authCase.origin, authCase.header, out.Code)
				}
			}
		})
	}
}

func TestDefaultManagerRemainsLocal(t *testing.T) {
	h := NewWorkHandler(nil, nil, nil, strings.Repeat("k", 32))
	for _, tt := range []struct {
		host, origin string
		want         bool
	}{{"localhost:4820", "", true}, {"localhost:4820", "http://localhost:4820", true}, {"p3express.avari.dev", "https://p3express.avari.dev", false}, {"localhost:4820", "https://p3express.avari.dev", false}} {
		r := httptest.NewRequest("POST", "http://"+tt.host+"/", nil)
		r.Header.Set("Origin", tt.origin)
		if h.trusted(r) != tt.want {
			t.Errorf("host %s origin %s", tt.host, tt.origin)
		}
	}
	invalid := NewWorkHandler(nil, nil, nil, "", "http://p3express.avari.dev")
	r := httptest.NewRequest("POST", "http://p3express.avari.dev/", nil)
	r.Header.Set("Origin", "http://p3express.avari.dev")
	if invalid.trusted(r) {
		t.Fatal("invalid configured origin trusted")
	}
}

func TestPublicManagerStillRequiresCredentials(t *testing.T) {
	const origin = "https://p3express.avari.dev"
	h := NewWorkHandler(nil, nil, nil, strings.Repeat("k", 32), origin)
	r := httptest.NewRequest("POST", "http://p3express.avari.dev/api/v1/work/session", strings.NewReader(`{"key":"wrong"}`))
	r.Header.Set("Origin", origin)
	r.Header.Set("X-Avari-Local", "manager")
	w := httptest.NewRecorder()
	h.Session(w, r)
	if w.Code != 401 || len(w.Result().Cookies()) != 0 {
		t.Fatalf("wrong key: %d", w.Code)
	}
	r = httptest.NewRequest("PATCH", "http://p3express.avari.dev/api/v1/p3/projects/demo", nil)
	r.Header.Set("Origin", origin)
	r.Header.Set("X-Avari-Local", "manager")
	w = httptest.NewRecorder()
	h.Auth(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("unauthenticated request reached handler") })).ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatalf("missing cookie: %d", w.Code)
	}
}

func TestPublicManagerBrowserReads(t *testing.T) {
	h := NewWorkHandler(nil, nil, nil, "", "https://p3express.avari.dev")
	for _, tt := range []struct {
		method, host, origin string
		want                 bool
	}{
		{"GET", "p3express.avari.dev", "", true}, {"HEAD", "p3express.avari.dev", "", true},
		{"GET", "p3express.avari.dev", "https://p3express.avari.dev", true},
		{"GET", "p3express.avari.dev", "https://evil.dev", false},
		{"GET", "other.avari.dev", "", false}, {"POST", "p3express.avari.dev", "", false},
		{"PATCH", "p3express.avari.dev", "", false}, {"DELETE", "p3express.avari.dev", "", false},
	} {
		r := httptest.NewRequest(tt.method, "http://"+tt.host+"/api/session", nil)
		r.Header.Set("Origin", tt.origin)
		if h.trustedManagerRequest(r) != tt.want {
			t.Errorf("%s host=%s origin=%s", tt.method, tt.host, tt.origin)
		}
	}
}
