package security

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type testUserRepository struct{ users map[int64]User }

func (s *testUserRepository) User(_ context.Context, id int64) (User, error) {
	u, ok := s.users[id]
	if !ok {
		return User{}, sql.ErrNoRows
	}
	return u, nil
}
func (s *testUserRepository) Authenticate(_ context.Context, name, password string) (User, bool, error) {
	for _, u := range s.users {
		if u.Username == name {
			return u, password == "valid-password-123" && u.Enabled, nil
		}
	}
	return User{}, false, nil
}

type testScopes map[string]ResourceScope

func (s testScopes) Resolve(_ context.Context, kind string, id int64) (ResourceScope, error) {
	v, ok := s[fmt.Sprintf("%s:%d", kind, id)]
	if !ok {
		return v, sql.ErrNoRows
	}
	return v, nil
}

func TestRoleProjectAuthorization(t *testing.T) {
	users := &testUserRepository{users: map[int64]User{
		1: {ID: 1, Username: "admin", Role: RoleAdmin, Enabled: true, Version: 1},
		2: {ID: 2, Username: "tester", Role: RoleTester, Enabled: true, ProjectIDs: []int{1}, Version: 1},
		3: {ID: 3, Username: "viewer", Role: RoleViewer, Enabled: true, ProjectIDs: []int{1}, Version: 1},
		4: {ID: 4, Username: "automatic", Role: RoleTester, Enabled: true, ProjectIDs: []int{1}, AllowAuto: true, Version: 1},
	}}
	a := &Access{Enabled: true, repository: users, resolver: testScopes{
		"api:10": {ProjectID: 1}, "api:20": {ProjectID: 2}, "case:10": {ProjectID: 1}, "case:20": {ProjectID: 2},
		"task:10": {ProjectID: 1, Auto: true}, "project_run:10": {ProjectID: 1, Auto: true}, "project_run:20": {ProjectID: 2},
		"document:20": {ProjectID: 2}, "model:10": {Enabled: true}, "model:20": {Enabled: false},
	}}
	for _, tc := range []struct {
		name, user, method, path, body string
		want                           int
	}{
		{"tester own read", "tester", "GET", "/api/apis?projectId=1", "", 200},
		{"tester foreign read", "tester", "GET", "/api/apis?projectId=2", "", 403},
		{"viewer own read", "viewer", "GET", "/api/runs?projectId=1", "", 200},
		{"viewer cannot run", "viewer", "POST", "/api/runs", `{"projectId":1,"apiId":10}`, 403},
		{"viewer cannot draft paid", "viewer", "POST", "/api/agent/draft", `{"projectId":1}`, 403},
		{"viewer cannot edit", "viewer", "PUT", "/api/apis", `{"projectId":1,"id":10}`, 403},
		{"tester own edit", "tester", "PUT", "/api/apis", `{"projectId":1,"id":10}`, 200},
		{"wrong resource parent", "tester", "POST", "/api/runs", `{"projectId":1,"apiId":20}`, 403},
		{"case overrides denied", "tester", "POST", "/api/runs", `{"projectId":1,"caseId":20}`, 403},
		{"infer case project", "tester", "POST", "/api/runs", `{"caseId":10}`, 200},
		{"regression foreign case", "tester", "GET", "/api/regression?caseId=20", "", 403},
		{"unscoped regression denied", "tester", "GET", "/api/regression", "", 400},
		{"chain foreign step", "tester", "POST", "/api/runs/chain", `{"projectId":1,"steps":[{"apiId":20}]}`, 403},
		{"batch foreign item", "tester", "POST", "/api/runs/batch", `{"items":[{"apiId":10},{"apiId":20}]}`, 403},
		{"batch retains handler path", "tester", "POST", "/api/runs/batch", `{"items":[{"apiId":10}]}`, 200},
		{"automatic needs grant", "tester", "POST", "/api/project-check", `{"projectId":1,"permission":"auto"}`, 403},
		{"automatic granted", "automatic", "POST", "/api/project-check", `{"projectId":1,"permission":"auto"}`, 200},
		{"saved auto requires review", "tester", "POST", "/api/project-tests/runs", `{"projectId":1,"taskId":10}`, 403},
		{"saved auto explicit review", "tester", "POST", "/api/project-tests/runs", `{"projectId":1,"taskId":10,"confirmed":true}`, 200},
		{"foreign resume denied", "automatic", "POST", "/api/project-tests/runs", `{"projectId":1,"taskId":10,"sourceRunId":20}`, 403},
		{"tester cannot configure pool", "tester", "POST", "/api/models/import", `{"projectId":1}`, 403},
		{"viewer model metadata", "viewer", "GET", "/api/models?projectId=1", "", 200},
		{"tester disabled model denied", "tester", "POST", "/api/evaluation-suites/runs", `{"projectId":1,"profileIds":[20]}`, 403},
		{"foreign documents denied", "tester", "POST", "/api/agent/draft", `{"projectId":1,"documentIds":[20]}`, 403},
		{"unknown future capability denied", "tester", "POST", "/api/future-task", `{"projectId":1}`, 403},
		{"case alias denied", "tester", "POST", "/api/runs", `{"projectId":1,"CASEID":20}`, 403},
		{"ambiguous alias rejected", "tester", "POST", "/api/runs", `{"projectId":1,"ProjectID":2,"apiId":10}`, 400},
		{"permission alias denied", "tester", "POST", "/api/project-check", `{"projectId":1,"PERMISSION":"auto"}`, 403},
		{"admin all projects", "admin", "GET", "/api/apis?projectId=2", "", 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			h := a.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				if tc.name == "batch retains handler path" && r.URL.Path != "/api/runs/batch" {
					t.Fatal("batch path mutated")
				}
				if tc.name == "infer case project" {
					var v map[string]any
					_ = json.NewDecoder(r.Body).Decode(&v)
					if v["projectId"] != float64(1) {
						t.Fatal("scope not supplied to executor")
					}
				}
				w.WriteHeader(200)
			}))
			r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			r.Header.Set("Content-Type", "application/json")
			r.SetBasicAuth(tc.user, "valid-password-123")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("status %d want %d: %s", w.Code, tc.want, w.Body.String())
			}
			if tc.want != 200 && called {
				t.Fatal("denied action reached executor")
			}
		})
	}
}

