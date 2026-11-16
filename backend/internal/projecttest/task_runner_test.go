package projecttest

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPlannerMissingInformationIsStructured(t *testing.T) {
	h := NewTaskHandler(nil, func(context.Context, Task, string) ([]TaskStep, error) {
		return nil, &MissingTaskInformation{Fields: []TaskInput{{Field: "captcha", Question: "请输入测试验证码", Secret: true}}}
	})
	raw := `{"projectId":1,"name":"test","baseUrl":"http://localhost:8082","template":"ai","accountMode":"auto","fileMode":"auto","permission":"review"}`
	req := httptest.NewRequest("POST", "/api/project-tests/plan", strings.NewReader(raw))
	w := httptest.NewRecorder()
	h.Plan(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "captcha") || !strings.Contains(w.Body.String(), "missingInformation") {
		t.Fatal(w.Code, w.Body.String())
	}
}
func TestExtraVariablesCannotOverrideRunResources(t *testing.T) {
	task := testTask("http://localhost:8082")
	task.InitialVariables = map[string]string{"fileId": "external", "captcha": "1234"}
	p, _, err := NewVariables(task)
	if err != nil || p.Variables["captcha"] != "1234" || len(p.Owned) > 0 {
		t.Fatal(p, err)
	}
	task.InitialVariables = map[string]string{"fileHash": "override"}
	if _, _, err := NewVariables(task); err == nil {
		t.Fatal("protected context overwritten")
	}
}

