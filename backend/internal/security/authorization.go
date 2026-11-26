package security

import (
	"ai-api-test-platform-v1/backend/internal/reportdoc"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type ResourceScope struct {
	ProjectID int
	Auto      bool
	Enabled   bool
}
type ScopeResolver interface {
	Resolve(context.Context, string, int64) (ResourceScope, error)
}

func (a *Access) SetResolver(resolver ScopeResolver) { a.resolver = resolver }

type permissionError struct {
	code    int
	message string
}

func (e *permissionError) Error() string { return e.message }
func forbidden(message string) error     { return &permissionError{403, message} }
func invalid(message string) error       { return &permissionError{400, message} }

// Routes describe platform capabilities. Unknown routes fail closed for
// non-administrators; project scope always comes from trusted stored resources.
func routeCapability(path, method string) string {
	if path == "/api/projects" {
		if method == "GET" {
			return "projects"
		}
		return "admin"
	}
	if path == "/api/models" && method == "GET" {
		return "global-read"
	}
	if path == "/api/settings/llm" && method == "GET" {
		return "global-read"
	}
	if strings.HasPrefix(path, "/api/models") || strings.HasPrefix(path, "/api/settings/") {
		return "admin"
	}
	switch path {
	case "/api/apis", "/api/cases", "/api/documents", "/api/evaluation-suites", "/api/project-tests/tasks":
		if method == "GET" {
			return "read"
		}
		return "edit"
	case "/api/knowledge/history":
		if method == "GET" {
			return "read"
		}
		return "deny"
	case "/api/runs", "/api/project-tests/runs", "/api/evaluation-suites/runs":
		if method == "GET" {
			return "read"
		}
		return "execute"
	case "/api/documents/search", "/api/project-check-reports", "/api/evaluations", "/api/regression", "/api/project-tests/export", "/api/evaluation-suites/export":
		if method == "GET" {
			return "read"
		}
		return "deny"
	case "/api/apis/import", "/api/agent/audit", "/api/evaluation-suites/review":
		return "edit"
	case "/api/runs/batch", "/api/runs/chain", "/api/project-check", "/api/agent/draft", "/api/agent/plan", "/api/agent/business-cases", "/api/evaluations/run", "/api/project-tests/plan", "/api/project-tests/cancel", "/api/evaluation-suites/cancel":
		return "execute"
	case "/api/agent/document":
		return "upload"
	case "/api/demo/echo", "/api/demo/auth", "/api/demo/llm":
		return "execute-local"
	default:
		return "deny"
	}
}
func number(value any) (int64, error) {
	if value == nil {
		return 0, nil
	}
	var id int64
	var err error
	switch v := value.(type) {
	case json.Number:
		id, err = v.Int64()
	case string:
		if v == "" {
			return 0, nil
		}
		id, err = strconv.ParseInt(v, 10, 64)
	case float64:
		id = int64(v)
		if float64(id) != v {
			err = fmt.Errorf("not integer")
		}
	default:
		err = fmt.Errorf("not integer")
	}
	if err != nil || id < 0 {
		return 0, invalid("资源编号必须是非负整数")
	}
	return id, nil
}
func readPayload(r *http.Request) (map[string]any, error) {
	if r.Body == nil || r.Method == "GET" || r.Method == "HEAD" {
		return map[string]any{}, nil
	}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		r.Body = http.MaxBytesReader(nil, r.Body, 16<<20)
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			return nil, invalid("上传内容过大或格式不正确")
		}
		return map[string]any{"projectId": r.FormValue("projectId")}, nil
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, (16<<20)+1))
	if err != nil || len(data) > 16<<20 {
		return nil, invalid("请求内容超过16MB")
	}
	r.Body = io.NopCloser(bytes.NewReader(data))
	if len(data) == 0 {
		return map[string]any{}, nil
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	payload := map[string]any{}
	if d.Decode(&payload) != nil || payload == nil || d.Decode(new(any)) != io.EOF {
		return nil, invalid("请求必须是单个JSON对象")
	}
	if err := canonicalFields(payload); err != nil {
		return nil, err
	}
	return payload, nil
}

