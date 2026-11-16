package projecttest

import (
	"ai-api-test-platform-v1/backend/internal/security"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type TaskPlanner func(context.Context, Task, string) ([]TaskStep, error)
type TaskHandler struct {
	store   *TaskStore
	planner TaskPlanner
	client  *http.Client
	mu      sync.Mutex
	cancels map[int64]context.CancelFunc
	queue   chan struct{}
}

func NewTaskHandler(store *TaskStore, planner TaskPlanner) *TaskHandler {
	client := security.HTTPClient(45 * time.Second)
	return &TaskHandler{store: store, planner: planner, client: client, cancels: map[int64]context.CancelFunc{}, queue: make(chan struct{}, 2)}
}
func RegisterTaskRoutes(mux *http.ServeMux, h *TaskHandler) {
	mux.HandleFunc("/api/project-tests/tasks", h.Tasks)
	mux.HandleFunc("/api/project-tests/plan", h.Plan)
	mux.HandleFunc("/api/project-tests/runs", h.Runs)
	mux.HandleFunc("/api/project-tests/cancel", h.Cancel)
	mux.HandleFunc("/api/project-tests/export", h.Export)
}
func taskRead(r *http.Request, value any) error {
	return json.NewDecoder(io.LimitReader(r.Body, 8<<20)).Decode(value)
}
func taskError(w http.ResponseWriter, err error) {
	writeJSON(w, 400, map[string]string{"error": err.Error()})
}
func projectQuery(r *http.Request) int {
	v, _ := strconv.Atoi(r.URL.Query().Get("projectId"))
	return v
}
func (h *TaskHandler) Tasks(w http.ResponseWriter, r *http.Request) {
	project := projectQuery(r)
	if r.Method == "GET" {
		if project <= 0 {
			taskError(w, fmt.Errorf("请选择项目"))
			return
		}
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if id > 0 {
			task, err := h.store.Task(r.Context(), project, id)
			if err != nil {
				taskError(w, err)
				return
			}
			writeJSON(w, 200, PublicTask(task))
			return
		}
		tasks, err := h.store.Tasks(r.Context(), project)
		if err != nil {
			taskError(w, err)
			return
		}
		writeJSON(w, 200, tasks)
		return
	}
	if r.Method != "POST" && r.Method != "PUT" {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var task Task
	if err := taskRead(r, &task); err != nil {
		taskError(w, err)
		return
	}
	if err := ValidateTask(task); err != nil {
		taskError(w, err)
		return
	}
	if task.FileData != "" {
		data, err := base64.StdEncoding.DecodeString(task.FileData)
		if err != nil || len(data) > 5<<20 {
			taskError(w, fmt.Errorf("请选择不超过 5 MB 的测试文件"))
			return
		}
		task.FileSize = len(data)
	}
	task, err := h.store.SaveTask(r.Context(), task)
	if err != nil {
		taskError(w, err)
		return
	}
	writeJSON(w, 200, task)
}
func (h *TaskHandler) Plan(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var task Task
	if err := taskRead(r, &task); err != nil {
		taskError(w, err)
		return
	}
	if err := ValidateTask(task); err != nil {
		taskError(w, err)
		return
	}
	var spec string
	if task.OpenAPIURL != "" {
		target, err := scopedURL(task.BaseURL, task.OpenAPIURL)
		if err != nil {
			taskError(w, err)
			return
		}
		req, err := http.NewRequestWithContext(r.Context(), "GET", target, nil)
		if err != nil {
			taskError(w, err)
			return
		}
		resp, err := h.client.Do(req)
		if err != nil {
			taskError(w, fmt.Errorf("接口文档读取失败：%w", err))
			return
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		if readErr != nil || resp.StatusCode != 200 {
			taskError(w, fmt.Errorf("接口文档未成功返回，请检查项目启动地址"))
			return
		}
		var parsed map[string]any
		if json.Unmarshal(data, &parsed) != nil || parsed["paths"] == nil {
			taskError(w, fmt.Errorf("文档不是有效 OpenAPI JSON"))
			return
		}
		spec = string(data)
	}
	var steps []TaskStep
	var err error
	if task.Template == "cloud_disk" {
		steps = CloudDiskPlan(task)
		if spec != "" {
			var doc struct {
				Paths map[string]map[string]any `json:"paths"`
			}
			_ = json.Unmarshal([]byte(spec), &doc)
			for _, step := range steps {
				path := variablePattern.ReplaceAllStringFunc(step.URL, func(value string) string {
					return "{" + strings.TrimSuffix(strings.TrimPrefix(value, "{{"), "}}") + "}"
				})
				if doc.Paths[path] == nil || doc.Paths[path][strings.ToLower(step.Method)] == nil {
					taskError(w, fmt.Errorf("Cloud Disk 模板需要 %s %s，但文档未包含；可改用 AI 生成流程", step.Method, path))
					return
				}
			}
		}
	} else {
		if h.planner == nil {
			err = fmt.Errorf("请先配置真实助手模型")
		} else {
			steps, err = h.planner(r.Context(), task, spec)
		}
	}
	if err == nil {
		err = ValidatePlan(steps, task.BaseURL)
	}
	if err != nil {
		var missing *MissingTaskInformation
		if errors.As(err, &missing) {
			writeJSON(w, 200, map[string]any{"steps": []TaskStep{}, "missingInformation": missing.Fields, "message": "补充信息后由助手继续生成流程"})
			return
		}
		taskError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"steps": steps, "source": task.Template, "message": "流程草稿已生成，执行前可核对请求、依赖与业务检查"})
}
func (h *TaskHandler) Runs(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		project := projectQuery(r)
		if project <= 0 {
			taskError(w, fmt.Errorf("请选择项目"))
			return
		}
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if id > 0 {
			run, _, err := h.store.Run(r.Context(), project, id)
			if err != nil {
				taskError(w, err)
				return
			}
			writeJSON(w, 200, run)
			return
		}
		runs, err := h.store.Runs(r.Context(), project)
		if err != nil {
			taskError(w, err)
			return
		}
		writeJSON(w, 200, runs)
		return
	}
	if r.Method != "POST" {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var input struct {
		ProjectID   int   `json:"projectId"`
		TaskID      int64 `json:"taskId"`
		Confirmed   bool  `json:"confirmed"`
		SourceRunID int64 `json:"sourceRunId"`
	}
	if err := taskRead(r, &input); err != nil {
		taskError(w, err)
		return
	}
	task, err := h.store.Task(r.Context(), input.ProjectID, input.TaskID)
	if err != nil {
		taskError(w, err)
		return
	}
	if task.Permission == "review" && !input.Confirmed {
		taskError(w, fmt.Errorf("请确认本次账号、文件和整套操作范围"))
		return
	}
	var private RunPrivate
	var file []byte
	var previous []StepResult
	if input.SourceRunID > 0 {
		var source TaskRun
		source, private, err = h.store.Run(r.Context(), input.ProjectID, input.SourceRunID)
		if err == nil && (source.TaskID != task.ID || source.Status == "running" || source.Status == "queued") {
			err = fmt.Errorf("只能重跑当前任务已结束的运行记录")
		}
		if err == nil {
			previous = source.Results
			task = source.Task
			file, err = base64.StdEncoding.DecodeString(private.FileData)
		}
	} else {
		private, file, err = NewVariables(task)
		private.FileData = base64.StdEncoding.EncodeToString(file)
	}
	if err != nil {
		taskError(w, err)
		return
	}
	if err = ValidatePlan(task.Steps, task.BaseURL); err != nil {
		taskError(w, err)
		return
	}
	if private.Variables == nil || private.Owned == nil {
		taskError(w, fmt.Errorf("原运行上下文不可用，请重新运行整个任务"))
		return
	}
	run, err := h.store.NewRun(r.Context(), TaskRun{ProjectID: task.ProjectID, TaskID: task.ID, SourceRunID: input.SourceRunID, Task: PublicTask(task), Status: "queued", Results: []StepResult{}, Variables: map[string]string{}})
	if err != nil {
		taskError(w, err)
		return
	}
	SummarizeRun(&run)
	if err = h.store.UpdateRun(r.Context(), run, private); err != nil {
		taskError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	h.mu.Lock()
	h.cancels[run.ID] = cancel
	h.mu.Unlock()
	go h.execute(ctx, cancel, run, task, private, file, previous)
	writeJSON(w, 202, run)
}
func (h *TaskHandler) execute(ctx context.Context, cancel context.CancelFunc, run TaskRun, task Task, private RunPrivate, file []byte, previous []StepResult) {
	defer cancel()
	defer func() { h.mu.Lock(); delete(h.cancels, run.ID); h.mu.Unlock() }()
	acquired := false
	select {
	case h.queue <- struct{}{}:
		acquired = true
	case <-ctx.Done():
	}
	if acquired {
		defer func() { <-h.queue }()
		run.Status = "running"
	}
	persist := func() error {
		SummarizeRun(&run)
		run.Variables = redactMap(private.Variables, private.Variables)
		return h.store.UpdateRun(context.Background(), run, private)
	}
	err := ctx.Err()
	if err == nil {
		err = persist()
	}
	if err == nil {
		err = RunTaskPlan(ctx, h.client, task, &private, file, previous, func(result StepResult) error {
			run.CurrentStep = result.Name
			if result.Status == "running" {
				return persist()
			}
			run.Results = append(run.Results, result)
			return persist()
		})
	}
	now := time.Now()
	run.FinishedAt = &now
	run.CurrentStep = ""
	run.Status = "completed"
	if err != nil {
		run.Error = redactText(err.Error(), private.Variables)
		run.Status = "failed"
		if errors.Is(err, context.Canceled) {
			run.Status = "cancelled"
		}
	}
	_ = persist()
}
func (h *TaskHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	var input struct {
		ProjectID int   `json:"projectId"`
		ID        int64 `json:"id"`
	}
	if err := taskRead(r, &input); err != nil {
		taskError(w, err)
		return
	}
	if _, _, err := h.store.Run(r.Context(), input.ProjectID, input.ID); err != nil {
		taskError(w, err)
		return
	}
	h.mu.Lock()
	cancel := h.cancels[input.ID]
	h.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	writeJSON(w, 200, map[string]bool{"cancelled": cancel != nil})
}
func (h *TaskHandler) Export(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	run, _, err := h.store.Run(r.Context(), projectQuery(r), id)
	if err != nil {
		taskError(w, err)
		return
	}
	if r.URL.Query().Get("format") == "json" {
		writeJSON(w, 200, run)
		return
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s — 项目测试报告\n\n- 项目地址：%s\n- 执行状态：%s\n- 检查：%d；通过 %d；失败 %d；阻塞 %d；未执行 %d\n\n", run.Task.Name, run.Task.BaseURL, run.Status, run.Summary.Total, run.Summary.Passed, run.Summary.Failed, run.Summary.Blocked, run.Summary.NotRun)
	for _, step := range run.Results {
		fmt.Fprintf(&b, "## %s · %s\n\n", step.Name, step.Status)
		for _, check := range step.Checks {
			fmt.Fprintf(&b, "- %s %s：预期 %q；实际 %q；通过=%t\n", check.Type, check.Path, check.Expected, check.Actual, check.Passed)
		}
		if step.Error != "" {
			fmt.Fprintf(&b, "- 原因：%s\n", step.Error)
		}
		fmt.Fprintf(&b, "\n请求：%s %s\n\n响应：\n\n```\n%s\n```\n\n", step.Method, step.URL, step.ResponseBody)
	}
	fmt.Fprintln(&b, "> 结论仅适用于本次覆盖的接口与业务检查；页面交互、未覆盖功能和专项性能/安全测试不在报告范围内。")
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	_, _ = io.WriteString(w, b.String())
}
