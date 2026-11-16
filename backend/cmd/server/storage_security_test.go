package main

import (
	"ai-api-test-platform-v1/backend/internal/projecttest"
	"ai-api-test-platform-v1/backend/internal/secrets"
	"ai-api-test-platform-v1/backend/internal/security"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Run only against an explicitly created, empty validation database.
func TestStorageSecurityIntegration(t *testing.T) {
	dsn := os.Getenv("PULSE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set PULSE_TEST_DATABASE_URL to an isolated empty database")
	}
	dir := t.TempDir()
	t.Setenv("PROJECT_TEST_DATA_DIR", dir)
	st, err := newStore(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer st.db.Close()
	ctx := context.Background()
	var project int
	if err := st.db.QueryRow(`SELECT id FROM projects ORDER BY id LIMIT 1`).Scan(&project); err != nil {
		t.Fatal(err)
	}
	const key = "test-model-secret-never-real"
	p, err := st.addModelProfile(ctx, ModelProfile{ProjectID: project, Name: "validation", Model: "actual-deployment", BaseURL: "https://provider.example/v1", APIKey: key, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if p.APIKey != "" {
		t.Fatal("public CRUD leaked key")
	}
	var stored string
	if err = st.db.QueryRow(`SELECT api_key FROM model_profiles WHERE id=$1`, p.ID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stored, secrets.Prefix) || strings.Contains(stored, key) {
		t.Fatal("key not encrypted")
	}
	read, ok := st.modelProfileByID(ctx, project, p.ID)
	if !ok || read.APIKey != key || read.Model != "actual-deployment" {
		t.Fatal("internal read lost configuration")
	}
	if _, err = st.updateModelProfile(ctx, ModelProfile{ProjectID: project, ID: p.ID, Name: "renamed"}); err != nil {
		t.Fatal(err)
	}
	read, ok = st.modelProfileByID(ctx, project, p.ID)
	if !ok || read.APIKey != key {
		t.Fatal("blank edit deleted credential")
	}
	if _, err = st.db.Exec(`UPDATE model_profiles SET api_key=$2 WHERE id=$1`, p.ID, key); err != nil {
		t.Fatal(err)
	}
	if err = st.migrateModelKeys(ctx); err != nil {
		t.Fatal(err)
	}
	if err = st.db.QueryRow(`SELECT api_key FROM model_profiles WHERE id=$1`, p.ID).Scan(&stored); err != nil || !strings.HasPrefix(stored, secrets.Prefix) {
		t.Fatal("legacy migration failed")
	}
	tasks, err := projecttest.NewTaskStore(st.db, dir)
	if err != nil {
		t.Fatal(err)
	}
	task, err := tasks.SaveTask(ctx, projecttest.Task{ProjectID: project, Name: "legacy task", BaseURL: "http://localhost:8082", Password: "test-task-password"})
	if err != nil {
		t.Fatal(err)
	}
	restarted, err := newStore(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.db.Close()
	read, ok = restarted.modelProfileByID(ctx, project, p.ID)
	if !ok || read.APIKey != key {
		t.Fatal("restart lost key")
	}
	tasks, err = projecttest.NewTaskStore(restarted.db, dir)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := tasks.Task(ctx, project, task.ID)
	if err != nil || saved.Password != "test-task-password" {
		t.Fatal("old task encryption changed")
	}
	original, err := os.ReadFile(filepath.Join(dir, "project-test.key"))
	if err != nil {
		t.Fatal(err)
	}
	wrong := make([]byte, 32)
	// Simulate a pre-upgrade database: task capsules already encrypted, but
	// model keys still plaintext. A wrong task key must not migrate any model.
	if _, err = st.db.Exec(`UPDATE model_profiles SET api_key=$2 WHERE id=$1`, p.ID, key); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "project-test.key"), wrong, 0600); err != nil {
		t.Fatal(err)
	}
	if incorrect, err := newStore(dsn); err == nil {
		incorrect.db.Close()
		t.Fatal("wrong key started server")
	}
	if err = st.db.QueryRow(`SELECT api_key FROM model_profiles WHERE id=$1`, p.ID).Scan(&stored); err != nil || stored != key {
		t.Fatal("wrong legacy task key partially migrated models")
	}
	if err = os.Remove(filepath.Join(dir, "project-test.key")); err != nil {
		t.Fatal(err)
	}
	if missing, err := newStore(dsn); err == nil {
		missing.db.Close()
		t.Fatal("missing key silently replaced")
	}
	if err = os.WriteFile(filepath.Join(dir, "project-test.key"), original, 0600); err != nil {
		t.Fatal(err)
	}
	if err = st.migrateModelKeys(ctx); err != nil {
		t.Fatal(err)
	}

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
		w.Write([]byte(`{"error":"provider unavailable test-model-secret-never-real"}`))
	}))
	defer provider.Close()
	policy, _ := security.NewPolicy([]string{provider.URL}, false)
	old := security.SetPolicy(policy)
	defer security.SetPolicy(old)
	_, err = st.db.Exec(`UPDATE model_profiles SET base_url=$2,verified=TRUE WHERE id=$1`, p.ID, provider.URL)
	if err != nil {
		t.Fatal(err)
	}
	if err = st.setAssistantModel(ctx, project, p.ID); err != nil {
		t.Fatal(err)
	}
	srv := &server{store: st}
	for _, prompt := range []string{"测试注册接口", "新增一个 POST 注册接口 http://localhost:8082/register"} {
		raw, _ := json.Marshal(agentInput{ProjectID: project, Prompt: prompt})
		w := httptest.NewRecorder()
		srv.agentDraft(w, httptest.NewRequest("POST", "/api/agent/draft", strings.NewReader(string(raw))))
		if w.Code != 502 || !strings.Contains(w.Body.String(), "assistant_model_failed") || strings.Contains(w.Body.String(), key) || strings.Contains(w.Body.String(), `"request"`) {
			t.Fatal("model failure returned a rule draft or secret", w.Code)
		}
	}
}