// encoding/json matches struct fields case-insensitively. Apply that same
// interpretation before authorization, rejecting ambiguous duplicate aliases.
func canonicalFields(payload map[string]any) error {
	for _, canonical := range []string{"projectId", "apiId", "caseId", "taskId", "sourceRunId", "id", "profileId", "profileIds", "documentIds", "selectedDocumentIds", "selectedProfileIds", "judgeProfileId", "permission", "confirmed", "items", "steps"} {
		keys := []string{}
		for key := range payload {
			if strings.EqualFold(key, canonical) {
				keys = append(keys, key)
			}
		}
		if len(keys) > 1 {
			return invalid("权限相关字段重复，请使用唯一字段名")
		}
		if len(keys) == 1 && keys[0] != canonical {
			payload[canonical] = payload[keys[0]]
			delete(payload, keys[0])
		}
	}
	return nil
}
func restorePayload(r *http.Request, payload map[string]any) {
	if r.Method == "GET" || r.Method == "HEAD" || strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		return
	}
	data, _ := json.Marshal(payload)
	r.Body = io.NopCloser(bytes.NewReader(data))
	r.ContentLength = int64(len(data))
}
func (a *Access) authorize(r *http.Request, user User) (int, error) {
	capability := routeCapability(r.URL.Path, r.Method)
	if user.Role != RoleAdmin {
		if capability == "admin" || capability == "deny" {
			return 0, forbidden("当前账号无此操作权限")
		}
		if !user.CanExecute() && (capability != "read" && capability != "projects" && capability != "global-read") {
			return 0, forbidden("只读账号只能查看记录和导出脱敏报告，不能执行或修改")
		}
	}
	if capability == "projects" || capability == "global-read" || capability == "admin" || capability == "deny" {
		return 0, nil
	}
	payload, err := readPayload(r)
	if err != nil {
		return 0, err
	}
	defer restorePayload(r, payload)
	q := r.URL.Query()
	project, err := number(payload["projectId"])
	if err != nil {
		return 0, err
	}
	queryProject, err := number(q.Get("projectId"))
	if err != nil {
		return 0, err
	}
	if project > 0 && queryProject > 0 && project != queryProject {
		return 0, invalid("请求中的项目编号不一致")
	}
	if project == 0 {
		project = queryProject
	}
	if project > 0 && !user.CanAccess(int(project)) {
		return 0, forbidden("无权访问这个项目，请联系管理员分配项目")
	}
	var storedAuto bool
	resolve := func(kind string, value any) error {
		id, err := number(value)
		if err != nil {
			return err
		}
		if id == 0 {
			return nil
		}
		if a.resolver == nil {
			if user.Role == RoleAdmin {
				return nil
			}
			return forbidden("项目权限服务未配置")
		}
		scope, err := a.resolver.Resolve(r.Context(), kind, id)
		if err != nil {
			return forbidden("资源不存在或无权访问")
		}
		if kind == "model" {
			if !scope.Enabled {
				return forbidden("该公共模型已停用，请选择启用模型")
			}
			return nil
		}
		if !user.CanAccess(scope.ProjectID) {
			return forbidden("资源不存在或无权访问")
		}
		if project > 0 && int(project) != scope.ProjectID {
			return forbidden("资源不属于当前项目")
		}
		project = int64(scope.ProjectID)
		storedAuto = storedAuto || scope.Auto
		return nil
	}
	// Resolve direct identifiers used by handlers, including run/case overrides.
	for field, kind := range map[string]string{"apiId": "api", "caseId": "case", "taskId": "task", "sourceRunId": "project_run"} {
		if field == "sourceRunId" && !strings.HasPrefix(r.URL.Path, "/api/project-tests") {
			continue
		}
		if err = resolve(kind, payload[field]); err != nil {
			return 0, err
		}
		if err = resolve(kind, q.Get(field)); err != nil {
			return 0, err
		}
	}
	idKind := map[string]string{"/api/apis": "api", "/api/cases": "case", "/api/documents": "document", "/api/project-tests/tasks": "task", "/api/project-tests/runs": "project_run", "/api/project-tests/export": "project_run", "/api/project-tests/cancel": "project_run", "/api/evaluation-suites": "evaluation_suite", "/api/evaluation-suites/runs": "evaluation_run", "/api/evaluation-suites/export": "evaluation_run", "/api/evaluation-suites/cancel": "evaluation_run", "/api/evaluation-suites/review": "evaluation_run"}[r.URL.Path]
	if idKind != "" {
		if err = resolve(idKind, payload["id"]); err != nil {
			return 0, err
		}
		if err = resolve(idKind, q.Get("id")); err != nil {
			return 0, err
		}
	}
	for _, field := range []string{"documentIds", "selectedDocumentIds"} {
		if values, ok := payload[field].([]any); ok {
			for _, v := range values {
				if err = resolve("document", v); err != nil {
					return 0, err
				}
			}
		}
	}
	for _, field := range []string{"profileIds", "selectedProfileIds"} {
		if values, ok := payload[field].([]any); ok {
			for _, v := range values {
				if err = resolve("model", v); err != nil {
					return 0, err
				}
			}
		}
	}
	if err = resolve("model", payload["judgeProfileId"]); err != nil {
		return 0, err
	}
	if err = resolve("model", payload["profileId"]); err != nil {
		return 0, err
	}
	if steps, ok := payload["steps"].([]any); ok && r.URL.Path == "/api/runs/chain" {
		for _, raw := range steps {
			step, ok := raw.(map[string]any)
			if !ok {
				return 0, invalid("链路步骤格式不正确")
			}
			if err = canonicalFields(step); err != nil {
				return 0, err
			}
			if err = resolve("api", step["apiId"]); err != nil {
				return 0, err
			}
		}
	}
	if r.URL.Path == "/api/runs/batch" {
		items, ok := payload["items"].([]any)
		if !ok || len(items) == 0 {
			return 0, invalid("请提供测试项")
		}
		for _, raw := range items {
			item, ok := raw.(map[string]any)
			if !ok {
				return 0, invalid("测试项格式不正确")
			}
			data, _ := json.Marshal(item)
			child := r.Clone(r.Context())
			childURL := *r.URL
			child.URL = &childURL
			child.URL.Path = "/api/runs"
			child.Body = io.NopCloser(bytes.NewReader(data))
			child.ContentLength = int64(len(data))
			child.Header = child.Header.Clone()
			child.Header.Set("Content-Type", "application/json")
			p, err := a.authorize(child, user)
			if err != nil {
				return 0, err
			}
			item["projectId"] = p
		}
		return int(project), nil
	}
	if !user.CanAuto() {
		if payload["permission"] == "auto" || payload["permission"] == "project_full" || r.Header.Get("X-Assistant-Mode") == "auto" {
			return 0, forbidden("管理员尚未授权助手自动执行，请使用先审核模式")
		}
		if storedAuto && r.URL.Path == "/api/project-tests/runs" && r.Method == "POST" && payload["confirmed"] != true {
			return 0, forbidden("此任务需手动确认；当前账号没有自动执行权限")
		}
	}
	if capability == "execute-local" || capability == "upload" {
		return int(project), nil
	}
	if project <= 0 && user.Role != RoleAdmin {
		return 0, invalid("请选择已获授权的项目")
	}
	if project > 0 {
		payload["projectId"] = project
		q.Set("projectId", strconv.FormatInt(project, 10))
		r.URL.RawQuery = q.Encode()
	}
	return int(project), nil
}

