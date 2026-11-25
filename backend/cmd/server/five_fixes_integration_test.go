package main

import (
	"ai-api-test-platform-v1/backend/internal/evaluation"
	"ai-api-test-platform-v1/backend/internal/projecttest"
	"ai-api-test-platform-v1/backend/internal/security"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestFiveFixesIntegration(t *testing.T) {
	dsn := os.Getenv("PULSE_FIVE_FIXES_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires isolated five-fixes database")
	}
	directory := os.Getenv("PULSE_FIVE_FIXES_DATA_DIR")
	if directory == "" {
		directory = t.TempDir()
	}
	t.Setenv("PROJECT_TEST_DATA_DIR", directory)
	st, err := newStore(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer st.db.Close()
	ctx := context.Background()
	s := &server{store: st}
	var project, apiID int
	st.db.QueryRow("SELECT id FROM projects ORDER BY id LIMIT 1").Scan(&project)
	st.db.QueryRow("SELECT id FROM apis WHERE project_id=$1 ORDER BY id LIMIT 1", project).Scan(&apiID)
	endpoint, ok := st.getAPI(ctx, apiID)
	if !ok {
		t.Fatal("missing fixture endpoint")
	}
	inputCase := TestCase{ProjectID: project, APIID: apiID, Name: "知识库保存验证", Request: RequestSpec{Method: endpoint.Method, URL: endpoint.URL, Body: `{"message":"case-check"}`}, Assertions: []Assertion{{Type: "status", Expected: "200"}}}
	casePayload, _ := json.Marshal(inputCase)
	saved := httptest.NewRecorder()
	s.cases(saved, httptest.NewRequest("POST", "/api/cases", strings.NewReader(string(casePayload))))
	if saved.Code != 201 {
		t.Fatalf("knowledge case could not save: %s", saved.Body.String())
	}
	var savedCase TestCase
	json.Unmarshal(saved.Body.Bytes(), &savedCase)
	stored, exists := st.getCase(ctx, savedCase.ID)
	if !exists || stored.Request.Body != inputCase.Request.Body || stored.Assertions[0].Expected != "200" {
		t.Fatal("saved case lost request or assertions")
	}
	inputCase.Request.URL = "/not-in-catalogue"
	casePayload, _ = json.Marshal(inputCase)
	denied := httptest.NewRecorder()
	s.cases(denied, httptest.NewRequest("POST", "/api/cases", strings.NewReader(string(casePayload))))
	if denied.Code != 400 {
		t.Fatal("invented case request accepted")
	}
	called := []string{}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/case-echo" {
			var incoming any
			json.NewDecoder(r.Body).Decode(&incoming)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"received": incoming})
			return
		}
		var payload struct {
			Model    string
			Messages []struct{ Content string }
		}
		json.NewDecoder(r.Body).Decode(&payload)
		called = append(called, payload.Model)
		answer := `{"intent":"test_api","summary":"fixture","method":"` + endpoint.Method + `","url":"` + endpoint.URL + `","headers":{},"query":{},"body":"{}","assertions":[{"type":"status","expected":"200"}]}`
		if strings.Contains(payload.Messages[0].Content, "业务验收用例设计助手") {
			if strings.Contains(payload.Messages[1].Content, "尚无规则") {
				answer = `{"documentAssessment":{"status":"usable","reason":"有规则但缺少预期"},"cases":[],"missingInformation":["请补充预期业务码"]}`
			} else {
				if !strings.Contains(payload.Messages[1].Content, "完整规则尾部") {
					t.Error("retrieval lost rule tail")
				}
				answer = `{"documentAssessment":{"status":"usable","reason":"提供业务条件"},"cases":[{"name":"业务草稿","scenario":"正常","rule":"完整规则尾部","expectedResult":"等待配置","request":{}}],"missingInformation":["请绑定真实业务接口"]}`
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": answer}}}})
	}))
	defer provider.Close()
	policy, err := security.NewPolicy([]string{provider.URL}, false)
	if err != nil {
		t.Fatal(err)
	}
	previousPolicy := security.SetPolicy(policy)
	defer security.SetPolicy(previousPolicy)
	s.client = security.HTTPClient(30 * time.Second)
	caseRunPayload, _ := json.Marshal(runInput{CaseID: savedCase.ID, Request: RequestSpec{Method: "POST", URL: provider.URL + "/case-echo", Body: `{"message":"runtime-value"}`}})
	caseRun := httptest.NewRecorder()
	s.runs(caseRun, httptest.NewRequest("POST", "/api/runs", strings.NewReader(string(caseRunPayload))))
	var executedCase RunRecord
	json.Unmarshal(caseRun.Body.Bytes(), &executedCase)
	if caseRun.Code != 201 || executedCase.CaseID != savedCase.ID || !executedCase.Success || !strings.Contains(executedCase.ResponseBody, "runtime-value") {
		t.Fatalf("saved case lost execution identity or current input: %s", caseRun.Body.String())
	}
	ids := []int{}
	for _, name := range []string{"alpha", "beta", "unverified"} {
		p, err := st.addModelProfile(ctx, ModelProfile{Name: name, ProjectID: project, BaseURL: provider.URL, Model: name, APIKey: "fixture-not-real", AuthHeader: "Authorization", Enabled: true})
		if err != nil {
			t.Fatal(err)
		}
		if name != "unverified" {
			if _, err = st.db.Exec("UPDATE model_profiles SET verified=true WHERE id=$1", p.ID); err != nil {
				t.Fatal(err)
			}
		}
		ids = append(ids, p.ID)
	}
	for index, id := range ids {
		w := httptest.NewRecorder()
		raw, _ := json.Marshal(agentInput{ProjectID: project, APIID: apiID, Scope: "endpoint", ProfileID: id, Prompt: "测试所选接口"})
		r := httptest.NewRequest("POST", "/api/agent/draft", strings.NewReader(string(raw)))
		s.agentDraft(w, r)
		want := 200
		if index == 2 {
			want = 409
		}
		if w.Code != want {
			t.Fatalf("model selection %d %s", w.Code, w.Body.String())
		}
	}
	if len(called) != 2 || called[0] != "alpha" || called[1] != "beta" {
		t.Fatalf("wrong model calls: %v", called)
	}
	doc, err := st.addDocument(ctx, ProjectDocument{ProjectID: project, Name: "业务规则.md", Content: "测试标题\n" + strings.Repeat("业务前置条件。\n", 30) + "完整规则尾部"}, []string{})
	if err != nil {
		t.Fatal(err)
	}
	view := httptest.NewRecorder()
	s.documentsHandler(view, httptest.NewRequest("GET", fmt.Sprintf("/api/documents?projectId=%d&id=%d", project, doc.ID), nil))
	if view.Code != 200 || !strings.Contains(view.Body.String(), "完整规则尾部") {
		t.Fatal("document preview lost extracted text")
	}
	for _, goal := range []string{"生成业务草稿", "尚无规则"} {
		raw, _ := json.Marshal(businessCasesInput{ProjectID: project, ProfileID: ids[0], Mode: "design", DocumentIDs: []int{doc.ID}, Prompt: goal})
		w := httptest.NewRecorder()
		s.businessCases(w, httptest.NewRequest("POST", "/api/agent/business-cases", strings.NewReader(string(raw))))
		if w.Code != 200 {
			t.Fatalf("missing info remains HTTP error: %s", w.Body.String())
		}
		var result map[string]any
		json.Unmarshal(w.Body.Bytes(), &result)
		if len(result["missingInformation"].([]any)) == 0 {
			t.Fatal("missing information discarded")
		}
		if goal == "生成业务草稿" && len(result["cases"].([]any)) != 1 {
			t.Fatal("usable rules failed to generate a draft")
		}
	}
	es, err := evaluation.NewStore(st.db)
	if err != nil {
		t.Fatal(err)
	}
	run, err := es.NewRun(ctx, evaluation.Run{ProjectID: project, Status: "completed", Suite: evaluation.Suite{ProjectID: project, Name: "测试报告", ProfileIDs: []int{ids[0]}, Repeats: 1, Cases: []evaluation.Sample{{ID: "s1", Name: "样本"}}}})
	if err != nil {
		t.Fatal(err)
	}
	h := evaluation.NewHandler(es, evaluationGateway{s: s})
	w := httptest.NewRecorder()
	h.Export(w, httptest.NewRequest("GET", fmtURL("/api/evaluation-suites/export", project, run.ID), nil))
	if w.Code != 200 || !strings.Contains(w.Header().Get("Content-Type"), "wordprocessingml") {
		t.Fatalf("Word route failed %s", w.Body.String())
	}
	ts, err := projecttest.NewTaskStore(st.db, directory)
	if err != nil {
		t.Fatal(err)
	}
	task, err := ts.SaveTask(ctx, projecttest.Task{ProjectID: project, Name: "云盘报告", BaseURL: "http://localhost:8082"})
	if err != nil {
		t.Fatal(err)
	}
	pr, err := ts.NewRun(ctx, projecttest.TaskRun{ProjectID: project, TaskID: task.ID, Task: task, Status: "completed"})
	if err != nil {
		t.Fatal(err)
	}
	if err = ts.UpdateRun(ctx, pr, projecttest.RunPrivate{}); err != nil {
		t.Fatal(err)
	}
	th := projecttest.NewTaskHandler(ts, nil)
	w = httptest.NewRecorder()
	th.Export(w, httptest.NewRequest("GET", fmtURL("/api/project-tests/export", project, pr.ID), nil))
	if w.Code != 200 || !strings.Contains(w.Header().Get("Content-Type"), "wordprocessingml") {
		t.Fatalf("project Word route failed %s", w.Body.String())
	}
}

func fmtURL(path string, project int, id int64) string {
	return fmt.Sprintf("%s?projectId=%d&id=%d&format=docx", path, project, id)
}
