package main

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const defaultPort = "8080"
const defaultDatabaseURL = "postgres://api_user:api_password@localhost:5432/api_test?sslmode=disable"

type Project struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Summary   string    `json:"summary"`
	CreatedAt time.Time `json:"createdAt"`
}
type Endpoint struct {
	ID        int               `json:"id"`
	ProjectID int               `json:"projectId"`
	Name      string            `json:"name"`
	Method    string            `json:"method"`
	URL       string            `json:"url"`
	Kind      string            `json:"kind"`
	Headers   map[string]string `json:"headers"`
	Query     map[string]string `json:"query"`
	Body      string            `json:"body"`
	Schema    map[string]any    `json:"schema,omitempty"`
	Enabled   bool              `json:"enabled"`
}
type RequestSpec struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Query   map[string]string `json:"query"`
	Body    string            `json:"body"`
}
type Assertion struct {
	Type     string `json:"type"`
	Path     string `json:"path"`
	Expected string `json:"expected"`
}
type TestCase struct {
	ID         int         `json:"id"`
	ProjectID  int         `json:"projectId"`
	APIID      int         `json:"apiId"`
	Name       string      `json:"name"`
	Prompt     string      `json:"prompt"`
	Request    RequestSpec `json:"request"`
	Assertions []Assertion `json:"assertions"`
	CreatedAt  time.Time   `json:"createdAt"`
}
type ProjectDocument struct {
	ID         int       `json:"id"`
	ProjectID  int       `json:"projectId"`
	Name       string    `json:"name"`
	DocType    string    `json:"docType"`
	Version    string    `json:"version"`
	Characters int       `json:"characters"`
	ChunkCount int       `json:"chunkCount"`
	Content    string    `json:"content,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}
type ModelProfile struct {
	ID         int       `json:"id"`
	ProjectID  int       `json:"projectId"`
	Name       string    `json:"name"`
	Provider   string    `json:"provider"`
	BaseURL    string    `json:"baseUrl"`
	APIKey     string    `json:"apiKey,omitempty"`
	APIKeyMask string    `json:"apiKeyMasked,omitempty"`
	Model      string    `json:"model"`
	AuthHeader string    `json:"authHeader"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"createdAt"`
}
type BusinessTestCase struct {
	Name       string      `json:"name"`
	Scenario   string      `json:"scenario"`
	Rule       string      `json:"rule"`
	Request    RequestSpec `json:"request"`
	Assertions []Assertion `json:"assertions"`
	Evidence   []string    `json:"evidence"`
}
type EvaluationResult struct {
	ProfileID         int      `json:"profileId"`
	ProfileName       string   `json:"profileName"`
	Provider          string   `json:"provider"`
	Question          string   `json:"question"`
	Answer            string   `json:"answer"`
	Success           bool     `json:"success"`
	DurationMs        int64    `json:"durationMs"`
	InputTokens       int      `json:"inputTokens"`
	OutputTokens      int      `json:"outputTokens"`
	RuleCoverage      int      `json:"ruleCoverage"`
	RuleTotal         int      `json:"ruleTotal"`
	CitationFound     bool     `json:"citationFound"`
	HallucinationRisk string   `json:"hallucinationRisk"`
	MatchedRules      []string `json:"matchedRules"`
	Evidence          []string `json:"evidence"`
	Error             string   `json:"error,omitempty"`
}
type EvaluationOutput struct {
	CreatedAt time.Time          `json:"createdAt"`
	Questions []string           `json:"questions"`
	Results   []EvaluationResult `json:"results"`
}
type RunRecord struct {
	ID             int               `json:"id"`
	ProjectID      int               `json:"projectId"`
	CaseID         int               `json:"caseId"`
	APIID          int               `json:"apiId"`
	CaseName       string            `json:"caseName"`
	StartedAt      time.Time         `json:"startedAt"`
	DurationMs     int64             `json:"durationMs"`
	FirstByteMs    int64             `json:"firstByteMs"`
	StatusCode     int               `json:"statusCode"`
	Success        bool              `json:"success"`
	ErrorCategory  string            `json:"errorCategory"`
	ErrorMessage   string            `json:"errorMessage"`
	ResponseHeader map[string]string `json:"responseHeaders"`
	ResponseBody   string            `json:"responseBody"`
	ContentType    string            `json:"contentType"`
	Stream         bool              `json:"stream"`
	InputTokens    int               `json:"inputTokens"`
	OutputTokens   int               `json:"outputTokens"`
	TokensPerSec   float64           `json:"tokensPerSec"`
	AssertionPass  int               `json:"assertionPass"`
	AssertionTotal int               `json:"assertionTotal"`
}

type store struct{ db *sql.DB }

const schemaSQL = `
CREATE TABLE IF NOT EXISTS projects (
  id SERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  summary TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS apis (
  id SERIAL PRIMARY KEY,
  project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  method TEXT NOT NULL,
  url TEXT NOT NULL,
  kind TEXT NOT NULL DEFAULT 'REST',
  headers JSONB NOT NULL DEFAULT '{}'::jsonb,
  query JSONB NOT NULL DEFAULT '{}'::jsonb,
  body TEXT NOT NULL DEFAULT '',
  schema JSONB NOT NULL DEFAULT '{}'::jsonb,
  enabled BOOLEAN NOT NULL DEFAULT TRUE
);
CREATE TABLE IF NOT EXISTS test_cases (
  id SERIAL PRIMARY KEY,
  project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  api_id INTEGER REFERENCES apis(id) ON DELETE SET NULL,
  name TEXT NOT NULL,
  prompt TEXT NOT NULL DEFAULT '',
  request JSONB NOT NULL,
  assertions JSONB NOT NULL DEFAULT '[]'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS runs (
  id BIGSERIAL PRIMARY KEY,
  case_id INTEGER REFERENCES test_cases(id) ON DELETE SET NULL,
  api_id INTEGER REFERENCES apis(id) ON DELETE SET NULL,
  case_name TEXT NOT NULL,
  started_at TIMESTAMPTZ NOT NULL,
  duration_ms BIGINT NOT NULL DEFAULT 0,
  first_byte_ms BIGINT NOT NULL DEFAULT 0,
  status_code INTEGER NOT NULL DEFAULT 0,
  success BOOLEAN NOT NULL DEFAULT FALSE,
  error_category TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
  response_headers JSONB NOT NULL DEFAULT '{}'::jsonb,
  response_body TEXT NOT NULL DEFAULT '',
  assertion_pass INTEGER NOT NULL DEFAULT 0,
  assertion_total INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_apis_project_id ON apis(project_id);
CREATE INDEX IF NOT EXISTS idx_runs_case_id ON runs(case_id);
CREATE INDEX IF NOT EXISTS idx_runs_started_at ON runs(started_at DESC);
ALTER TABLE runs ADD COLUMN IF NOT EXISTS content_type TEXT NOT NULL DEFAULT '';
ALTER TABLE runs ADD COLUMN IF NOT EXISTS is_stream BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE runs ADD COLUMN IF NOT EXISTS input_tokens INTEGER NOT NULL DEFAULT 0;
ALTER TABLE runs ADD COLUMN IF NOT EXISTS output_tokens INTEGER NOT NULL DEFAULT 0;
ALTER TABLE runs ADD COLUMN IF NOT EXISTS tokens_per_sec DOUBLE PRECISION NOT NULL DEFAULT 0;
ALTER TABLE runs ADD COLUMN IF NOT EXISTS project_id INTEGER NOT NULL DEFAULT 0;
UPDATE runs SET project_id=COALESCE((SELECT project_id FROM test_cases WHERE test_cases.id=runs.case_id),(SELECT project_id FROM apis WHERE apis.id=runs.api_id),0) WHERE project_id=0;
ALTER TABLE apis ADD COLUMN IF NOT EXISTS schema JSONB NOT NULL DEFAULT '{}'::jsonb;
CREATE TABLE IF NOT EXISTS agent_audits (
  id BIGSERIAL PRIMARY KEY,
  project_id INTEGER REFERENCES projects(id) ON DELETE CASCADE,
  api_id INTEGER REFERENCES apis(id) ON DELETE SET NULL,
  action TEXT NOT NULL,
  prompt TEXT NOT NULL DEFAULT '',
  result JSONB NOT NULL DEFAULT '{}'::jsonb,
  confirmed BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_agent_audits_created_at ON agent_audits(created_at DESC);
CREATE TABLE IF NOT EXISTS project_documents (
  id SERIAL PRIMARY KEY,
  project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  doc_type TEXT NOT NULL DEFAULT '业务规则',
  version TEXT NOT NULL DEFAULT 'v1',
  content TEXT NOT NULL,
  chunks JSONB NOT NULL DEFAULT '[]'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_project_documents_project_id ON project_documents(project_id);
CREATE TABLE IF NOT EXISTS model_profiles (
  id SERIAL PRIMARY KEY,
  project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  provider TEXT NOT NULL DEFAULT 'OpenAI Compatible',
  base_url TEXT NOT NULL,
  api_key TEXT NOT NULL DEFAULT '',
  model TEXT NOT NULL,
  auth_header TEXT NOT NULL DEFAULT 'Authorization',
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_model_profiles_project_id ON model_profiles(project_id);
`

func newStore(dsn string) (*store, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err = db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("连接 PostgreSQL 失败：%w", err)
	}
	if _, err = db.ExecContext(ctx, schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("初始化 PostgreSQL 表结构失败：%w", err)
	}
	s := &store{db: db}
	if err := s.seed(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *store) seed(ctx context.Context) error {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	var projectID int
	err := s.db.QueryRowContext(ctx, `INSERT INTO projects(name, summary) VALUES($1,$2) RETURNING id`, "本地接口演示项目", "用于演示 Agent 生成请求、真实执行和回归对比").Scan(&projectID)
	if err != nil {
		return err
	}
	items := []Endpoint{
		{ProjectID: projectID, Name: "回声接口 / Echo", Method: http.MethodPost, URL: "/api/demo/echo", Kind: "REST", Headers: map[string]string{"Content-Type": "application/json"}, Query: map[string]string{}, Body: `{"message":"hello from api tester"}`, Enabled: true},
		{ProjectID: projectID, Name: "需要鉴权的接口 / Auth", Method: http.MethodGet, URL: "/api/demo/auth", Kind: "REST", Headers: map[string]string{}, Query: map[string]string{}, Body: "", Enabled: true},
		{ProjectID: projectID, Name: "模型流式演示 / SSE", Method: http.MethodPost, URL: "/api/demo/llm", Kind: "LLM", Headers: map[string]string{"Content-Type": "application/json"}, Query: map[string]string{"stream": "true"}, Body: `{"question":"什么是 API 回归测试？"}`, Enabled: true},
	}
	for _, item := range items {
		if _, err := s.addAPI(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

func jsonBytes(value any) []byte { raw, _ := json.Marshal(value); return raw }
func decodeJSON(raw []byte, target any) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, target)
}
func normalizeMaps(item Endpoint) Endpoint {
	if item.Headers == nil {
		item.Headers = map[string]string{}
	}
	if item.Query == nil {
		item.Query = map[string]string{}
	}
	if item.Schema == nil {
		item.Schema = map[string]any{}
	}
	return item
}

func (s *store) projects(ctx context.Context) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,summary,created_at FROM projects ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Project
	for rows.Next() {
		var item Project
		if err := rows.Scan(&item.ID, &item.Name, &item.Summary, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func (s *store) addProject(ctx context.Context, item Project) (Project, error) {
	err := s.db.QueryRowContext(ctx, `INSERT INTO projects(name,summary) VALUES($1,$2) RETURNING id,created_at`, item.Name, item.Summary).Scan(&item.ID, &item.CreatedAt)
	return item, err
}

func chunkDocument(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return []string{}
	}
	paragraphs := strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == '\r' })
	chunks := make([]string, 0)
	var current strings.Builder
	flush := func() {
		value := strings.TrimSpace(current.String())
		if value != "" {
			chunks = append(chunks, value)
		}
		current.Reset()
	}
	for _, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}
		if current.Len() > 0 && current.Len()+len(paragraph)+1 > 900 {
			flush()
		}
		current.WriteString(paragraph)
		current.WriteByte('\n')
	}
	flush()
	if len(chunks) == 0 {
		chunks = []string{text}
	}
	return chunks
}

func maskAPIKey(key string) string {
	if len(key) > 4 {
		return "••••••" + key[len(key)-4:]
	}
	if key != "" {
		return "••••••"
	}
	return ""
}

func (s *store) addDocument(ctx context.Context, item ProjectDocument, chunks []string) (ProjectDocument, error) {
	if item.DocType == "" {
		item.DocType = "业务规则"
	}
	if item.Version == "" {
		item.Version = "v1"
	}
	item.Characters = len([]rune(item.Content))
	item.ChunkCount = len(chunks)
	err := s.db.QueryRowContext(ctx, `INSERT INTO project_documents(project_id,name,doc_type,version,content,chunks) VALUES($1,$2,$3,$4,$5,$6::jsonb) RETURNING id,created_at`, item.ProjectID, item.Name, item.DocType, item.Version, item.Content, jsonBytes(chunks)).Scan(&item.ID, &item.CreatedAt)
	return item, err
}

func scanDocument(row scanner, includeContent bool) (ProjectDocument, error) {
	var item ProjectDocument
	var content string
	var chunksRaw []byte
	if err := row.Scan(&item.ID, &item.ProjectID, &item.Name, &item.DocType, &item.Version, &content, &chunksRaw, &item.CreatedAt); err != nil {
		return item, err
	}
	if includeContent {
		item.Content = content
	}
	var chunks []string
	_ = decodeJSON(chunksRaw, &chunks)
	item.Characters = len([]rune(content))
	item.ChunkCount = len(chunks)
	return item, nil
}

func (s *store) documents(ctx context.Context, projectID int, includeContent bool) ([]ProjectDocument, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,project_id,name,doc_type,version,content,chunks,created_at FROM project_documents WHERE project_id=$1 ORDER BY id DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ProjectDocument{}
	for rows.Next() {
		item, err := scanDocument(rows, includeContent)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *store) documentByID(ctx context.Context, projectID, id int) (ProjectDocument, bool) {
	row := s.db.QueryRowContext(ctx, `SELECT id,project_id,name,doc_type,version,content,chunks,created_at FROM project_documents WHERE project_id=$1 AND id=$2`, projectID, id)
	item, err := scanDocument(row, true)
	return item, err == nil
}

func (s *store) addModelProfile(ctx context.Context, item ModelProfile) (ModelProfile, error) {
	err := s.db.QueryRowContext(ctx, `INSERT INTO model_profiles(project_id,name,provider,base_url,api_key,model,auth_header,enabled) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id,created_at`, item.ProjectID, item.Name, firstNonEmpty(item.Provider, "OpenAI Compatible"), item.BaseURL, item.APIKey, item.Model, firstNonEmpty(item.AuthHeader, "Authorization"), item.Enabled).Scan(&item.ID, &item.CreatedAt)
	item.APIKeyMask = maskAPIKey(item.APIKey)
	item.APIKey = ""
	return item, err
}

func (s *store) modelProfiles(ctx context.Context, projectID int, includeSecret bool) ([]ModelProfile, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,project_id,name,provider,base_url,api_key,model,auth_header,enabled,created_at FROM model_profiles WHERE project_id=$1 ORDER BY id`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ModelProfile{}
	for rows.Next() {
		var item ModelProfile
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.Name, &item.Provider, &item.BaseURL, &item.APIKey, &item.Model, &item.AuthHeader, &item.Enabled, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.APIKeyMask = maskAPIKey(item.APIKey)
		if !includeSecret {
			item.APIKey = ""
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *store) modelProfileByID(ctx context.Context, projectID, id int) (ModelProfile, bool) {
	var item ModelProfile
	err := s.db.QueryRowContext(ctx, `SELECT id,project_id,name,provider,base_url,api_key,model,auth_header,enabled,created_at FROM model_profiles WHERE project_id=$1 AND id=$2`, projectID, id).Scan(&item.ID, &item.ProjectID, &item.Name, &item.Provider, &item.BaseURL, &item.APIKey, &item.Model, &item.AuthHeader, &item.Enabled, &item.CreatedAt)
	return item, err == nil
}

type scanner interface{ Scan(...any) error }

func scanEndpoint(row scanner) (Endpoint, error) {
	var item Endpoint
	var headers, query, schema []byte
	err := row.Scan(&item.ID, &item.ProjectID, &item.Name, &item.Method, &item.URL, &item.Kind, &headers, &query, &item.Body, &schema, &item.Enabled)
	if err != nil {
		return item, err
	}
	_ = decodeJSON(headers, &item.Headers)
	_ = decodeJSON(query, &item.Query)
	_ = decodeJSON(schema, &item.Schema)
	return normalizeMaps(item), nil
}
func (s *store) apis(ctx context.Context, projectID int) ([]Endpoint, error) {
	query := `SELECT id,project_id,name,method,url,kind,headers,query,body,schema,enabled FROM apis ORDER BY id`
	args := []any{}
	if projectID > 0 {
		query = `SELECT id,project_id,name,method,url,kind,headers,query,body,schema,enabled FROM apis WHERE project_id=$1 ORDER BY id`
		args = append(args, projectID)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Endpoint
	for rows.Next() {
		item, err := scanEndpoint(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func (s *store) addAPI(ctx context.Context, item Endpoint) (Endpoint, error) {
	item = normalizeMaps(item)
	item.Method = strings.ToUpper(item.Method)
	err := s.db.QueryRowContext(ctx, `INSERT INTO apis(project_id,name,method,url,kind,headers,query,body,schema,enabled) VALUES($1,$2,$3,$4,$5,$6::jsonb,$7::jsonb,$8,$9::jsonb,$10) RETURNING id`, item.ProjectID, item.Name, item.Method, item.URL, firstNonEmpty(item.Kind, "REST"), jsonBytes(item.Headers), jsonBytes(item.Query), item.Body, jsonBytes(item.Schema), true).Scan(&item.ID)
	return item, err
}
func (s *store) getAPI(ctx context.Context, id int) (Endpoint, bool) {
	row := s.db.QueryRowContext(ctx, `SELECT id,project_id,name,method,url,kind,headers,query,body,schema,enabled FROM apis WHERE id=$1`, id)
	item, err := scanEndpoint(row)
	return item, err == nil
}

func (s *store) cases(ctx context.Context, projectIDs ...int) ([]TestCase, error) {
	query := `SELECT id,project_id,COALESCE(api_id,0),name,prompt,request,assertions,created_at FROM test_cases ORDER BY id DESC`
	args := []any{}
	if len(projectIDs) > 0 && projectIDs[0] > 0 {
		query = `SELECT id,project_id,COALESCE(api_id,0),name,prompt,request,assertions,created_at FROM test_cases WHERE project_id=$1 ORDER BY id DESC`
		args = append(args, projectIDs[0])
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TestCase
	for rows.Next() {
		var item TestCase
		var request, assertions []byte
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.APIID, &item.Name, &item.Prompt, &request, &assertions, &item.CreatedAt); err != nil {
			return nil, err
		}
		_ = decodeJSON(request, &item.Request)
		_ = decodeJSON(assertions, &item.Assertions)
		out = append(out, item)
	}
	return out, rows.Err()
}
func (s *store) addCase(ctx context.Context, item TestCase) (TestCase, error) {
	err := s.db.QueryRowContext(ctx, `INSERT INTO test_cases(project_id,api_id,name,prompt,request,assertions) VALUES($1,NULLIF($2,0),$3,$4,$5::jsonb,$6::jsonb) RETURNING id,created_at`, item.ProjectID, item.APIID, item.Name, item.Prompt, jsonBytes(item.Request), jsonBytes(item.Assertions)).Scan(&item.ID, &item.CreatedAt)
	return item, err
}
func (s *store) updateCase(ctx context.Context, item TestCase) error {
	_, err := s.db.ExecContext(ctx, `UPDATE test_cases SET project_id=$1,api_id=NULLIF($2,0),name=$3,prompt=$4,request=$5::jsonb,assertions=$6::jsonb WHERE id=$7`, item.ProjectID, item.APIID, item.Name, item.Prompt, jsonBytes(item.Request), jsonBytes(item.Assertions), item.ID)
	return err
}
func (s *store) getCase(ctx context.Context, id int) (TestCase, bool) {
	var item TestCase
	var request, assertions []byte
	err := s.db.QueryRowContext(ctx, `SELECT id,project_id,COALESCE(api_id,0),name,prompt,request,assertions,created_at FROM test_cases WHERE id=$1`, id).Scan(&item.ID, &item.ProjectID, &item.APIID, &item.Name, &item.Prompt, &request, &assertions, &item.CreatedAt)
	if err != nil {
		return item, false
	}
	_ = decodeJSON(request, &item.Request)
	_ = decodeJSON(assertions, &item.Assertions)
	return item, true
}

func scanRun(row scanner) (RunRecord, error) {
	var item RunRecord
	var headers []byte
	err := row.Scan(&item.ID, &item.ProjectID, &item.CaseID, &item.APIID, &item.CaseName, &item.StartedAt, &item.DurationMs, &item.FirstByteMs, &item.StatusCode, &item.Success, &item.ErrorCategory, &item.ErrorMessage, &headers, &item.ResponseBody, &item.ContentType, &item.Stream, &item.InputTokens, &item.OutputTokens, &item.TokensPerSec, &item.AssertionPass, &item.AssertionTotal)
	if err != nil {
		return item, err
	}
	_ = decodeJSON(headers, &item.ResponseHeader)
	if item.ResponseHeader == nil {
		item.ResponseHeader = map[string]string{}
	}
	return item, nil
}
func (s *store) runs(ctx context.Context, projectIDs ...int) ([]RunRecord, error) {
	query := `SELECT id,project_id,COALESCE(case_id,0),COALESCE(api_id,0),case_name,started_at,duration_ms,first_byte_ms,status_code,success,error_category,error_message,response_headers,response_body,content_type,is_stream,input_tokens,output_tokens,tokens_per_sec,assertion_pass,assertion_total FROM runs ORDER BY started_at DESC,id DESC`
	args := []any{}
	if len(projectIDs) > 0 && projectIDs[0] > 0 {
		query = `SELECT id,project_id,COALESCE(case_id,0),COALESCE(api_id,0),case_name,started_at,duration_ms,first_byte_ms,status_code,success,error_category,error_message,response_headers,response_body,content_type,is_stream,input_tokens,output_tokens,tokens_per_sec,assertion_pass,assertion_total FROM runs WHERE project_id=$1 ORDER BY started_at DESC,id DESC`
		args = append(args, projectIDs[0])
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RunRecord
	for rows.Next() {
		item, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func (s *store) addRun(ctx context.Context, item RunRecord) (RunRecord, error) {
	if item.ResponseHeader == nil {
		item.ResponseHeader = map[string]string{}
	}
	err := s.db.QueryRowContext(ctx, `INSERT INTO runs(project_id,case_id,api_id,case_name,started_at,duration_ms,first_byte_ms,status_code,success,error_category,error_message,response_headers,response_body,content_type,is_stream,input_tokens,output_tokens,tokens_per_sec,assertion_pass,assertion_total) VALUES($1,NULLIF($2,0),NULLIF($3,0),$4,$5,$6,$7,$8,$9,$10,$11,$12::jsonb,$13,$14,$15,$16,$17,$18,$19,$20) RETURNING id`, item.ProjectID, item.CaseID, item.APIID, item.CaseName, item.StartedAt, item.DurationMs, item.FirstByteMs, item.StatusCode, item.Success, item.ErrorCategory, item.ErrorMessage, jsonBytes(item.ResponseHeader), item.ResponseBody, item.ContentType, item.Stream, item.InputTokens, item.OutputTokens, item.TokensPerSec, item.AssertionPass, item.AssertionTotal).Scan(&item.ID)
	return item, err
}

type server struct {
	store         *store
	client        *http.Client
	llmMu         sync.RWMutex
	llmBaseURL    string
	llmAPIKey     string
	llmModel      string
	llmAuthHeader string
}

type agentAuditInput struct {
	ProjectID int            `json:"projectId"`
	APIID     int            `json:"apiId"`
	Action    string         `json:"action"`
	Prompt    string         `json:"prompt"`
	Result    map[string]any `json:"result"`
	Confirmed bool           `json:"confirmed"`
}

func (s *store) addAgentAudit(ctx context.Context, input agentAuditInput) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO agent_audits(project_id,api_id,action,prompt,result,confirmed) VALUES(NULLIF($1,0),NULLIF($2,0),$3,$4,$5::jsonb,$6)`, input.ProjectID, input.APIID, firstNonEmpty(input.Action, "agent_action"), input.Prompt, jsonBytes(input.Result), input.Confirmed)
	return err
}

func (s *server) agentAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var input agentAuditInput
	if err := readJSON(r, &input); err != nil || strings.TrimSpace(input.Action) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Agent 审计记录缺少 action"})
		return
	}
	if err := s.store.addAgentAudit(r.Context(), input); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"recorded": true})
}

type agentPlanInput struct {
	ProjectID int    `json:"projectId"`
	APIID     int    `json:"apiId"`
	Prompt    string `json:"prompt"`
}

type agentPlanStep struct {
	Order                int    `json:"order"`
	Tool                 string `json:"tool"`
	Title                string `json:"title"`
	Description          string `json:"description"`
	RequiresConfirmation bool   `json:"requiresConfirmation"`
}

type agentPlanOutput struct {
	Summary              string          `json:"summary"`
	Intent               string          `json:"intent"`
	Steps                []agentPlanStep `json:"steps"`
	RequiresConfirmation bool            `json:"requiresConfirmation"`
	Mode                 string          `json:"mode"`
}

// agentPlan 提供一个轻量的“计划—确认—执行”编排层。
// 它不直接执行任何工具，前端会把计划展示给用户，再调用现有草稿接口。
// 这样没有配置 LLM 时也能完整演示 Agent 的工作流；配置 LLM 后，Tool Calling
// 仍然负责生成具体参数，计划层负责控制步骤和安全边界。
func (s *server) agentPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var input agentPlanInput
	if err := readJSON(r, &input); err != nil || strings.TrimSpace(input.Prompt) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请输入 Agent 任务"})
		return
	}
	text := strings.ToLower(input.Prompt)
	add := isAddEndpointPrompt(input.Prompt)
	regression := strings.Contains(text, "回归") || strings.Contains(text, "重放") || strings.Contains(text, "版本")
	steps := make([]agentPlanStep, 0, 5)
	order := 1
	addStep := func(tool, title, description string, confirmation bool) {
		steps = append(steps, agentPlanStep{Order: order, Tool: tool, Title: title, Description: description, RequiresConfirmation: confirmation})
		order++
	}
	if input.APIID > 0 {
		addStep("get_api_schema", "读取接口契约", "获取当前接口的 URL、参数、请求体和响应约束，避免 Agent 臆造字段。", false)
	} else {
		addStep("list_apis", "查找可用接口", "先查询当前项目已有接口，再决定复用还是新增。", false)
	}
	if add {
		addStep("create_api_draft", "生成接口配置草稿", "根据自然语言、接口文档和环境变量生成名称、方法、URL、Headers、Body。", false)
		addStep("create_endpoint", "等待确认后保存接口", "用户检查草稿并确认后，才写入当前项目。", true)
	} else {
		addStep("create_api_draft", "生成测试请求草稿", "自动补齐请求参数、测试问题和基础断言。", false)
		if regression {
			addStep("run_api_test", "重放历史测试用例", "重新执行同一组请求，记录本次状态码、耗时和响应。", true)
			addStep("analyze_response", "比较结果并分析变化", "识别状态码、延迟和响应结构是否发生回归。", false)
		} else {
			addStep("run_api_test", "等待确认后执行测试", "用户确认请求内容后，才发送到被测 API。", true)
			addStep("analyze_response", "分析响应结果", "把 401、403、404、429、500 等错误转换为可执行的排查建议。", false)
		}
	}
	intent := "test_api"
	if add {
		intent = "add_api"
	} else if regression {
		intent = "regression"
	}
	writeJSON(w, http.StatusOK, agentPlanOutput{
		Summary: fmt.Sprintf("已规划 %d 个步骤，写入和执行动作需要你确认", len(steps)),
		Intent:  intent, Steps: steps, RequiresConfirmation: true, Mode: "pulse-state-agent",
	})
}

func main() {
	port := firstNonEmpty(os.Getenv("PORT"), defaultPort)
	dsn := firstNonEmpty(os.Getenv("DATABASE_URL"), defaultDatabaseURL)
	database, err := newStore(dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer database.db.Close()
	s := &server{store: database, client: &http.Client{Timeout: 30 * time.Second}, llmBaseURL: strings.TrimSpace(os.Getenv("LLM_BASE_URL")), llmAPIKey: strings.TrimSpace(os.Getenv("LLM_API_KEY")), llmModel: firstNonEmpty(os.Getenv("LLM_MODEL"), "gpt-4o-mini"), llmAuthHeader: firstNonEmpty(os.Getenv("LLM_AUTH_HEADER"), "Authorization")}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.health)
	mux.HandleFunc("/api/projects", s.projects)
	mux.HandleFunc("/api/apis", s.apis)
	mux.HandleFunc("/api/apis/import", s.importOpenAPI)
	mux.HandleFunc("/api/cases", s.cases)
	mux.HandleFunc("/api/runs", s.runs)
	mux.HandleFunc("/api/runs/batch", s.batchRuns)
	mux.HandleFunc("/api/runs/chain", s.chainRuns)
	mux.HandleFunc("/api/agent/draft", s.agentDraft)
	mux.HandleFunc("/api/agent/plan", s.agentPlan)
	mux.HandleFunc("/api/agent/document", s.agentDocument)
	mux.HandleFunc("/api/documents", s.documentsHandler)
	mux.HandleFunc("/api/documents/search", s.documentSearch)
	mux.HandleFunc("/api/models", s.modelProfilesHandler)
	mux.HandleFunc("/api/agent/business-cases", s.businessCases)
	mux.HandleFunc("/api/evaluations/run", s.evaluationRun)
	mux.HandleFunc("/api/agent/audit", s.agentAudit)
	mux.HandleFunc("/api/settings/llm", s.llmSettings)
	mux.HandleFunc("/api/regression", s.regression)
	mux.HandleFunc("/api/demo/echo", demoEcho)
	mux.HandleFunc("/api/demo/auth", demoAuth)
	mux.HandleFunc("/api/demo/llm", demoLLM)
	log.Printf("AI API Test Platform V1 listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, withCORS(mux)))
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func readJSON(r *http.Request, dst any) error {
	return json.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(dst)
}

func (s *server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "service": "ai-api-test-platform", "version": "v2", "storage": "postgresql"})
}
func (s *server) projects(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.Method == http.MethodGet {
		items, err := s.store.projects(ctx)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, items)
		return
	}
	if r.Method == http.MethodPost {
		var item Project
		if err := readJSON(r, &item); err != nil || strings.TrimSpace(item.Name) == "" {
			writeJSON(w, 400, map[string]string{"error": "项目名称不能为空"})
			return
		}
		created, err := s.store.addProject(ctx, item)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 201, created)
		return
	}
	writeJSON(w, 405, map[string]string{"error": "method not allowed"})
}
func (s *server) apis(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.Method == http.MethodGet {
		projectID, _ := strconv.Atoi(r.URL.Query().Get("projectId"))
		items, err := s.store.apis(ctx, projectID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, items)
		return
	}
	if r.Method == http.MethodPost {
		var item Endpoint
		if err := readJSON(r, &item); err != nil || item.ProjectID == 0 || item.Name == "" || item.URL == "" {
			writeJSON(w, 400, map[string]string{"error": "接口至少需要项目、名称和 URL"})
			return
		}
		created, err := s.store.addAPI(ctx, item)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 201, created)
		return
	}
	writeJSON(w, 405, map[string]string{"error": "method not allowed"})
}

type openAPIImportInput struct {
	ProjectID int             `json:"projectId"`
	Spec      json.RawMessage `json:"spec"`
}

func (s *server) importOpenAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var input openAPIImportInput
	if err := readJSON(r, &input); err != nil || input.ProjectID == 0 || len(input.Spec) == 0 {
		writeJSON(w, 400, map[string]string{"error": "请提供项目 ID 和 OpenAPI JSON"})
		return
	}
	var document struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(input.Spec, &document); err != nil || len(document.Paths) == 0 {
		writeJSON(w, 400, map[string]string{"error": "OpenAPI 文档缺少可解析的 paths"})
		return
	}
	allowed := map[string]bool{"get": true, "post": true, "put": true, "delete": true, "patch": true}
	created := []Endpoint{}
	for path, operations := range document.Paths {
		for method, rawOperation := range operations {
			if !allowed[strings.ToLower(method)] {
				continue
			}
			var operation struct {
				Summary     string          `json:"summary"`
				Description string          `json:"description"`
				OperationID string          `json:"operationId"`
				RequestBody json.RawMessage `json:"requestBody"`
			}
			_ = json.Unmarshal(rawOperation, &operation)
			name := firstNonEmpty(operation.OperationID, operation.Summary, operation.Description, strings.ToUpper(method)+" "+path)
			body := ""
			headers := map[string]string{}
			if len(operation.RequestBody) > 0 && string(operation.RequestBody) != "null" {
				body = "{}"
				headers["Content-Type"] = "application/json"
			}
			var operationSchema map[string]any
			_ = json.Unmarshal(rawOperation, &operationSchema)
			item, err := s.store.addAPI(r.Context(), Endpoint{ProjectID: input.ProjectID, Name: name, Method: strings.ToUpper(method), URL: path, Kind: "REST", Headers: headers, Query: map[string]string{}, Body: body, Schema: operationSchema, Enabled: true})
			if err != nil {
				writeJSON(w, 500, map[string]string{"error": err.Error()})
				return
			}
			created = append(created, item)
		}
	}
	writeJSON(w, 201, map[string]any{"imported": len(created), "apis": created})
}

func (s *server) cases(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPut {
		var item TestCase
		if err := readJSON(r, &item); err != nil || item.ID == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "测试用例参数不正确"})
			return
		}
		if err := s.store.updateCase(r.Context(), item); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, item)
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	projectID, _ := strconv.Atoi(r.URL.Query().Get("projectId"))
	items, err := s.store.cases(r.Context(), projectID)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, items)
}

type runInput struct {
	CaseID     int         `json:"caseId"`
	APIID      int         `json:"apiId"`
	ProjectID  int         `json:"projectId"`
	Name       string      `json:"name"`
	Prompt     string      `json:"prompt"`
	Request    RequestSpec `json:"request"`
	Assertions []Assertion `json:"assertions"`
	SaveCase   bool        `json:"saveCase"`
}

func (s *server) runs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.Method == http.MethodGet {
		projectID, _ := strconv.Atoi(r.URL.Query().Get("projectId"))
		items, err := s.store.runs(ctx, projectID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, items)
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var input runInput
	if err := readJSON(r, &input); err != nil {
		writeJSON(w, 400, map[string]string{"error": "测试请求参数不正确"})
		return
	}
	if input.CaseID > 0 {
		if c, ok := s.store.getCase(ctx, input.CaseID); ok {
			input.APIID, input.ProjectID, input.Request, input.Assertions, input.Name, input.Prompt = c.APIID, c.ProjectID, c.Request, c.Assertions, c.Name, c.Prompt
		}
	}
	if input.Request.URL == "" {
		if api, ok := s.store.getAPI(ctx, input.APIID); ok {
			input.Request = RequestSpec{Method: api.Method, URL: api.URL, Headers: api.Headers, Query: api.Query, Body: api.Body}
			if input.Name == "" {
				input.Name = api.Name
			}
		}
	}
	if input.Request.URL == "" {
		writeJSON(w, 400, map[string]string{"error": "请先选择接口或提供请求 URL"})
		return
	}
	if input.SaveCase {
		c, err := s.store.addCase(ctx, TestCase{ProjectID: input.ProjectID, APIID: input.APIID, Name: firstNonEmpty(input.Name, "未命名测试用例"), Prompt: input.Prompt, Request: input.Request, Assertions: input.Assertions})
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		input.CaseID = c.ID
	}
	run := s.execute(ctx, input)
	run.ProjectID = input.ProjectID
	run.CaseID = input.CaseID
	run.APIID = input.APIID
	run.CaseName = firstNonEmpty(input.Name, "临时请求")
	created, err := s.store.addRun(ctx, run)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 201, created)
}

type batchInput struct {
	Items       []runInput `json:"items"`
	Concurrency int        `json:"concurrency"`
}

type chainStepInput struct {
	APIID      int               `json:"apiId"`
	Name       string            `json:"name"`
	Request    RequestSpec       `json:"request"`
	Assertions []Assertion       `json:"assertions"`
	Extract    map[string]string `json:"extract"`
}

type chainInput struct {
	ProjectID int               `json:"projectId"`
	Variables map[string]string `json:"variables"`
	Steps     []chainStepInput  `json:"steps"`
}

type chainOutput struct {
	Success       bool              `json:"success"`
	FailedAt      int               `json:"failedAt"`
	Steps         []RunRecord       `json:"steps"`
	Variables     map[string]string `json:"variables"`
	VariableCount int               `json:"variableCount"`
}

func resolveChainRequest(request RequestSpec, variables map[string]string) RequestSpec {
	resolve := func(value string) string {
		result := value
		for key, item := range variables {
			result = strings.ReplaceAll(result, "{{"+key+"}}", item)
		}
		return result
	}
	request.URL = resolve(request.URL)
	request.Body = resolve(request.Body)
	for key, value := range request.Headers {
		request.Headers[key] = resolve(value)
	}
	for key, value := range request.Query {
		request.Query[key] = resolve(value)
	}
	return request
}

func maskChainVariables(variables map[string]string) map[string]string {
	masked := map[string]string{}
	for key, value := range variables {
		if strings.Contains(strings.ToLower(key), "token") || strings.Contains(strings.ToLower(key), "key") || strings.Contains(strings.ToLower(key), "secret") || strings.Contains(strings.ToLower(key), "password") {
			if len(value) > 6 {
				masked[key] = value[:3] + "••••••" + value[len(value)-3:]
			} else if value != "" {
				masked[key] = "••••••"
			} else {
				masked[key] = ""
			}
		} else {
			masked[key] = value
		}
	}
	return masked
}

func (s *server) chainRuns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var input chainInput
	if err := readJSON(r, &input); err != nil || len(input.Steps) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "链路测试至少需要一个步骤"})
		return
	}
	variables := map[string]string{}
	for key, value := range input.Variables {
		variables[key] = value
	}
	output := chainOutput{Success: true, FailedAt: 0, Steps: []RunRecord{}, Variables: map[string]string{}, VariableCount: 0}
	for index, step := range input.Steps {
		if step.Request.URL == "" && step.APIID > 0 {
			if endpoint, ok := s.store.getAPI(r.Context(), step.APIID); ok {
				step.Request = RequestSpec{Method: endpoint.Method, URL: endpoint.URL, Headers: endpoint.Headers, Query: endpoint.Query, Body: endpoint.Body}
				if step.Name == "" {
					step.Name = endpoint.Name
				}
			}
		}
		if step.Request.URL == "" {
			output.Success = false
			output.FailedAt = index + 1
			break
		}
		step.Request = resolveChainRequest(step.Request, variables)
		run := s.execute(r.Context(), runInput{ProjectID: input.ProjectID, APIID: step.APIID, Name: firstNonEmpty(step.Name, fmt.Sprintf("链路步骤 %d", index+1)), Request: step.Request, Assertions: step.Assertions})
		run.ProjectID, run.APIID = input.ProjectID, step.APIID
		run.CaseName = firstNonEmpty(step.Name, fmt.Sprintf("链路步骤 %d", index+1))
		stored, err := s.store.addRun(r.Context(), run)
		if err != nil {
			output.Success = false
			output.FailedAt = index + 1
			break
		}
		output.Steps = append(output.Steps, stored)
		if !stored.Success {
			output.Success = false
			output.FailedAt = index + 1
			break
		}
		for variable, path := range step.Extract {
			value, ok := jsonPathValue(stored.ResponseBody, path)
			if !ok {
				stored.Success = false
				stored.ErrorCategory = "链路变量提取失败"
				stored.ErrorMessage = fmt.Sprintf("无法从响应中提取变量 %s（路径：%s）", variable, path)
				output.Steps[len(output.Steps)-1] = stored
				output.Success = false
				output.FailedAt = index + 1
				break
			}
			variables[variable] = value
		}
		if !output.Success {
			break
		}
	}
	output.Variables = maskChainVariables(variables)
	output.VariableCount = len(variables)
	writeJSON(w, http.StatusOK, output)
}

func (s *server) batchRuns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var input batchInput
	if err := readJSON(r, &input); err != nil || len(input.Items) == 0 {
		writeJSON(w, 400, map[string]string{"error": "批量测试至少需要一条请求"})
		return
	}
	if input.Concurrency <= 0 || input.Concurrency > 20 {
		input.Concurrency = 4
	}
	results := make([]RunRecord, len(input.Items))
	errorsByIndex := make([]string, len(input.Items))
	jobs := make(chan int)
	var waitGroup sync.WaitGroup
	for worker := 0; worker < input.Concurrency; worker++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for index := range jobs {
				item := input.Items[index]
				ctx := r.Context()
				if item.CaseID > 0 {
					if testCase, ok := s.store.getCase(ctx, item.CaseID); ok {
						item.APIID, item.ProjectID, item.Request, item.Assertions, item.Name, item.Prompt = testCase.APIID, testCase.ProjectID, testCase.Request, testCase.Assertions, testCase.Name, testCase.Prompt
					}
				}
				if item.Request.URL == "" {
					if endpoint, ok := s.store.getAPI(ctx, item.APIID); ok {
						item.Request = RequestSpec{Method: endpoint.Method, URL: endpoint.URL, Headers: endpoint.Headers, Query: endpoint.Query, Body: endpoint.Body}
						item.Name = firstNonEmpty(item.Name, endpoint.Name)
					}
				}
				if item.Request.URL == "" {
					errorsByIndex[index] = "缺少请求 URL"
					continue
				}
				if item.SaveCase {
					testCase, err := s.store.addCase(ctx, TestCase{ProjectID: item.ProjectID, APIID: item.APIID, Name: firstNonEmpty(item.Name, "批量测试用例"), Prompt: item.Prompt, Request: item.Request, Assertions: item.Assertions})
					if err != nil {
						errorsByIndex[index] = err.Error()
						continue
					}
					item.CaseID = testCase.ID
				}
				run := s.execute(ctx, item)
				run.ProjectID, run.CaseID, run.APIID, run.CaseName = item.ProjectID, item.CaseID, item.APIID, firstNonEmpty(item.Name, "批量请求")
				stored, err := s.store.addRun(ctx, run)
				if err != nil {
					errorsByIndex[index] = err.Error()
					continue
				}
				results[index] = stored
			}
		}()
	}
	for index := range input.Items {
		jobs <- index
	}
	close(jobs)
	waitGroup.Wait()
	validResults := make([]RunRecord, 0, len(results))
	validErrors := make([]string, 0)
	for index, item := range results {
		if item.ID > 0 {
			validResults = append(validResults, item)
		} else if errorsByIndex[index] != "" {
			validErrors = append(validErrors, errorsByIndex[index])
		}
	}
	writeJSON(w, 201, map[string]any{"count": len(validResults), "runs": validResults, "errors": validErrors})
}

func extractDocxText(data []byte) (string, error) {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}
	for _, file := range archive.File {
		if file.Name != "word/document.xml" {
			continue
		}
		handle, err := file.Open()
		if err != nil {
			return "", err
		}
		defer handle.Close()
		decoder := xml.NewDecoder(handle)
		var parts []string
		for {
			token, tokenErr := decoder.Token()
			if tokenErr == io.EOF {
				break
			}
			if tokenErr != nil {
				return "", tokenErr
			}
			start, ok := token.(xml.StartElement)
			if !ok || start.Name.Local != "t" {
				continue
			}
			var value string
			if tokenErr := decoder.DecodeElement(&value, &start); tokenErr == nil && strings.TrimSpace(value) != "" {
				parts = append(parts, value)
			}
		}
		return strings.Join(parts, " "), nil
	}
	return "", errors.New("DOCX 文档缺少 word/document.xml")
}

var pdfTextPattern = regexp.MustCompile(`\(([^()]*)\)\s*Tj`)

func extractPDFText(data []byte) string {
	matches := pdfTextPattern.FindAllSubmatch(data, -1)
	parts := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			parts = append(parts, string(match[1]))
		}
	}
	return strings.Join(parts, " ")
}

func (s *server) agentDocument(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "文档不能超过 8MB"})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请上传文档文件"})
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 8<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "文档读取失败"})
		return
	}
	extension := strings.ToLower(filepath.Ext(header.Filename))
	text := ""
	switch extension {
	case ".txt", ".md", ".json", ".yaml", ".yml", ".csv", ".rst":
		text = string(data)
	case ".docx":
		text, err = extractDocxText(data)
	case ".pdf":
		text = extractPDFText(data)
		if text == "" {
			err = errors.New("PDF 没有可直接提取的文本层，请先转成 DOCX、Markdown 或 TXT")
		}
	default:
		err = fmt.Errorf("暂不支持 %s 文件，请上传 TXT、Markdown、JSON、YAML、DOCX 或 PDF", extension)
	}
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	text = strings.TrimSpace(text)
	if text == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "文档中没有可识别的文本"})
		return
	}
	if len(text) > 24000 {
		text = text[:24000]
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": header.Filename, "type": extension, "text": text, "characters": len([]rune(text))})
}

func (s *server) documentsHandler(w http.ResponseWriter, r *http.Request) {
	projectID, _ := strconv.Atoi(r.URL.Query().Get("projectId"))
	if r.Method == http.MethodGet {
		if projectID <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请提供 projectId，文档只允许在当前项目内读取"})
			return
		}
		items, err := s.store.documents(r.Context(), projectID, false)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, items)
		return
	}
	if r.Method == http.MethodDelete {
		id, _ := strconv.Atoi(r.URL.Query().Get("id"))
		if projectID <= 0 || id <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "文档删除需要 projectId 和 id"})
			return
		}
		result, err := s.store.db.ExecContext(r.Context(), `DELETE FROM project_documents WHERE project_id=$1 AND id=$2`, projectID, id)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		count, _ := result.RowsAffected()
		writeJSON(w, http.StatusOK, map[string]any{"deleted": count > 0})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if err := r.ParseMultipartForm(12 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "文档不能超过 12MB"})
		return
	}
	projectID, _ = strconv.Atoi(r.FormValue("projectId"))
	if projectID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请先选择项目"})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请上传 TXT、Markdown、JSON、YAML、DOCX 或 PDF"})
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 12<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "文档读取失败"})
		return
	}
	extension := strings.ToLower(filepath.Ext(header.Filename))
	text := ""
	switch extension {
	case ".txt", ".md", ".json", ".yaml", ".yml", ".csv", ".rst":
		text = string(data)
	case ".docx":
		text, err = extractDocxText(data)
	case ".pdf":
		text = extractPDFText(data)
		if text == "" {
			err = errors.New("PDF 没有可直接提取的文本层，请先转成 DOCX、Markdown 或 TXT")
		}
	default:
		err = fmt.Errorf("暂不支持 %s 文件", extension)
	}
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	text = strings.TrimSpace(text)
	if text == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "文档中没有可识别的文本"})
		return
	}
	if len([]rune(text)) > 120000 {
		text = string([]rune(text)[:120000])
	}
	item, err := s.store.addDocument(r.Context(), ProjectDocument{ProjectID: projectID, Name: header.Filename, DocType: firstNonEmpty(r.FormValue("docType"), "业务规则"), Version: firstNonEmpty(r.FormValue("version"), "v1"), Content: text}, chunkDocument(text))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

type documentSearchHit struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	DocType string `json:"docType"`
	Version string `json:"version"`
	Score   int    `json:"score"`
	Snippet string `json:"snippet"`
}

func searchDocumentText(content, query string) (int, string) {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return 1, firstNonEmpty(strings.TrimSpace(content), "")
	}
	terms := strings.FieldsFunc(query, func(r rune) bool {
		return r == ' ' || r == '\t' || r == ',' || r == '，' || r == '。' || r == '?' || r == '？'
	})
	score := 0
	best := ""
	for _, line := range strings.FieldsFunc(content, func(r rune) bool { return r == '\n' || r == '\r' }) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lineScore := 0
		lower := strings.ToLower(line)
		for _, term := range terms {
			if len([]rune(term)) >= 2 && strings.Contains(lower, term) {
				lineScore++
			}
		}
		if lineScore > score {
			score, best = lineScore, line
		}
	}
	if best == "" {
		best = string([]rune(content)[:minInt(len([]rune(content)), 240)])
	}
	return score, best
}

func (s *server) documentSearch(w http.ResponseWriter, r *http.Request) {
	projectID, _ := strconv.Atoi(r.URL.Query().Get("projectId"))
	query := r.URL.Query().Get("q")
	if projectID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请提供 projectId"})
		return
	}
	items, err := s.store.documents(r.Context(), projectID, true)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	hits := make([]documentSearchHit, 0, len(items))
	for _, item := range items {
		score, snippet := searchDocumentText(item.Content, query)
		hits = append(hits, documentSearchHit{ID: item.ID, Name: item.Name, DocType: item.DocType, Version: item.Version, Score: score, Snippet: snippet})
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > 8 {
		hits = hits[:8]
	}
	writeJSON(w, http.StatusOK, hits)
}

func (s *server) modelProfilesHandler(w http.ResponseWriter, r *http.Request) {
	projectID, _ := strconv.Atoi(r.URL.Query().Get("projectId"))
	if projectID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请提供 projectId"})
		return
	}
	if r.Method == http.MethodGet {
		items, err := s.store.modelProfiles(r.Context(), projectID, false)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, items)
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var input ModelProfile
	if err := readJSON(r, &input); err != nil || strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.BaseURL) == "" || strings.TrimSpace(input.Model) == "" || strings.TrimSpace(input.APIKey) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "模型名称、接口地址、模型名和 API Key 都不能为空"})
		return
	}
	input.ProjectID = projectID
	input.Enabled = true
	item, err := s.store.addModelProfile(r.Context(), input)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type agentInput struct {
	ProjectID   int    `json:"projectId"`
	APIID       int    `json:"apiId"`
	Prompt      string `json:"prompt"`
	DocumentIDs []int  `json:"documentIds"`
}
type agentOutput struct {
	Summary              string      `json:"summary"`
	Request              RequestSpec `json:"request"`
	Assertions           []Assertion `json:"assertions"`
	Reasoning            []string    `json:"reasoning"`
	Mode                 string      `json:"mode"`
	Intent               string      `json:"intent,omitempty"`
	Endpoint             *Endpoint   `json:"endpoint,omitempty"`
	RequiresConfirmation bool        `json:"requiresConfirmation,omitempty"`
}

func projectDocumentContext(ctx context.Context, s *store, projectID int, documentIDs []int, query string) (string, []string) {
	items, err := s.documents(ctx, projectID, true)
	if err != nil {
		return "", nil
	}
	selected := map[int]bool{}
	for _, id := range documentIDs {
		selected[id] = true
	}
	var parts []string
	evidence := []string{}
	for _, item := range items {
		if len(selected) > 0 && !selected[item.ID] {
			continue
		}
		_, snippet := searchDocumentText(item.Content, query)
		if query == "" || snippet == "" {
			snippet = item.Content
		}
		if len([]rune(snippet)) > 3000 {
			snippet = string([]rune(snippet)[:3000])
		}
		parts = append(parts, fmt.Sprintf("[%s · %s · %s]\n%s", item.Name, item.DocType, item.Version, snippet))
		evidence = append(evidence, item.Name)
	}
	return strings.Join(parts, "\n\n"), evidence
}

type businessCasesInput struct {
	ProjectID   int    `json:"projectId"`
	APIID       int    `json:"apiId"`
	DocumentIDs []int  `json:"documentIds"`
	Prompt      string `json:"prompt"`
}

func (s *server) businessCases(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var input businessCasesInput
	if err := readJSON(r, &input); err != nil || input.ProjectID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请提供 projectId"})
		return
	}
	contextText, evidence := projectDocumentContext(r.Context(), s.store, input.ProjectID, input.DocumentIDs, input.Prompt)
	if strings.TrimSpace(contextText) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "当前项目还没有业务规则文档，请先上传 PRD、业务规则或错误码文档"})
		return
	}
	base := Endpoint{Method: http.MethodPost, URL: "/api/orders", Headers: map[string]string{"Content-Type": "application/json"}, Query: map[string]string{}, Body: `{}`}
	if input.APIID > 0 {
		if endpoint, ok := s.store.getAPI(r.Context(), input.APIID); ok {
			base = endpoint
		}
	}
	baseBody := map[string]any{}
	if strings.TrimSpace(base.Body) != "" {
		_ = json.Unmarshal([]byte(base.Body), &baseBody)
	}
	if baseBody == nil {
		baseBody = map[string]any{}
	}
	setBody := func(values map[string]any) RequestSpec {
		body := map[string]any{}
		for key, value := range baseBody {
			body[key] = value
		}
		for key, value := range values {
			body[key] = value
		}
		raw, _ := json.Marshal(body)
		return RequestSpec{Method: firstNonEmpty(base.Method, http.MethodPost), URL: base.URL, Headers: base.Headers, Query: base.Query, Body: string(raw)}
	}
	findEvidence := func(keywords ...string) []string {
		result := []string{}
		for _, line := range strings.FieldsFunc(contextText, func(r rune) bool { return r == '\n' || r == '\r' }) {
			lower := strings.ToLower(line)
			for _, keyword := range keywords {
				if strings.Contains(lower, strings.ToLower(keyword)) {
					result = append(result, strings.TrimSpace(line))
					break
				}
			}
		}
		if len(result) > 3 {
			result = result[:3]
		}
		if len(result) == 0 {
			result = evidence
		}
		return result
	}
	cases := []BusinessTestCase{
		{Name: "正常下单", Scenario: "使用合法金额、数量和登录态创建订单", Rule: "验证业务规则全部满足时应成功", Request: setBody(map[string]any{"amount": 100, "quantity": 1}), Assertions: []Assertion{{Type: "status", Expected: "200"}}, Evidence: findEvidence("订单", "下单")},
		{Name: "金额边界", Scenario: "订单金额取业务规则允许的最低值", Rule: "验证最低金额边界", Request: setBody(map[string]any{"amount": 10, "quantity": 1}), Assertions: []Assertion{{Type: "status", Expected: "200"}}, Evidence: findEvidence("金额", "元")},
		{Name: "金额非法", Scenario: "订单金额低于业务规则要求", Rule: "验证非法金额被拒绝", Request: setBody(map[string]any{"amount": 9, "quantity": 1}), Assertions: []Assertion{{Type: "status", Expected: "400"}}, Evidence: findEvidence("金额", "低于")},
		{Name: "未登录下单", Scenario: "移除 Authorization 后创建订单", Rule: "验证未登录请求必须被拒绝", Request: setBody(map[string]any{"amount": 100, "quantity": 1}), Assertions: []Assertion{{Type: "status", Expected: "401"}}, Evidence: findEvidence("登录", "401", "Token")},
		{Name: "库存不足", Scenario: "购买数量超过可用库存", Rule: "验证库存不足不能下单", Request: setBody(map[string]any{"amount": 100, "quantity": 99999}), Assertions: []Assertion{{Type: "status", Expected: "409"}}, Evidence: findEvidence("库存")},
		{Name: "非法订单状态", Scenario: "传入业务规则之外的订单状态", Rule: "验证订单状态枚举约束", Request: setBody(map[string]any{"amount": 100, "status": "unknown"}), Assertions: []Assertion{{Type: "status", Expected: "400"}}, Evidence: findEvidence("状态", "pending", "paid")},
	}
	writeJSON(w, http.StatusOK, map[string]any{"source": "project-rag", "documents": evidence, "cases": cases})
}

type evaluationInput struct {
	ProjectID   int      `json:"projectId"`
	ProfileIDs  []int    `json:"profileIds"`
	DocumentIDs []int    `json:"documentIds"`
	Questions   []string `json:"questions"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

func normalizeChatURL(base string) string {
	base = strings.TrimSpace(base)
	if strings.Contains(base, "chat/completions") {
		return base
	}
	return strings.TrimRight(base, "/") + "/chat/completions"
}

func callChatCompletion(ctx context.Context, profile ModelProfile, system, user string) (string, int, int, error) {
	payload := map[string]any{"model": profile.Model, "temperature": 0.1, "messages": []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": user}}}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", 0, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, normalizeChatURL(profile.BaseURL), bytes.NewReader(raw))
	if err != nil {
		return "", 0, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(profile.AuthHeader, authValue(profile.AuthHeader, profile.APIKey))
	started := time.Now()
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		return "", 0, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", 0, 0, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", 0, 0, fmt.Errorf("模型服务返回 HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var result chatCompletionResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return "", 0, 0, err
	}
	if len(result.Choices) == 0 {
		return "", 0, 0, errors.New("模型没有返回回答")
	}
	inputTokens, outputTokens := result.Usage.PromptTokens, result.Usage.CompletionTokens
	answer := result.Choices[0].Message.Content
	if inputTokens == 0 {
		inputTokens = approxTokens(user)
	}
	if outputTokens == 0 {
		outputTokens = approxTokens(answer)
	}
	_ = started
	return answer, inputTokens, outputTokens, nil
}

func answerEvaluation(question, answer, contextText string) (int, int, bool, string, []string) {
	rules := []string{}
	for _, line := range strings.FieldsFunc(contextText, func(r rune) bool { return r == '\n' || r == '\r' }) {
		line = strings.TrimSpace(line)
		if line == "" || len([]rune(line)) < 4 {
			continue
		}
		lower := strings.ToLower(line)
		if strings.Contains(lower, "金额") || strings.Contains(lower, "库存") || strings.Contains(lower, "退款") || strings.Contains(lower, "取消") || strings.Contains(lower, "登录") || strings.Contains(lower, "状态") || strings.Contains(lower, "401") {
			rules = append(rules, line)
		}
	}
	if len(rules) > 8 {
		rules = rules[:8]
	}
	matched := []string{}
	answerLower := strings.ToLower(answer)
	for _, rule := range rules {
		keywords := strings.FieldsFunc(strings.ToLower(rule), func(r rune) bool { return r == ' ' || r == '，' || r == '。' || r == ',' || r == '：' || r == ':' })
		for _, keyword := range keywords {
			if len([]rune(keyword)) >= 2 && strings.Contains(answerLower, keyword) {
				matched = append(matched, rule)
				break
			}
		}
	}
	citation := strings.Contains(answer, "文档") || strings.Contains(answer, "规则") || strings.Contains(answer, "依据") || strings.Contains(answer, "根据")
	risk := "高"
	if len(rules) == 0 {
		risk = "未知"
	} else if len(matched) >= len(rules) {
		risk = "低"
	} else if len(matched) > 0 {
		risk = "中"
	}
	_ = question
	return len(matched), len(rules), citation, risk, matched
}

func (s *server) evaluationRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var input evaluationInput
	if err := readJSON(r, &input); err != nil || input.ProjectID <= 0 || len(input.ProfileIDs) == 0 || len(input.Questions) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请至少选择一个模型和一个评测问题"})
		return
	}
	profiles := []ModelProfile{}
	for _, id := range input.ProfileIDs {
		if profile, ok := s.store.modelProfileByID(r.Context(), input.ProjectID, id); ok && profile.Enabled {
			profiles = append(profiles, profile)
		}
	}
	if len(profiles) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "没有找到当前项目下可用的模型配置"})
		return
	}
	results := make([]EvaluationResult, len(profiles)*len(input.Questions))
	var group sync.WaitGroup
	var lock sync.Mutex
	for profileIndex, profile := range profiles {
		for questionIndex, question := range input.Questions {
			index := profileIndex*len(input.Questions) + questionIndex
			group.Add(1)
			go func(index int, profile ModelProfile, question string) {
				defer group.Done()
				started := time.Now()
				contextText, evidence := projectDocumentContext(r.Context(), s.store, input.ProjectID, input.DocumentIDs, question)
				system := "你是业务规则评测助手。只能依据提供的项目文档回答，不知道就明确说文档未说明。回答中请列出依据，不要补充文档外的业务规则。"
				user := fmt.Sprintf("项目文档：\n%s\n\n问题：%s\n请直接回答，并在结尾列出依据文档。", contextText, question)
				answer, inTokens, outTokens, err := callChatCompletion(r.Context(), profile, system, user)
				item := EvaluationResult{ProfileID: profile.ID, ProfileName: profile.Name, Provider: profile.Provider, Question: question, DurationMs: time.Since(started).Milliseconds(), InputTokens: inTokens, OutputTokens: outTokens, Evidence: evidence}
				if err != nil {
					item.Error = err.Error()
				} else {
					item.Success = true
					item.Answer = answer
					item.RuleCoverage, item.RuleTotal, item.CitationFound, item.HallucinationRisk, item.MatchedRules = answerEvaluation(question, answer, contextText)
				}
				lock.Lock()
				results[index] = item
				lock.Unlock()
			}(index, profile, question)
		}
	}
	group.Wait()
	writeJSON(w, http.StatusOK, EvaluationOutput{CreatedAt: time.Now(), Questions: input.Questions, Results: results})
}

type toolCallResponse struct {
	Choices []struct {
		Message struct {
			ToolCalls []struct {
				Function struct {
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
}

type llmSettingsInput struct {
	BaseURL    string `json:"baseUrl"`
	APIKey     string `json:"apiKey"`
	Model      string `json:"model"`
	AuthHeader string `json:"authHeader"`
}

func (s *server) llmSettings(w http.ResponseWriter, r *http.Request) {
	s.llmMu.Lock()
	defer s.llmMu.Unlock()
	if r.Method == http.MethodGet {
		masked := ""
		if len(s.llmAPIKey) > 4 {
			masked = "••••••" + s.llmAPIKey[len(s.llmAPIKey)-4:]
		} else if s.llmAPIKey != "" {
			masked = "••••••"
		}
		writeJSON(w, http.StatusOK, map[string]any{"baseUrl": s.llmBaseURL, "model": s.llmModel, "authHeader": s.llmAuthHeader, "configured": s.llmBaseURL != "" && s.llmAPIKey != "", "apiKeyMasked": masked})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var input llmSettingsInput
	if err := readJSON(r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "模型配置格式不正确"})
		return
	}
	if strings.TrimSpace(input.APIKey) != "" && !strings.HasPrefix(input.APIKey, "••••••") {
		s.llmAPIKey = strings.TrimSpace(input.APIKey)
	}
	s.llmBaseURL = strings.TrimSpace(input.BaseURL)
	s.llmModel = firstNonEmpty(strings.TrimSpace(input.Model), "gpt-4o-mini")
	s.llmAuthHeader = firstNonEmpty(strings.TrimSpace(input.AuthHeader), "Authorization")
	writeJSON(w, http.StatusOK, map[string]any{"baseUrl": s.llmBaseURL, "model": s.llmModel, "authHeader": s.llmAuthHeader, "configured": s.llmBaseURL != "" && s.llmAPIKey != ""})
}

func (s *server) llmDraft(ctx context.Context, prompt string, base Endpoint, knowledge string) (agentOutput, error) {
	s.llmMu.RLock()
	endpoint, key, model, authHeader := s.llmBaseURL, s.llmAPIKey, s.llmModel, s.llmAuthHeader
	s.llmMu.RUnlock()
	if endpoint == "" || key == "" {
		return agentOutput{}, errors.New("未配置 LLM_BASE_URL 或 LLM_API_KEY")
	}
	tool := map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        "propose_api_test",
			"description": "根据用户测试目标生成一份可确认的 API 测试请求草稿，不执行请求。",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"summary":    map[string]string{"type": "string"},
					"method":     map[string]any{"type": "string", "enum": []string{"GET", "POST", "PUT", "DELETE"}},
					"url":        map[string]string{"type": "string"},
					"headers":    map[string]any{"type": "object", "additionalProperties": map[string]string{"type": "string"}},
					"query":      map[string]any{"type": "object", "additionalProperties": map[string]string{"type": "string"}},
					"body":       map[string]string{"type": "string"},
					"assertions": map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]string{"type": "string"}}},
					"reasoning":  map[string]any{"type": "array", "items": map[string]string{"type": "string"}},
				},
				"required": []string{"summary", "method", "url", "headers", "query", "body", "assertions", "reasoning"},
			},
		},
	}
	system := "你是 API 测试规划 Agent。你只生成请求草稿，不执行请求。必须复用接口上下文中的认证信息，不要臆造密钥；所有结果都要让用户确认后才能执行。断言优先包含状态码，JSON 字段检查使用 json_field。"
	user := fmt.Sprintf("测试目标：%s\n参考接口上下文：%s\n项目知识库检索结果（只能作为补充依据）：%s", prompt, string(jsonBytes(base)), firstNonEmpty(knowledge, "无"))
	payload := map[string]any{
		"model":       model,
		"temperature": 0,
		"messages":    []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": user}},
		"tools":       []any{tool},
		"tool_choice": map[string]any{"type": "function", "function": map[string]string{"name": "propose_api_test"}},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return agentOutput{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return agentOutput{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(authHeader, authValue(authHeader, key))
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return agentOutput{}, err
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return agentOutput{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return agentOutput{}, fmt.Errorf("模型服务返回 HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	var response toolCallResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return agentOutput{}, err
	}
	if len(response.Choices) == 0 || len(response.Choices[0].Message.ToolCalls) == 0 {
		return agentOutput{}, errors.New("模型没有返回工具调用结果")
	}
	var args struct {
		Summary    string            `json:"summary"`
		Method     string            `json:"method"`
		URL        string            `json:"url"`
		Headers    map[string]string `json:"headers"`
		Query      map[string]string `json:"query"`
		Body       string            `json:"body"`
		Assertions []Assertion       `json:"assertions"`
		Reasoning  []string          `json:"reasoning"`
	}
	if err := json.Unmarshal([]byte(response.Choices[0].Message.ToolCalls[0].Function.Arguments), &args); err != nil {
		return agentOutput{}, err
	}
	if args.Method == "" || args.URL == "" {
		return agentOutput{}, errors.New("模型生成的请求缺少 method 或 url")
	}
	if args.Headers == nil {
		args.Headers = base.Headers
	}
	if args.Query == nil {
		args.Query = base.Query
	}
	if args.Assertions == nil {
		args.Assertions = []Assertion{{Type: "status", Expected: "200"}}
	}
	return agentOutput{Summary: firstNonEmpty(args.Summary, "模型生成的 API 测试草稿"), Request: RequestSpec{Method: strings.ToUpper(args.Method), URL: args.URL, Headers: args.Headers, Query: args.Query, Body: args.Body}, Assertions: args.Assertions, Reasoning: args.Reasoning, Mode: "llm"}, nil
}

func authValue(header, key string) string {
	if strings.EqualFold(header, "Authorization") && !strings.HasPrefix(strings.ToLower(key), "bearer ") {
		return "Bearer " + key
	}
	return key
}

var agentMethodPattern = regexp.MustCompile(`(?i)\b(GET|POST|PUT|DELETE|PATCH)\b`)
var agentURLPattern = regexp.MustCompile(`https?://[^\s，。；;]+|/[A-Za-z0-9_./:{}?=&%-]+`)

func isAddEndpointPrompt(prompt string) bool {
	text := strings.ToLower(prompt)
	if strings.Contains(text, "测试用例") || strings.Contains(text, "测试案例") {
		if !strings.Contains(text, "添加接口") && !strings.Contains(text, "新增接口") && !strings.Contains(text, "录入接口") && !strings.Contains(text, "配置接口") {
			return false
		}
	}
	keywords := []string{"添加接口", "新增接口", "创建接口", "录入接口", "配置接口", "保存接口", "加一个接口", "建一个接口"}
	for _, keyword := range keywords {
		if strings.Contains(text, strings.ToLower(keyword)) {
			return true
		}
	}
	return strings.Contains(text, "接口") && (strings.Contains(text, "添加") || strings.Contains(text, "新增") || strings.Contains(text, "创建"))
}

func cleanAgentURL(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), "，。；;,.!！")
}

func endpointNameFromPrompt(prompt string) string {
	for _, marker := range []string{"接口：", "接口:", "接口 ", "添加", "新增", "创建"} {
		if index := strings.Index(prompt, marker); index >= 0 {
			candidate := strings.TrimSpace(prompt[index+len(marker):])
			candidate = strings.Trim(candidate, "：:，,。 ")
			if match := agentMethodPattern.FindStringIndex(candidate); match != nil {
				candidate = strings.TrimSpace(candidate[:match[0]])
			}
			if match := agentURLPattern.FindStringIndex(candidate); match != nil {
				candidate = strings.TrimSpace(candidate[:match[0]])
			}
			candidate = strings.Trim(candidate, "：:，,。 ")
			if candidate != "" && len([]rune(candidate)) <= 32 {
				return candidate
			}
		}
	}
	return "AI 新增接口"
}

func ruleEndpointDraft(prompt string, projectID int) Endpoint {
	method := http.MethodGet
	if match := agentMethodPattern.FindStringSubmatch(prompt); len(match) > 1 {
		method = strings.ToUpper(match[1])
	} else if strings.Contains(prompt, "登录") || strings.Contains(prompt, "提交") || strings.Contains(prompt, "创建") || strings.Contains(prompt, "发送") {
		method = http.MethodPost
	}
	path := "/api/new-endpoint"
	if match := agentURLPattern.FindString(prompt); match != "" {
		path = cleanAgentURL(match)
	}
	kind := "REST"
	if strings.Contains(prompt, "大模型") || strings.Contains(strings.ToLower(prompt), "llm") || strings.Contains(prompt, "模型接口") {
		kind = "LLM"
	}
	if strings.Contains(prompt, "图片") || strings.Contains(prompt, "多模态") {
		kind = "Multimodal"
	}
	headers := map[string]string{}
	body := ""
	if kind == "LLM" {
		headers["Content-Type"] = "application/json"
		if strings.Contains(prompt, "Azure") {
			headers["api-key"] = "{{azureApiKey}}"
		} else {
			headers["Authorization"] = "Bearer {{apiKey}}"
		}
		body = `{"model":"{{model}}","messages":[{"role":"user","content":"hello"}]}`
	} else if strings.Contains(prompt, "JSON") || strings.Contains(strings.ToLower(prompt), "json") || method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch {
		headers["Content-Type"] = "application/json"
		body = `{"message":"hello"}`
	}
	if strings.Contains(strings.ToLower(prompt), "email") && strings.Contains(strings.ToLower(prompt), "password") {
		headers["Content-Type"] = "application/json"
		body = `{"email":"","password":""}`
	}
	if start := strings.Index(prompt, "{"); start >= 0 {
		if end := strings.LastIndex(prompt, "}"); end > start {
			candidate := strings.TrimSpace(prompt[start : end+1])
			var decoded map[string]any
			if json.Unmarshal([]byte(candidate), &decoded) == nil {
				body = candidate
				if headers["Content-Type"] == "" {
					headers["Content-Type"] = "application/json"
				}
			}
		}
	}
	if strings.Contains(prompt, "Token") || strings.Contains(strings.ToLower(prompt), "token") || strings.Contains(prompt, "鉴权") {
		headers["Authorization"] = "Bearer {{token}}"
	}
	return Endpoint{ProjectID: projectID, Name: endpointNameFromPrompt(prompt), Method: method, URL: path, Kind: kind, Headers: headers, Query: map[string]string{}, Body: body, Enabled: true}
}

func schemaMap(value any) map[string]any {
	item, _ := value.(map[string]any)
	return item
}

func schemaSample(schema map[string]any) any {
	if schema == nil {
		return nil
	}
	if value, ok := schema["example"]; ok {
		return value
	}
	if value, ok := schema["default"]; ok {
		return value
	}
	switch fmt.Sprint(schema["type"]) {
	case "object":
		result := map[string]any{}
		if properties, ok := schema["properties"].(map[string]any); ok {
			for name, raw := range properties {
				if sample := schemaSample(schemaMap(raw)); sample != nil {
					result[name] = sample
				}
			}
		}
		return result
	case "array":
		if sample := schemaSample(schemaMap(schema["items"])); sample != nil {
			return []any{sample}
		}
		return []any{}
	case "integer", "number":
		return 0
	case "boolean":
		return false
	default:
		return ""
	}
}

func schemaBodyAndAssertions(schema map[string]any) (string, []Assertion) {
	if schema == nil {
		return "", nil
	}
	body := ""
	if requestBody, ok := schema["requestBody"].(map[string]any); ok {
		if content, ok := requestBody["content"].(map[string]any); ok {
			for _, raw := range []string{"application/json", "application/*+json"} {
				if media, ok := content[raw].(map[string]any); ok {
					if sample := schemaSample(schemaMap(media["schema"])); sample != nil {
						if data, err := json.MarshalIndent(sample, "", "  "); err == nil {
							body = string(data)
						}
					}
					break
				}
			}
		}
	}
	assertions := []Assertion{}
	if responses, ok := schema["responses"].(map[string]any); ok {
		if response, ok := responses["200"].(map[string]any); ok {
			if content, ok := response["content"].(map[string]any); ok {
				if media, ok := content["application/json"].(map[string]any); ok {
					if responseSchema := schemaMap(media["schema"]); responseSchema != nil {
						if required, ok := responseSchema["required"].([]any); ok {
							for _, field := range required {
								assertions = append(assertions, Assertion{Type: "json_field", Path: fmt.Sprint(field), Expected: "存在"})
							}
						}
					}
				}
			}
		}
	}
	return body, assertions
}

func (s *server) llmEndpointDraft(ctx context.Context, prompt string, projectID int, knowledge string) (Endpoint, []string, error) {
	s.llmMu.RLock()
	endpoint, key, model, authHeader := s.llmBaseURL, s.llmAPIKey, s.llmModel, s.llmAuthHeader
	s.llmMu.RUnlock()
	if endpoint == "" || key == "" {
		return Endpoint{}, nil, errors.New("未配置 LLM_BASE_URL 或 LLM_API_KEY")
	}
	tool := map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        "propose_api_config",
			"description": "根据用户描述生成一个待确认的 API 接口配置草稿，不保存、不发送请求。密钥只能使用环境变量占位符。",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name":      map[string]string{"type": "string"},
					"method":    map[string]any{"type": "string", "enum": []string{"GET", "POST", "PUT", "DELETE", "PATCH"}},
					"url":       map[string]string{"type": "string"},
					"kind":      map[string]any{"type": "string", "enum": []string{"REST", "LLM", "Multimodal"}},
					"headers":   map[string]any{"type": "object", "additionalProperties": map[string]string{"type": "string"}},
					"query":     map[string]any{"type": "object", "additionalProperties": map[string]string{"type": "string"}},
					"body":      map[string]string{"type": "string"},
					"reasoning": map[string]any{"type": "array", "items": map[string]string{"type": "string"}},
				},
				"required": []string{"name", "method", "url", "kind", "headers", "query", "body", "reasoning"},
			},
		},
	}
	system := "你是 API 配置助手。只生成接口配置草稿，不保存数据库、不发送请求。URL 必须是 http/https 或以 / 开头。绝对不能生成真实密钥，认证值使用 {{token}}、{{apiKey}}、{{azureApiKey}} 等占位符。"
	payload := map[string]any{"model": model, "temperature": 0, "messages": []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": fmt.Sprintf("项目ID：%d\n用户描述：%s\n项目知识库检索结果：%s", projectID, prompt, firstNonEmpty(knowledge, "无"))}}, "tools": []any{tool}, "tool_choice": map[string]any{"type": "function", "function": map[string]string{"name": "propose_api_config"}}}
	raw, err := json.Marshal(payload)
	if err != nil {
		return Endpoint{}, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return Endpoint{}, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(authHeader, authValue(authHeader, key))
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return Endpoint{}, nil, err
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return Endpoint{}, nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Endpoint{}, nil, fmt.Errorf("模型服务返回 HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	var response toolCallResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return Endpoint{}, nil, err
	}
	if len(response.Choices) == 0 || len(response.Choices[0].Message.ToolCalls) == 0 {
		return Endpoint{}, nil, errors.New("模型没有返回接口配置工具调用")
	}
	var args struct {
		Name      string            `json:"name"`
		Method    string            `json:"method"`
		URL       string            `json:"url"`
		Kind      string            `json:"kind"`
		Headers   map[string]string `json:"headers"`
		Query     map[string]string `json:"query"`
		Body      string            `json:"body"`
		Reasoning []string          `json:"reasoning"`
	}
	if err := json.Unmarshal([]byte(response.Choices[0].Message.ToolCalls[0].Function.Arguments), &args); err != nil {
		return Endpoint{}, nil, err
	}
	if args.Name == "" || args.Method == "" || args.URL == "" {
		return Endpoint{}, nil, errors.New("模型生成的接口配置缺少名称、方法或 URL")
	}
	if args.Headers == nil {
		args.Headers = map[string]string{}
	}
	if args.Query == nil {
		args.Query = map[string]string{}
	}
	return Endpoint{ProjectID: projectID, Name: args.Name, Method: strings.ToUpper(args.Method), URL: cleanAgentURL(args.URL), Kind: firstNonEmpty(args.Kind, "REST"), Headers: args.Headers, Query: args.Query, Body: args.Body, Enabled: true}, args.Reasoning, nil
}

func (s *server) agentDraft(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var input agentInput
	if err := readJSON(r, &input); err != nil || strings.TrimSpace(input.Prompt) == "" {
		writeJSON(w, 400, map[string]string{"error": "请输入自然语言测试目标"})
		return
	}
	knowledge, documentEvidence := projectDocumentContext(r.Context(), s.store, input.ProjectID, input.DocumentIDs, input.Prompt)
	if isAddEndpointPrompt(input.Prompt) {
		endpoint := ruleEndpointDraft(input.Prompt, input.ProjectID)
		reasoning := []string{"识别到用户意图是新增接口，而不是执行测试", "从自然语言中提取接口名称、请求方法和 URL", "认证信息使用环境变量占位符，不让模型接触真实密钥", "接口写入数据库前需要用户点击确认"}
		mode := "rule-agent"
		if llmEndpoint, llmReasoning, err := s.llmEndpointDraft(r.Context(), input.Prompt, input.ProjectID, knowledge); err == nil {
			endpoint = llmEndpoint
			reasoning = append([]string{"已使用 OpenAI 兼容 Tool Calling 生成结构化接口配置"}, llmReasoning...)
			mode = "llm-tool-calling"
		} else if s.llmAPIKey != "" {
			log.Printf("real LLM endpoint agent unavailable, fallback to rule agent: %v", err)
		}
		if len(documentEvidence) > 0 {
			reasoning = append(reasoning, "已检索当前项目文档："+strings.Join(documentEvidence, "、"))
		}
		_ = s.store.addAgentAudit(r.Context(), agentAuditInput{ProjectID: input.ProjectID, Action: "draft_generated", Prompt: input.Prompt, Result: map[string]any{"intent": "add_api", "mode": mode, "endpoint": endpoint}})
		writeJSON(w, 200, agentOutput{Summary: fmt.Sprintf("已生成“%s”的接口配置草稿，请确认后添加", endpoint.Name), Reasoning: reasoning, Mode: mode, Intent: "add_api", Endpoint: &endpoint, RequiresConfirmation: true})
		return
	}
	var base Endpoint
	var ok bool
	if input.APIID > 0 {
		base, ok = s.store.getAPI(r.Context(), input.APIID)
	}
	if !ok {
		for _, item := range s.mustAPIs(r.Context(), input.ProjectID) {
			if strings.Contains(input.Prompt, "模型") && item.Kind == "LLM" {
				base, ok = item, true
				break
			}
			if strings.Contains(input.Prompt, "鉴权") && strings.Contains(item.Name, "Auth") {
				base, ok = item, true
				break
			}
		}
	}
	if !ok {
		base = Endpoint{Method: http.MethodPost, URL: "/api/demo/echo", Name: "Agent 临时生成的回声请求", Headers: map[string]string{"Content-Type": "application/json"}, Query: map[string]string{}, Body: `{}`}
	}
	if output, err := s.llmDraft(r.Context(), input.Prompt, base, knowledge); err == nil {
		writeJSON(w, 200, output)
		return
	} else if os.Getenv("LLM_API_KEY") != "" {
		log.Printf("real LLM agent unavailable, fallback to rule agent: %v", err)
	}
	body := base.Body
	assertions := []Assertion{{Type: "status", Expected: "200"}}
	if body == "" || strings.TrimSpace(body) == "{}" {
		if schemaBody, schemaAssertions := schemaBodyAndAssertions(base.Schema); schemaBody != "" {
			body = schemaBody
			if len(schemaAssertions) > 0 {
				assertions = schemaAssertions
			}
		}
	}
	if strings.Contains(input.Prompt, "退款") {
		body = `{"question":"退款流程是什么？","topic":"refund"}`
	}
	if strings.Contains(input.Prompt, "JSON") || strings.Contains(strings.ToLower(input.Prompt), "json") {
		body = `{"question":"请返回结构化结果","response_format":"json_object"}`
	}
	if body == "" && (base.Method == http.MethodPost || base.Method == http.MethodPut) {
		body = `{"message":"generated by agent"}`
	}
	if strings.Contains(input.Prompt, "JSON") || strings.Contains(strings.ToLower(input.Prompt), "json") {
		assertions = append(assertions, Assertion{Type: "json_field", Path: "message", Expected: "存在"})
	}
	reasoning := []string{"识别测试对象和接口类型", "复用已保存的接口地址与认证配置", "根据测试目标补齐请求体和基础断言", "生成后仍需由用户确认，再发送真实请求"}
	if len(documentEvidence) > 0 {
		reasoning = append(reasoning, "已检索当前项目文档："+strings.Join(documentEvidence, "、"))
	}
	writeJSON(w, 200, agentOutput{Summary: fmt.Sprintf("已根据“%s”生成一份可确认的 %s 请求草稿", input.Prompt, base.Method), Request: RequestSpec{Method: base.Method, URL: base.URL, Headers: base.Headers, Query: base.Query, Body: body}, Assertions: assertions, Reasoning: reasoning, Mode: "rule-agent"})
}
func (s *server) mustAPIs(ctx context.Context, projectID int) []Endpoint {
	items, _ := s.store.apis(ctx, projectID)
	return items
}

func (s *server) regression(w http.ResponseWriter, r *http.Request) {
	caseID, _ := strconv.Atoi(r.URL.Query().Get("caseId"))
	runs, err := s.store.runs(r.Context())
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	var related []RunRecord
	for _, item := range runs {
		if caseID == 0 || item.CaseID == caseID {
			related = append(related, item)
		}
	}
	if len(related) == 0 {
		writeJSON(w, 200, map[string]any{"hasBaseline": false, "message": "暂无同一测试用例的历史运行记录"})
		return
	}
	latest, baseline := related[0], related[len(related)-1]
	diff := []map[string]any{}
	if latest.Success != baseline.Success {
		diff = append(diff, map[string]any{"type": "success", "message": fmt.Sprintf("成功状态从 %t 变为 %t", baseline.Success, latest.Success)})
	}
	if latest.DurationMs > baseline.DurationMs+200 {
		diff = append(diff, map[string]any{"type": "latency", "message": fmt.Sprintf("耗时增加 %dms", latest.DurationMs-baseline.DurationMs)})
	}
	if latest.StatusCode != baseline.StatusCode {
		diff = append(diff, map[string]any{"type": "status", "message": fmt.Sprintf("状态码从 %d 变为 %d", baseline.StatusCode, latest.StatusCode)})
	}
	if responseChanges := responseDiff(baseline.ResponseBody, latest.ResponseBody); len(responseChanges) > 0 {
		diff = append(diff, map[string]any{"type": "response", "message": fmt.Sprintf("响应结构或内容发生 %d 处变化", len(responseChanges)), "changes": responseChanges})
	}
	writeJSON(w, 200, map[string]any{"hasBaseline": len(related) > 1, "baseline": baseline, "latest": latest, "diff": diff, "runCount": len(related)})
}

func responseDiff(baseline, latest string) []map[string]string {
	var left, right any
	if json.Unmarshal([]byte(baseline), &left) != nil || json.Unmarshal([]byte(latest), &right) != nil {
		if baseline != latest {
			return []map[string]string{{"path": "$", "type": "changed", "message": "响应文本发生变化"}}
		}
		return nil
	}
	changes := []map[string]string{}
	var compare func(any, any, string)
	compare = func(before, after any, path string) {
		if len(changes) >= 20 {
			return
		}
		beforeObject, beforeOK := before.(map[string]any)
		afterObject, afterOK := after.(map[string]any)
		if beforeOK && afterOK {
			seen := map[string]bool{}
			for key, oldValue := range beforeObject {
				seen[key] = true
				newValue, exists := afterObject[key]
				if !exists {
					changes = append(changes, map[string]string{"path": path + "." + key, "type": "removed", "message": "字段已移除：" + path + "." + key})
					continue
				}
				compare(oldValue, newValue, path+"."+key)
			}
			for key := range afterObject {
				if !seen[key] {
					changes = append(changes, map[string]string{"path": path + "." + key, "type": "added", "message": "新增字段：" + path + "." + key})
				}
			}
			return
		}
		beforeArray, beforeArrayOK := before.([]any)
		afterArray, afterArrayOK := after.([]any)
		if beforeArrayOK && afterArrayOK {
			if len(beforeArray) != len(afterArray) {
				changes = append(changes, map[string]string{"path": path, "type": "changed", "message": fmt.Sprintf("数组长度变化：%s（%d → %d）", path, len(beforeArray), len(afterArray))})
			}
			limit := len(beforeArray)
			if len(afterArray) < limit {
				limit = len(afterArray)
			}
			for index := 0; index < limit; index++ {
				compare(beforeArray[index], afterArray[index], fmt.Sprintf("%s[%d]", path, index))
			}
			return
		}
		if fmt.Sprint(before) != fmt.Sprint(after) {
			changes = append(changes, map[string]string{"path": path, "type": "changed", "message": "字段值变化：" + path})
		}
	}
	compare(left, right, "$")
	return changes
}

func (s *server) execute(ctx context.Context, input runInput) RunRecord {
	started := time.Now()
	reqURL := input.Request.URL
	if strings.HasPrefix(reqURL, "/") {
		reqURL = "http://localhost:" + firstNonEmpty(os.Getenv("PORT"), defaultPort) + reqURL
	}
	u, err := url.Parse(reqURL)
	if err != nil {
		return failedRun(started, 0, "配置错误", err.Error())
	}
	q := u.Query()
	for k, v := range input.Request.Query {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	var body io.Reader
	if input.Request.Body != "" {
		body = strings.NewReader(input.Request.Body)
	}
	req, err := http.NewRequestWithContext(ctx, firstNonEmpty(input.Request.Method, http.MethodGet), u.String(), body)
	if err != nil {
		return failedRun(started, 0, "配置错误", err.Error())
	}
	for k, v := range input.Request.Headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("Content-Type") == "" && input.Request.Body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := s.client.Do(req)
	firstByte := time.Since(started).Milliseconds()
	if err != nil {
		return RunRecord{StartedAt: started, DurationMs: time.Since(started).Milliseconds(), FirstByteMs: firstByte, Success: false, ErrorCategory: classifyNetworkError(err), ErrorMessage: err.Error(), ResponseHeader: map[string]string{}}
	}
	defer resp.Body.Close()
	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	bodyText := string(raw)
	durationMs := time.Since(started).Milliseconds()
	contentType := resp.Header.Get("Content-Type")
	outputTokens := approxTokens(bodyText)
	inputTokens := approxTokens(input.Request.Body)
	tokensPerSec := float64(0)
	if durationMs > 0 {
		tokensPerSec = float64(outputTokens) / (float64(durationMs) / 1000)
	}
	success := resp.StatusCode >= 200 && resp.StatusCode < 300 && readErr == nil
	category, message := "", ""
	if !success {
		category, message = classifyHTTP(resp.StatusCode, bodyText, readErr)
	}
	pass, total := evaluateAssertions(input.Assertions, resp.StatusCode, bodyText)
	return RunRecord{StartedAt: started, DurationMs: durationMs, FirstByteMs: firstByte, StatusCode: resp.StatusCode, Success: success && pass == total, ErrorCategory: category, ErrorMessage: message, ResponseHeader: headerMap(resp.Header), ResponseBody: bodyText, ContentType: contentType, Stream: strings.Contains(strings.ToLower(contentType), "text/event-stream"), InputTokens: inputTokens, OutputTokens: outputTokens, TokensPerSec: tokensPerSec, AssertionPass: pass, AssertionTotal: total}
}

func approxTokens(value string) int {
	text := strings.TrimSpace(value)
	if text == "" {
		return 0
	}
	count := len([]rune(text)) / 4
	if count < 1 {
		return 1
	}
	return count
}
func failedRun(started time.Time, status int, category, message string) RunRecord {
	return RunRecord{StartedAt: started, DurationMs: time.Since(started).Milliseconds(), StatusCode: status, Success: false, ErrorCategory: category, ErrorMessage: message, ResponseHeader: map[string]string{}}
}
func classifyNetworkError(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "超时"
	}
	return "网络错误"
}
func classifyHTTP(status int, body string, err error) (string, string) {
	if err != nil {
		return "响应读取失败", err.Error()
	}
	switch status {
	case 401, 403:
		return "鉴权失败", "服务拒绝了请求，请检查 Authorization、API Key 或权限范围"
	case 404:
		return "地址不存在", "请检查 URL、部署名称或路由前缀"
	case 408, 504:
		return "服务超时", "上游服务没有在限定时间内完成响应"
	case 429:
		return "请求过快/限流", "服务端触发了频率或配额限制"
	case 500, 502, 503:
		return "服务端错误", "上游服务发生异常，可结合响应体和服务日志排查"
	default:
		return "HTTP 错误", strings.TrimSpace(body)
	}
}
func headerMap(h http.Header) map[string]string {
	out := map[string]string{}
	for k, values := range h {
		if len(values) > 0 {
			out[k] = values[0]
		}
	}
	return out
}
func evaluateAssertions(assertions []Assertion, status int, body string) (int, int) {
	if len(assertions) == 0 {
		return 1, 1
	}
	pass := 0
	for _, a := range assertions {
		ok := false
		if a.Type == "status" {
			ok = strconv.Itoa(status) == a.Expected
		}
		if a.Type == "contains" {
			ok = strings.Contains(body, a.Expected)
		}
		if a.Type == "json_field" {
			var value any
			if json.Unmarshal([]byte(body), &value) == nil {
				ok = jsonPathExists(value, a.Path)
			}
		}
		if a.Type == "json_contract" || a.Type == "json_schema" {
			var value any
			if json.Unmarshal([]byte(body), &value) == nil {
				if a.Type == "json_schema" && strings.TrimSpace(a.Expected) != "" {
					var schema any
					if json.Unmarshal([]byte(a.Expected), &schema) == nil {
						ok = validateJSONSchema(value, schema)
					}
				} else {
					ok = true
					for _, required := range strings.Split(a.Path, ",") {
						if strings.TrimSpace(required) != "" && !jsonPathExists(value, strings.TrimSpace(required)) {
							ok = false
						}
					}
				}
			}
		}
		if ok {
			pass++
		}
	}
	return pass, len(assertions)
}
func jsonPathExists(value any, path string) bool {
	current := value
	for _, part := range strings.Split(strings.Trim(path, "."), ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return false
		}
		current, ok = object[part]
		if !ok {
			return false
		}
	}
	return true
}

func jsonPathValue(body, path string) (string, bool) {
	var value any
	if json.Unmarshal([]byte(body), &value) != nil {
		return "", false
	}
	current := value
	for _, part := range strings.Split(strings.Trim(path, "."), ".") {
		if strings.TrimSpace(part) == "" {
			continue
		}
		object, ok := current.(map[string]any)
		if !ok {
			return "", false
		}
		current, ok = object[part]
		if !ok {
			return "", false
		}
	}
	switch item := current.(type) {
	case string:
		return item, true
	case float64:
		return strconv.FormatFloat(item, 'f', -1, 64), true
	case bool:
		return strconv.FormatBool(item), true
	default:
		raw, err := json.Marshal(item)
		return string(raw), err == nil
	}
}

func validateJSONSchema(value, schema any) bool {
	definition, ok := schema.(map[string]any)
	if !ok {
		return false
	}
	if expected, ok := definition["type"].(string); ok && !jsonTypeMatches(value, expected) {
		return false
	}
	if enum, ok := definition["enum"].([]any); ok {
		matched := false
		for _, candidate := range enum {
			if fmt.Sprint(candidate) == fmt.Sprint(value) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if required, ok := definition["required"].([]any); ok {
		object, objectOK := value.(map[string]any)
		if !objectOK {
			return false
		}
		for _, item := range required {
			key, keyOK := item.(string)
			if !keyOK {
				continue
			}
			if _, exists := object[key]; !exists {
				return false
			}
		}
	}
	if properties, ok := definition["properties"].(map[string]any); ok {
		object, objectOK := value.(map[string]any)
		if !objectOK {
			return false
		}
		for key, childSchema := range properties {
			if childValue, exists := object[key]; exists && !validateJSONSchema(childValue, childSchema) {
				return false
			}
		}
	}
	if items, ok := definition["items"]; ok {
		array, arrayOK := value.([]any)
		if !arrayOK {
			return false
		}
		for _, child := range array {
			if !validateJSONSchema(child, items) {
				return false
			}
		}
	}
	if minimum, ok := definition["minimum"].(float64); ok {
		if number, numberOK := value.(float64); !numberOK || number < minimum {
			return false
		}
	}
	if minLength, ok := definition["minLength"].(float64); ok {
		if text, textOK := value.(string); !textOK || float64(len([]rune(text))) < minLength {
			return false
		}
	}
	return true
}

func jsonTypeMatches(value any, expected string) bool {
	switch expected {
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "string":
		_, ok := value.(string)
		return ok
	case "number", "integer":
		number, ok := value.(float64)
		return ok && (expected != "integer" || number == float64(int64(number)))
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "null":
		return value == nil
	default:
		return true
	}
}
func firstNonEmpty(items ...string) string {
	for _, item := range items {
		if strings.TrimSpace(item) != "" {
			return item
		}
	}
	return ""
}
func demoEcho(w http.ResponseWriter, r *http.Request) {
	var incoming any
	raw, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &incoming)
	}
	writeJSON(w, 200, map[string]any{"message": "echo ok", "method": r.Method, "received": incoming, "timestamp": time.Now().Format(time.RFC3339)})
}
func demoAuth(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer demo-token" {
		writeJSON(w, 401, map[string]any{"code": "UNAUTHORIZED", "message": "缺少或错误的 Bearer Token"})
		return
	}
	writeJSON(w, 200, map[string]any{"message": "auth ok", "userId": "u_demo_001"})
}
func demoLLM(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("stream") == "true" {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, _ := w.(http.Flusher)
		for _, piece := range []string{"回归测试是", "在接口或模型发生变化后", "重新运行原有测试并比较结果。"} {
			fmt.Fprintf(w, "data: %s\n\n", piece)
			if flusher != nil {
				flusher.Flush()
			}
			time.Sleep(80 * time.Millisecond)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
		return
	}
	writeJSON(w, 200, map[string]string{"answer": "这是一个模拟的大模型响应。"})
}