type bufferedResponse struct {
	header   http.Header
	status   int
	body     bytes.Buffer
	overflow bool
}

func newBufferedResponse() *bufferedResponse    { return &bufferedResponse{header: make(http.Header)} }
func (b *bufferedResponse) Header() http.Header { return b.header }
func (b *bufferedResponse) WriteHeader(status int) {
	if b.status == 0 {
		b.status = status
	}
}
func (b *bufferedResponse) Write(data []byte) (int, error) {
	if b.status == 0 {
		b.status = 200
	}
	if b.body.Len()+len(data) > 32<<20 {
		b.overflow = true
		return 0, fmt.Errorf("response too large")
	}
	return b.body.Write(data)
}

func (a *Access) serveAuthorized(next http.Handler, w http.ResponseWriter, r *http.Request, user User) {
	project, err := a.authorize(r, user)
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	if err != nil {
		status := 403
		if e, ok := err.(*permissionError); ok {
			status = e.code
		}
		if a.users != nil {
			_ = a.users.Record(r.Context(), user, r.URL.Path, r.Method, project, 0, status)
		}
		authJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	var auditID int64
	if a.users != nil && r.Method != "GET" && r.Method != "HEAD" {
		auditID, err = a.users.BeginOperation(r.Context(), user, r.URL.Path, r.Method, project)
		if err != nil {
			authJSON(w, 503, map[string]string{"error": "操作日志服务暂时不可用，本次操作未执行"})
			return
		}
	}
	capability := routeCapability(r.URL.Path, r.Method)
	protectedMetadata := user.Role != RoleAdmin && (capability == "projects" || capability == "global-read")
	if user.Role != RoleViewer && !protectedMetadata {
		tracked := &trackingResponse{ResponseWriter: w, access: a, request: r, user: user, project: project}
		next.ServeHTTP(tracked, r)
		if auditID > 0 {
			status := tracked.status
			if status == 0 {
				status = 200
			}
			_ = a.users.FinishOperation(r.Context(), auditID, tracked.resource, status)
		}
		return
	}
	// Exports for viewers use the same sanitized data as on-screen results.
	docx := user.Role == RoleViewer && strings.HasSuffix(r.URL.Path, "/export") && r.URL.Query().Get("format") == "docx"
	markdown := user.Role == RoleViewer && strings.HasSuffix(r.URL.Path, "/export") && r.URL.Query().Get("format") != "json"
	if markdown {
		q := r.URL.Query()
		q.Set("format", "json")
		r.URL.RawQuery = q.Encode()
	}
	b := newBufferedResponse()
	next.ServeHTTP(b, r)
	if b.status == 0 {
		b.status = 200
	}
	if b.overflow {
		authJSON(w, 503, map[string]string{"error": "报告过大，请缩小查询范围"})
		return
	}
	data := b.body.Bytes()
	var value any
	if json.Unmarshal(data, &value) == nil {
		if r.URL.Path == "/api/projects" && r.Method == "GET" && user.Role != RoleAdmin {
			filtered := []any{}
			if items, ok := value.([]any); ok {
				for _, raw := range items {
					if item, ok := raw.(map[string]any); ok {
						id, _ := number(item["id"])
						if user.CanAccess(int(id)) {
							filtered = append(filtered, item)
						}
					}
				}
			}
			value = filtered
		}
		if a.users != nil && r.Method == "GET" {
			value = a.users.AddActors(r.Context(), r.URL.Path, project, value)
		}
		if user.Role == RoleViewer {
			value = Sanitize(value, true)
		} else if user.Role != RoleAdmin && routeCapability(r.URL.Path, r.Method) == "global-read" {
			value = Sanitize(value, false)
		}
		data, _ = json.Marshal(value)
	}
	if a.users != nil && r.Method != "GET" && r.Method != "HEAD" {
		resource := int64(0)
		if object, ok := value.(map[string]any); ok {
			resource, _ = number(object["id"])
		}
		if err := a.users.FinishOperation(r.Context(), auditID, resource, b.status); err != nil {
			// The action may already have completed; never falsely report a retryable
			// failure that would cause duplicate writes. Server logs flag audit loss.
			fmt.Printf("operation audit persistence failed (actor=%d, path=%s)\n", user.ID, r.URL.Path)
		}
		if resource > 0 && b.status < 300 && isRunPath(r.URL.Path) {
			if object, ok := value.(map[string]any); ok {
				object["createdBy"] = user.Username
				data, _ = json.Marshal(value)
			}
		}
	}
	for key, values := range b.header {
		if key == "Content-Length" {
			continue
		}
		w.Header()[key] = values
	}
	if docx && b.status < 300 {
		var err error
		if strings.HasPrefix(r.URL.Path, "/api/evaluation-suites/") {
			data, err = reportdoc.Evaluation(data)
		} else {
			data, err = reportdoc.Project(data)
		}
		if err != nil {
			authJSON(w, 500, map[string]string{"error": "报告生成失败"})
			return
		}
		reportdoc.Download(w, "readonly-report", data)
		return
	}
	if markdown && b.status < 300 {
		pretty, _ := json.MarshalIndent(value, "", "  ")
		data = []byte("# 项目报告（只读脱敏版）\n\n原始请求、响应和凭据不向只读账号展示。\n\n```json\n" + string(pretty) + "\n```\n")
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	}
	w.WriteHeader(b.status)
	_, _ = w.Write(data)
}

// Executing users keep the original streaming/large-response behavior. Only
// small JSON records are enriched; we do not buffer image-heavy history pages.
type trackingResponse struct {
	http.ResponseWriter
	access   *Access
	request  *http.Request
	user     User
	project  int
	status   int
	resource int64
}

func (t *trackingResponse) WriteHeader(status int) {
	if t.status == 0 {
		t.status = status
		t.ResponseWriter.WriteHeader(status)
	}
}
func (t *trackingResponse) Write(data []byte) (int, error) {
	if t.status == 0 {
		t.status = 200
	}
	var value any
	if len(data) < 16<<20 && strings.Contains(t.Header().Get("Content-Type"), "application/json") && json.Unmarshal(data, &value) == nil {
		if object, ok := value.(map[string]any); ok {
			t.resource, _ = number(object["id"])
		}
		if t.access.users != nil && t.request.Method == "GET" {
			value = t.access.users.AddActors(t.request.Context(), t.request.URL.Path, t.project, value)
		}
		if t.request.Method == "POST" && isRunPath(t.request.URL.Path) && t.status < 300 {
			if object, ok := value.(map[string]any); ok {
				object["createdBy"] = t.user.Username
			}
		}
		if t.user.Role != RoleAdmin && routeCapability(t.request.URL.Path, t.request.Method) == "global-read" {
			value = Sanitize(value, false)
		}
		data, _ = json.Marshal(value)
		t.Header().Del("Content-Length")
	}
	return t.ResponseWriter.Write(data)
}
func (t *trackingResponse) Flush() {
	if t.status == 0 {
		t.status = 200
	}
	if f, ok := t.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