func TestUserSessionsRevokedOnPermissionChange(t *testing.T) {
	repo := &testUserRepository{users: map[int64]User{1: {ID: 1, Username: "owner", Role: RoleTester, Enabled: true, ProjectIDs: []int{1}, Version: 1}}}
	a := &Access{Enabled: true, repository: repo}
	h := a.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	w := authCall(h, "POST", "/api/auth/login", `{"username":"owner","password":"valid-password-123"}`, nil)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	cookie := w.Result().Cookies()[0]
	if authCall(h, "GET", "/api/projects", "", cookie).Code != 200 {
		t.Fatal("user session failed")
	}
	u := repo.users[1]
	u.Version++
	repo.users[1] = u
	if authCall(h, "GET", "/api/projects", "", cookie).Code != 401 {
		t.Fatal("old session survived permission/password version change")
	}
	w = authCall(h, "POST", "/api/auth/login", `{"username":"owner","password":"valid-password-123"}`, nil)
	cookie = w.Result().Cookies()[0]
	u.Enabled = false
	repo.users[1] = u
	if authCall(h, "GET", "/api/projects", "", cookie).Code != 401 {
		t.Fatal("disabled user's session accepted")
	}
	if authCall(h, "POST", "/api/auth/login", `{"username":"owner","password":"valid-password-123"}`, nil).Code != 401 {
		t.Fatal("disabled login accepted")
	}
}

func TestProjectFilteringAndViewerRedaction(t *testing.T) {
	u := User{ID: 1, Username: "viewer", Role: RoleViewer, Enabled: true, ProjectIDs: []int{1}}
	a := &Access{Enabled: true, repository: &testUserRepository{users: map[int64]User{1: u}}}
	h := a.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/projects" {
			authJSON(w, 200, []map[string]any{{"id": 1, "name": "allowed"}, {"id": 2, "name": "forbidden"}})
			return
		}
		authJSON(w, 200, map[string]any{"responseBody": "arbitrary secret text", "requestBody": "password=secret", "variables": map[string]string{"token": "secret"}, "inputTokens": 13, "outputTokens": 20, "apiKey": "key", "checks": []map[string]any{{"path": "data.token", "expected": "secret", "actual": "secret", "passed": true}}})
	}))
	for _, path := range []string{"/api/projects", "/api/runs?projectId=1"} {
		r := httptest.NewRequest("GET", path, nil)
		r.SetBasicAuth("viewer", "valid-password-123")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		if strings.Contains(w.Body.String(), "forbidden") || strings.Contains(w.Body.String(), "secret") {
			t.Fatal("unauthorized content exposed")
		}
		if strings.Contains(path, "runs") && !strings.Contains(w.Body.String(), `"inputTokens":13`) {
			t.Fatal("usage numbers incorrectly redacted")
		}
	}
}

func TestPasswordStorageAndUserValidation(t *testing.T) {
	hash, err := HashPassword("password-testing-123")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(hash, "password-testing") || !CheckPassword(hash, "password-testing-123") || CheckPassword(hash, "wrong") {
		t.Fatal("invalid password hash")
	}
	for _, bad := range []string{"", "sha256$value", "pbkdf2-sha256$1$a$b"} {
		if CheckPassword(bad, "password-testing-123") {
			t.Fatal("malformed hash accepted")
		}
	}
	if ValidateUser(UserInput{Username: "tester", Role: RoleTester, Password: "short"}, true) == nil {
		t.Fatal("short initial password accepted")
	}
	if ValidateUser(UserInput{Username: "tester", Role: "root"}, false) == nil {
		t.Fatal("unknown role accepted")
	}
}

func TestTesterListFilteringAndCredentialScope(t *testing.T) {
	u := User{ID: 2, Username: "tester", Role: RoleTester, Enabled: true, ProjectIDs: []int{1}}
	a := &Access{Enabled: true, repository: &testUserRepository{users: map[int64]User{2: u}}}
	h := a.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/projects" {
			authJSON(w, 200, []map[string]any{{"id": 1, "name": "assigned"}, {"id": 2, "name": "foreign"}})
		} else {
			authJSON(w, 200, map[string]any{"apiKey": "private-key", "requestBody": "project-credential"})
		}
	}))
	for _, tc := range []struct{ path, forbidden, required string }{
		{"/api/projects", "foreign", "assigned"},
		{"/api/models", "private-key", "["},
		{"/api/apis?projectId=1", "unrelated-value", "project-credential"},
	} {
		r := httptest.NewRequest("GET", tc.path, nil)
		r.SetBasicAuth("tester", "valid-password-123")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 200 || strings.Contains(w.Body.String(), tc.forbidden) || !strings.Contains(w.Body.String(), tc.required) {
			t.Fatalf("%s: %d %s", tc.path, w.Code, w.Body.String())
		}
	}
}
