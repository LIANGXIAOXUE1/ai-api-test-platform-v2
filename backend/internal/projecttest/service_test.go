package projecttest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type memoryRepository struct {
	endpoints   []Endpoint
	reports     []Report
	endpointErr error
	saveErr     error
	listErr     error
	projectID   int
}

func (r *memoryRepository) Endpoints(_ context.Context, projectID int) ([]Endpoint, error) {
	r.projectID = projectID
	return r.endpoints, r.endpointErr
}
func (r *memoryRepository) SaveReport(_ context.Context, report Report) (Report, error) {
	if r.saveErr != nil {
		return report, r.saveErr
	}
	report.ID = 73
	r.reports = append(r.reports, report)
	return report, nil
}
func (r *memoryRepository) Reports(_ context.Context, projectID int) ([]Report, error) {
	r.projectID = projectID
	if r.reports == nil {
		return []Report{}, r.listErr
	}
	return r.reports, r.listErr
}

func TestReviewDoesNotExecuteRequestsAndKeepsReportContract(t *testing.T) {
	repo := &memoryRepository{endpoints: []Endpoint{
		{ID: 1, Name: "读取", Method: "GET", URL: "/read"},
		{ID: 2, Name: "写入", Method: "POST", URL: "/write"},
	}}
	service := NewService(repo, ProbeExecutorFunc(func(context.Context, ProbeRequest) (ProbeResult, error) {
		t.Fatal("review must not execute a request")
		return ProbeResult{}, nil
	}))
	report, err := service.Run(context.Background(), Input{ProjectID: 7, BaseURL: " http://localhost:8082/ ", Permission: "review"})
	if err != nil {
		t.Fatal(err)
	}
	if repo.projectID != 7 || report.ID != 73 || report.Executed || report.Summary.Pending != 2 {
		t.Fatalf("unexpected review report: %+v", report)
	}
	if report.BaseURL != "http://localhost:8082" || len(repo.reports) != 1 {
		t.Fatalf("normalization/persistence failed: %+v", report)
	}
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"id", "projectId", "baseUrl", "task", "permission", "executed", "createdAt", "summary", "items", "markdown"} {
		if _, ok := body[key]; !ok {
			t.Errorf("missing existing response field %q", key)
		}
	}
}

func TestAutoOnlyProbesLocalReadRequestsAndPreservesReachabilitySemantics(t *testing.T) {
	repo := &memoryRepository{endpoints: []Endpoint{
		{ID: 1, Name: "需要登录", Method: "GET", URL: "http://127.0.0.1:8082/private"},
		{ID: 2, Name: "创建文件", Method: "POST", URL: "/create"},
		{ID: 3, Name: "外部服务", Method: "GET", URL: "https://example.invalid/external"},
		{ID: 4, Name: "不存在", Method: "GET", URL: "/missing"},
		{ID: 5, Name: "未启动", Method: "GET", URL: "/offline"},
	}}
	var probes []ProbeRequest
	service := NewService(repo, ProbeExecutorFunc(func(_ context.Context, input ProbeRequest) (ProbeResult, error) {
		probes = append(probes, input)
		if input.ProjectID != 7 || input.Method != http.MethodGet {
			t.Fatalf("bad probe: %+v", input)
		}
		switch input.APIID {
		case 1:
			if input.URL != "http://localhost:8082/private" {
				t.Fatalf("loopback alias changed: %s", input.URL)
			}
			return ProbeResult{StatusCode: 200, ResponseBody: "{\"code\":401}", DurationMs: 8}, nil
		case 4:
			return ProbeResult{StatusCode: 404, ErrorMessage: "missing"}, nil
		case 5:
			return ProbeResult{StatusCode: 0, ErrorMessage: "connection refused"}, nil
		default:
			t.Fatalf("unexpected executed endpoint: %+v", input)
			return ProbeResult{}, nil
		}
	}))
	report, err := service.Run(context.Background(), Input{ProjectID: 7, BaseURL: "http://localhost:8082", Permission: "auto", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(probes) != 3 || !report.Executed || report.Summary != (Summary{Total: 5, Reachable: 2, Unavailable: 1, Pending: 2}) {
		t.Fatalf("unexpected result: %+v; probes=%+v", report, probes)
	}
	if report.Items[0].BusinessCode != "401" || !report.Items[3].Reachable || report.Items[4].Reachable {
		t.Fatalf("legacy connectivity semantics changed: %+v", report.Items)
	}
	if !strings.Contains(report.Markdown, "需要登录") || !strings.Contains(report.Markdown, "不携带 Token") {
		t.Fatalf("report content missing: %s", report.Markdown)
	}
}

func TestStorageFailuresAreNotReportedAsSuccessfulResults(t *testing.T) {
	repo := &memoryRepository{endpoints: []Endpoint{{ID: 1, Name: "read", Method: "GET", URL: "/read"}}}
	service := NewService(repo, ProbeExecutorFunc(func(context.Context, ProbeRequest) (ProbeResult, error) {
		return ProbeResult{StatusCode: 200}, errors.New("cannot save run")
	}))
	report, err := service.Run(context.Background(), Input{ProjectID: 7, BaseURL: "http://localhost:8082", Permission: "auto"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Unavailable != 1 || report.Items[0].Error != "cannot save run" || report.Items[0].Reachable {
		t.Fatalf("lost persistence failure: %+v", report)
	}
	repo.saveErr = errors.New("cannot save report")
	if _, err := service.Run(context.Background(), Input{ProjectID: 7, BaseURL: "http://localhost:8082"}); !errors.Is(err, repo.saveErr) {
		t.Fatalf("report storage failure missing: %v", err)
	}
}

func TestHTTPValidationAndHistoryRemainCompatible(t *testing.T) {
	repo := &memoryRepository{reports: []Report{{
		ID: 12, ProjectID: 7, BaseURL: "http://localhost:8082", CreatedAt: time.Unix(1, 0).UTC(),
		Summary: Summary{Total: 1, Pending: 1}, Items: []Item{}, Markdown: "saved evidence",
	}}}
	handler := NewHandler(NewService(repo, ProbeExecutorFunc(func(context.Context, ProbeRequest) (ProbeResult, error) {
		t.Fatal("validation/history must not execute requests")
		return ProbeResult{}, nil
	})))
	cases := []struct {
		method, path, body string
		status             int
	}{
		{"GET", "/api/project-check", "", 405},
		{"POST", "/api/project-check", "{", 400},
		{"POST", "/api/project-check", "{\"projectId\":7,\"baseUrl\":\"file:///tmp\"}", 400},
		{"POST", "/api/project-check-reports?projectId=7", "", 405},
		{"GET", "/api/project-check-reports?projectId=0", "", 400},
		{"GET", "/api/project-check-reports?projectId=7", "", 200},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path+" "+tc.body, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if strings.HasPrefix(tc.path, "/api/project-check-reports") {
				handler.Reports(rec, req)
			} else {
				handler.Run(rec, req)
			}
			if rec.Code != tc.status {
				t.Fatalf("status=%d, body=%s", rec.Code, rec.Body.String())
			}
			if tc.status == 200 {
				var reports []Report
				if err := json.Unmarshal(rec.Body.Bytes(), &reports); err != nil {
					t.Fatal(err)
				}
				if len(reports) != 1 || reports[0].ID != 12 || reports[0].Markdown != "saved evidence" || repo.projectID != 7 {
					t.Fatalf("history changed: %+v", reports)
				}
			}
		})
	}
}
