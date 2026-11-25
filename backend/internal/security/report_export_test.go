package security

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestViewerWordExportUsesSanitizedReport(t *testing.T) {
	user := User{ID: 3, Username: "viewer", Role: RoleViewer, Enabled: true, Version: 1, ProjectIDs: []int{1}}
	a := &Access{Enabled: true, repository: &testUserRepository{users: map[int64]User{3: user}}, resolver: testScopes{"project_run:10": {ProjectID: 1}}}
	h := a.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("format") != "json" {
			t.Fatal("viewer export skipped JSON sanitation")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"id": 10, "status": "completed", "task": map[string]any{"name": "viewer report", "password": "private-pass"}, "summary": map[string]any{"total": 1, "passed": 1}, "results": []any{map[string]any{"name": "login", "status": "passed", "responseBody": "private-token-123", "requestHeaders": map[string]any{"Authorization": "secret-header"}}}})
	}))
	r := httptest.NewRequest("GET", "/api/project-tests/export?projectId=1&id=10&format=docx", nil)
	r.SetBasicAuth("viewer", "valid-password-123")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Header().Get("Content-Type"), "wordprocessingml") {
		t.Fatalf("not Word: %d %s", w.Code, w.Body.String())
	}
	z, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range z.File {
		reader, _ := f.Open()
		raw, _ := io.ReadAll(reader)
		reader.Close()
		for _, secret := range []string{"private-pass", "private-token-123", "secret-header"} {
			if strings.Contains(string(raw), secret) {
				t.Fatal("Word leaked private response")
			}
		}
	}
}