func testTask(base string) Task { return Task{BaseURL: base, AccountMode: "auto", FileMode: "auto"} }
func TestNewTaskVariablesUniqueAndExactFile(t *testing.T) {
	a, file, err := NewVariables(testTask("http://localhost:8082"))
	if err != nil {
		t.Fatal(err)
	}
	b, _, _ := NewVariables(testTask("http://localhost:8082"))
	if len(file) != 2048 || a.Variables["username"] == b.Variables["username"] || a.Variables["password"] == "" {
		t.Fatal("invalid generated account or sample")
	}
}
func TestExpectedBusinessErrorPasses(t *testing.T) {
	result := checkAssertion(TaskAssertion{Type: "json_equals", Path: "code", Expected: "409"}, 200, nil, map[string]any{"code": float64(409)}, nil)
	if !result.Passed {
		t.Fatal(result)
	}
}
func TestBinaryMustMatchActualContent(t *testing.T) {
	hash := sha256.Sum256([]byte("sample"))
	a := TaskAssertion{Type: "binary_sha256", Expected: hex.EncodeToString(hash[:])}
	if !checkAssertion(a, 200, []byte("sample"), nil, nil).Passed || checkAssertion(a, 200, nil, nil, nil).Passed {
		t.Fatal("HTTP200 alone is insufficient")
	}
}
func TestArrayCheckSameResource(t *testing.T) {
	doc := map[string]any{"list": []any{map[string]any{"id": "ours", "size": 1}, map[string]any{"id": "other", "size": 2048}}}
	a := TaskAssertion{Type: "array_contains", Path: "list", Field: "size", Expected: "2048", MatchField: "id", MatchValue: "ours"}
	if checkAssertion(a, 200, nil, doc, nil).Passed {
		t.Fatal("matched unrelated resource")
	}
}
func TestExtractMatchesNotFirstRow(t *testing.T) {
	p := RunPrivate{Variables: map[string]string{"name": "ours"}, Owned: map[string]string{}}
	doc := map[string]any{"list": []any{map[string]any{"name": "other", "id": "1"}, map[string]any{"name": "ours", "id": "2"}}}
	v, err := extractValues([]TaskExtract{{Name: "id", Path: "list", MatchField: "name", MatchValue: "{{name}}", ValueField: "id"}}, doc, &p)
	if err != nil || v["id"] != "2" {
		t.Fatal(v, err)
	}
}
func TestPartialExtractionDoesNotMutateContext(t *testing.T) {
	p := RunPrivate{Variables: map[string]string{}, Owned: map[string]string{}}
	_, err := extractValues([]TaskExtract{{Name: "one", Path: "id"}, {Name: "missing", Path: "none"}}, map[string]any{"id": "1"}, &p)
	if err == nil || len(p.Variables) > 0 {
		t.Fatal("partial state must not leak")
	}
}
func TestExternalScopeAndUnboundGuardRejected(t *testing.T) {
	a := TaskStep{ID: "x", Name: "delete", Method: "DELETE", URL: "http://example.com/x", Guard: "id", Assertions: []TaskAssertion{{Type: "status", Expected: "200"}}}
	if ValidatePlan([]TaskStep{a}, "http://localhost:8082") == nil {
		t.Fatal("external target")
	}
	a.URL = "/file/999"
	if ValidatePlan([]TaskStep{a}, "http://localhost:8082") == nil {
		t.Fatal("guard did not bind resource")
	}
	a.URL = "/file/{{id}}"
	if err := ValidatePlan([]TaskStep{a}, "http://localhost:8082"); err != nil {
		t.Fatal(err)
	}
}
func TestUnownedGuardSendsNoRequest(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
	defer s.Close()
	p := RunPrivate{Variables: map[string]string{"id": "1"}, Owned: map[string]string{}}
	r := ExecuteTaskStep(context.Background(), s.Client(), testTask(s.URL), TaskStep{Method: "DELETE", URL: "/{{id}}", Guard: "id"}, &p, nil)
	if r.Status != "failed" || calls != 0 {
		t.Fatal(r, calls)
	}
}
func TestReadCannotClaimArbitraryOwnedResource(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"id":"999"}`)) }))
	defer s.Close()
	p, _, _ := NewVariables(testTask(s.URL))
	step := TaskStep{Method: "GET", URL: "/file", Assertions: []TaskAssertion{{Type: "status", Expected: "200"}}, Extract: []TaskExtract{{Name: "id", Path: "id", Owned: true}}}
	r := ExecuteTaskStep(context.Background(), s.Client(), testTask(s.URL), step, &p, nil)
	if r.Status != "failed" || len(p.Owned) != 0 {
		t.Fatal(r)
	}
}
func TestMultipartAndReportRedaction(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(4096); err != nil {
			t.Error(err)
		}
		f, _, err := r.FormFile("payload")
		if err != nil {
			t.Error(err)
			return
		}
		defer f.Close()
		buffer := new(bytes.Buffer)
		buffer.ReadFrom(f)
		if buffer.String() != "sample" {
			t.Error("file body")
		}
		w.Write([]byte(`{"code":200,"token":"sensitive-token"}`))
	}))
	defer s.Close()
	p, _, _ := NewVariables(testTask(s.URL))
	p.Variables["token"] = "sensitive-token"
	step := TaskStep{Method: "POST", URL: "/upload", Upload: true, FileField: "payload", Headers: map[string]string{"Authorization": "Bearer {{token}}"}, Assertions: []TaskAssertion{{Type: "status", Expected: "200"}}}
	r := ExecuteTaskStep(context.Background(), s.Client(), testTask(s.URL), step, &p, []byte("sample"))
	raw, _ := json.Marshal(r)
	if r.Status != "passed" || strings.Contains(string(raw), "sensitive-token") {
		t.Fatal(string(raw))
	}
}
func TestDependenciesBlockAndReplaySkipsSuccess(t *testing.T) {
	calls := map[string]int{}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls[r.URL.Path]++; w.Write([]byte(`{"code":409}`)) }))
	defer s.Close()
	steps := []TaskStep{
		{ID: "register", Method: "POST", URL: "/register", Assertions: []TaskAssertion{{Type: "status", Expected: "200"}}},
		{ID: "bad", Method: "GET", URL: "/bad", DependsOn: []string{"register"}, Assertions: []TaskAssertion{{Type: "json_equals", Path: "code", Expected: "200"}}},
		{ID: "child", Method: "GET", URL: "/child", DependsOn: []string{"bad"}, Assertions: []TaskAssertion{{Type: "status", Expected: "200"}}},
	}
	task := testTask(s.URL)
	task.Steps = steps
	p, _, _ := NewVariables(task)
	results := []StepResult{}
	err := RunTaskPlan(context.Background(), s.Client(), task, &p, nil, []StepResult{{ID: "register", Status: "passed"}}, func(r StepResult) error {
		if r.Status != "running" {
			results = append(results, r)
		}
		return nil
	})
	if err != nil || calls["/register"] != 0 || calls["/bad"] != 1 || calls["/child"] != 0 || !results[0].Reused || results[2].Status != "blocked" {
		t.Fatal(err, calls, results)
	}
}
func TestAuthRejectionDoesNotAcceptServerFailure(t *testing.T) {
	a := TaskAssertion{Type: "auth_rejected"}
	if checkAssertion(a, 500, nil, map[string]any{"code": 500}, nil).Passed || !checkAssertion(a, 401, nil, nil, nil).Passed {
		t.Fatal("invalid auth verdict")
	}
}
