package projecttest

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var variablePattern = regexp.MustCompile(`\{\{([a-zA-Z0-9_]+)\}\}`)

func interpolate(value string, variables map[string]string) (string, error) {
	var missing string
	result := variablePattern.ReplaceAllStringFunc(value, func(match string) string {
		key := variablePattern.FindStringSubmatch(match)[1]
		v, ok := variables[key]
		if !ok {
			missing = key
		}
		return v
	})
	if missing != "" {
		return "", fmt.Errorf("缺少变量 %s，请检查前置步骤或补充信息", missing)
	}
	return result, nil
}
func bodyInterpolate(value string, variables map[string]string) (string, error) {
	var missing string
	value = variablePattern.ReplaceAllStringFunc(value, func(match string) string {
		k := variablePattern.FindStringSubmatch(match)[1]
		v, ok := variables[k]
		if !ok {
			missing = k
		}
		quoted, _ := json.Marshal(v)
		return string(quoted[1 : len(quoted)-1])
	})
	if missing != "" {
		return "", fmt.Errorf("缺少请求体变量 %s", missing)
	}
	return value, nil
}
func jsonValue(value any, field string) (any, bool) {
	field = strings.TrimPrefix(field, "$.")
	field = strings.TrimPrefix(field, "$")
	if field == "" {
		return value, true
	}
	field = strings.ReplaceAll(strings.ReplaceAll(field, "[", "."), "]", "")
	for _, part := range strings.Split(field, ".") {
		if part == "" {
			continue
		}
		switch item := value.(type) {
		case map[string]any:
			var ok bool
			value, ok = item[part]
			if !ok {
				return nil, false
			}
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(item) {
				return nil, false
			}
			value = item[index]
		default:
			return nil, false
		}
	}
	return value, true
}
func scalar(value any) string {
	if value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	raw, _ := json.Marshal(value)
	return string(raw)
}
func secretKey(key string) bool {
	return regexp.MustCompile(`(?i)password|passwd|token|authorization|api.?key|secret|cookie`).MatchString(key)
}
func redactValue(value any) any {
	switch item := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, v := range item {
			if secretKey(k) {
				out[k] = "••••••"
			} else {
				out[k] = redactValue(v)
			}
		}
		return out
	case []any:
		out := []any{}
		for _, v := range item {
			out = append(out, redactValue(v))
		}
		return out
	default:
		return value
	}
}
func redactText(value string, variables map[string]string) string {
	var parsed any
	if json.Unmarshal([]byte(value), &parsed) == nil {
		raw, _ := json.MarshalIndent(redactValue(parsed), "", "  ")
		value = string(raw)
	}
	for key, secret := range variables {
		if secretKey(key) && secret != "" {
			value = strings.ReplaceAll(value, secret, "••••••")
		}
	}
	return value
}
func redactMap(values map[string]string, variables map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range values {
		if secretKey(k) {
			out[k] = "••••••"
		} else {
			out[k] = redactText(v, variables)
		}
	}
	return out
}
func scopedURL(base, relative string) (string, error) {
	b, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	p, err := url.Parse(relative)
	if err != nil {
		return "", err
	}
	if p.IsAbs() {
		if !sameCheckTarget(b, p) {
			return "", fmt.Errorf("步骤地址不属于本任务目标项目")
		}
		p.Host = b.Host
		p.Scheme = b.Scheme
		return p.String(), nil
	}
	if strings.HasPrefix(relative, "//") {
		return "", fmt.Errorf("不允许外部地址")
	}
	return b.ResolveReference(p).String(), nil
}
func destructive(step TaskStep) bool {
	label := strings.ToLower(step.URL)
	return step.Method == "DELETE" || strings.Contains(label, "delete") || strings.Contains(label, "remove") || strings.Contains(label, "rename") || strings.Contains(label, "restore") || strings.Contains(label, "/move")
}
func bindsGuard(step TaskStep) bool {
	ref := "{{" + step.Guard + "}}"
	if strings.Contains(step.URL, ref) || strings.Contains(step.Body, ref) {
		return true
	}
	for _, value := range step.Query {
		if value == ref {
			return true
		}
	}
	return false
}
func ValidatePlan(steps []TaskStep, base string) error {
	if len(steps) == 0 {
		return fmt.Errorf("请先生成测试流程")
	}
	if len(steps) > 80 {
		return fmt.Errorf("最多支持 80 个步骤")
	}
	seen := map[string]bool{}
	for _, step := range steps {
		if step.ID == "" || seen[step.ID] {
			return fmt.Errorf("步骤 ID 为空或重复")
		}
		for _, dep := range step.DependsOn {
			if !seen[dep] {
				return fmt.Errorf("步骤 %s 的依赖 %s 必须位于前面", step.ID, dep)
			}
		}
		seen[step.ID] = true
		if len(step.Assertions) == 0 {
			return fmt.Errorf("步骤 %s 缺少业务检查标准", step.Name)
		}
		if step.Method != "GET" && step.Method != "POST" && step.Method != "PUT" && step.Method != "PATCH" && step.Method != "DELETE" && step.Method != "HEAD" && step.Method != "OPTIONS" {
			return fmt.Errorf("不支持的请求方式")
		}
		placeholder := variablePattern.ReplaceAllString(step.URL, "placeholder")
		if _, err := scopedURL(base, placeholder); err != nil {
			return err
		}
		if strings.Contains(strings.ToLower(step.URL), "permanentdelete") || strings.HasSuffix(strings.ToLower(step.URL), "/clear") {
			return fmt.Errorf("整库清空与永久删除不在自动验收范围内")
		}
		if destructive(step) && step.Guard == "" {
			return fmt.Errorf("步骤 %s 必须指定本次资源 Guard，避免操作其他数据", step.Name)
		}
		if step.Guard != "" && !bindsGuard(step) {
			return fmt.Errorf("步骤 %s 的请求必须引用 Guard 资源变量", step.Name)
		}
		for _, a := range step.Assertions {
			switch a.Type {
			case "status", "json_equals", "json_nonempty", "json_contains", "array_contains", "array_absent", "binary_sha256", "auth_rejected":
			default:
				return fmt.Errorf("不支持断言 %s", a.Type)
			}
		}
	}
	return nil
}
func NewVariables(task Task) (RunPrivate, []byte, error) {
	random := make([]byte, 6)
	if _, err := rand.Read(random); err != nil {
		return RunPrivate{}, nil, err
	}
	suffix := hex.EncodeToString(random)
	username, password := task.Username, task.Password
	if task.AccountMode == "auto" {
		username = "pulse_" + suffix
		password = "P!" + suffix + "9a"
	}
	if username == "" || password == "" {
		return RunPrivate{}, nil, fmt.Errorf("请补充测试账号和密码")
	}
	var content []byte
	filename := task.FileName
	if task.FileMode == "manual" {
		var err error
		content, err = base64.StdEncoding.DecodeString(task.FileData)
		if err != nil || len(content) == 0 {
			return RunPrivate{}, nil, fmt.Errorf("请选择有效测试文件")
		}
	} else {
		filename = "pulse-" + suffix + ".txt"
		content = bytes.Repeat([]byte("Pulse acceptance test.\n"), 100)
		content = content[:2048]
	}
	if len(content) > 5<<20 {
		return RunPrivate{}, nil, fmt.Errorf("测试样本不能超过 5 MB")
	}
	hash := sha256.Sum256(content)
	variables := map[string]string{"username": username, "password": password, "runId": suffix, "folderName": "pulse_" + suffix, "fileName": path.Base(filename), "renamedFile": "pulse_" + suffix + "_renamed.txt", "fileSize": strconv.Itoa(len(content)), "fileHash": hex.EncodeToString(hash[:])}
	for name, value := range task.InitialVariables {
		if _, protected := variables[name]; protected {
			return RunPrivate{}, nil, fmt.Errorf("变量 %s 由账号与文件配置生成，不能覆盖", name)
		}
		if !regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*$`).MatchString(name) {
			return RunPrivate{}, nil, fmt.Errorf("变量名称 %s 无效", name)
		}
		variables[name] = value
	}
	return RunPrivate{Variables: variables, Owned: map[string]string{}, CreatedNames: map[string]bool{}}, content, nil
}
func checkAssertion(a TaskAssertion, status int, body []byte, document any, variables map[string]string) CheckResult {
	expected, err := interpolate(a.Expected, variables)
	r := CheckResult{Type: a.Type, Path: a.Path, Expected: expected}
	if err != nil {
		r.Actual = err.Error()
		return r
	}
	value, exists := jsonValue(document, a.Path)
	r.Actual = scalar(value)
	switch a.Type {
	case "status":
		r.Actual = strconv.Itoa(status)
		r.Passed = r.Actual == expected
	case "json_equals":
		r.Passed = exists && scalar(value) == expected
	case "json_nonempty":
		r.Passed = exists && value != nil && scalar(value) != "" && scalar(value) != "[]" && scalar(value) != "{}"
	case "json_contains":
		r.Passed = exists && strings.Contains(scalar(value), expected)
	case "array_contains", "array_absent":
		list, ok := value.([]any)
		found := false
		if ok {
			for _, item := range list {
				if a.MatchField != "" {
					match, err := interpolate(a.MatchValue, variables)
					v, exists := jsonValue(item, a.MatchField)
					if err != nil || !exists || scalar(v) != match {
						continue
					}
				}
				candidate, exists := jsonValue(item, a.Field)
				if exists && scalar(candidate) == expected {
					found = true
					break
				}
			}
		}
		r.Actual = fmt.Sprintf("数组有效=%t，匹配=%t", ok, found)
		r.Passed = ok && ((a.Type == "array_contains" && found) || (a.Type == "array_absent" && !found))
	case "binary_sha256":
		hash := sha256.Sum256(body)
		r.Actual = hex.EncodeToString(hash[:])
		r.Passed = r.Actual == expected
	case "auth_rejected":
		code, _ := jsonValue(document, "code")
		text := scalar(code)
		r.Actual = fmt.Sprintf("HTTP %d，业务码 %s", status, text)
		r.Passed = status == 401 || status == 403 || text == "401" || text == "403" || text == "-1"
	}
	return r
}
func extractValues(extracts []TaskExtract, document any, private *RunPrivate) (map[string]string, error) {
	result := map[string]string{}
	for _, e := range extracts {
		value, ok := jsonValue(document, e.Path)
		if e.MatchField != "" {
			expected, err := interpolate(e.MatchValue, private.Variables)
			if err != nil {
				return result, err
			}
			list, isList := value.([]any)
			ok = false
			if isList {
				for _, entry := range list {
					candidate, present := jsonValue(entry, e.MatchField)
					if present && scalar(candidate) == expected {
						value, ok = jsonValue(entry, e.ValueField)
						break
					}
				}
			}
		}
		if !ok || value == nil || scalar(value) == "" {
			return result, fmt.Errorf("无法从 %s 提取 %s，请检查实际响应或补充信息", e.Path, e.Name)
		}
		result[e.Name] = scalar(value)
	}
	for _, e := range extracts {
		private.Variables[e.Name] = result[e.Name]
		if e.Owned {
			private.Owned[e.Name] = result[e.Name]
		}
	}
	return result, nil
}
func ExecuteTaskStep(ctx context.Context, client *http.Client, task Task, step TaskStep, private *RunPrivate, file []byte) StepResult {
	started := time.Now()
	result := StepResult{ID: step.ID, Name: step.Name, Status: "failed", Method: step.Method, Checks: []CheckResult{}, Extracted: map[string]string{}}
	fail := func(err error) StepResult {
		result.Error = redactText(err.Error(), private.Variables)
		result.DurationMs = time.Since(started).Milliseconds()
		return result
	}
	if step.Guard != "" {
		owned, ok := private.Owned[step.Guard]
		if !ok || owned == "" || owned != private.Variables[step.Guard] {
			return fail(fmt.Errorf("本次资源 %s 尚未创建，停止写操作", step.Guard))
		}
		if !bindsGuard(step) {
			return fail(fmt.Errorf("请求没有绑定本次资源变量，已停止写操作"))
		}
	}
	relative, err := interpolate(step.URL, private.Variables)
	if err != nil {
		return fail(err)
	}
	target, err := scopedURL(task.BaseURL, relative)
	if err != nil {
		return fail(err)
	}
	u, err := url.Parse(target)
	if err != nil {
		return fail(err)
	}
	q := u.Query()
	for k, v := range step.Query {
		resolved, err := interpolate(v, private.Variables)
		if err != nil {
			return fail(err)
		}
		q.Set(k, resolved)
	}
	u.RawQuery = q.Encode()
	headers := map[string]string{}
	for k, v := range step.Headers {
		value, err := interpolate(v, private.Variables)
		if err != nil {
			return fail(err)
		}
		headers[k] = value
	}
	body, err := bodyInterpolate(step.Body, private.Variables)
	if err != nil {
		return fail(err)
	}
	var reader io.Reader = strings.NewReader(body)
	if step.Upload {
		var data bytes.Buffer
		writer := multipart.NewWriter(&data)
		field := step.FileField
		if field == "" {
			field = "file"
		}
		part, err := writer.CreateFormFile(field, private.Variables["fileName"])
		if err != nil {
			return fail(err)
		}
		if _, err = part.Write(file); err != nil {
			return fail(err)
		}
		if err = writer.Close(); err != nil {
			return fail(err)
		}
		reader = &data
		headers["Content-Type"] = writer.FormDataContentType()
		result.RequestBody = fmt.Sprintf("multipart 文件 %s，%d 字节", private.Variables["fileName"], len(file))
	} else {
		result.RequestBody = redactText(body, private.Variables)
		if body != "" {
			headers["Content-Type"] = "application/json"
		}
	}
	req, err := http.NewRequestWithContext(ctx, step.Method, u.String(), reader)
	if err != nil {
		return fail(err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	result.URL = redactText(u.String(), private.Variables)
	result.RequestHeaders = redactMap(headers, private.Variables)
	resp, err := client.Do(req)
	if err != nil {
		return fail(err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 6<<20+1))
	if err != nil {
		return fail(err)
	}
	if len(payload) > 6<<20 {
		return fail(fmt.Errorf("响应超过 6 MB，无法完成内容校验"))
	}
	result.StatusCode = resp.StatusCode
	result.ResponseBytes = len(payload)
	var document any
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	_ = decoder.Decode(&document)
	if document != nil {
		result.ResponseBody = redactText(string(payload), private.Variables)
	} else {
		hash := sha256.Sum256(payload)
		result.ResponseBody = fmt.Sprintf("非 JSON 响应，%d 字节；SHA256=%x", len(payload), hash)
	}
	passed := true
	for _, a := range step.Assertions {
		check := checkAssertion(a, resp.StatusCode, payload, document, private.Variables)
		check.Expected = redactText(check.Expected, private.Variables)
		check.Actual = redactText(check.Actual, private.Variables)
		result.Checks = append(result.Checks, check)
		if !check.Passed {
			passed = false
		}
	}
	if passed {
		if private.CreatedNames == nil {
			private.CreatedNames = map[string]bool{}
		}
		creates := step.Method == "POST" && !destructive(step)
		uniqueNames := []string{private.Variables["folderName"], private.Variables["runId"]}
		for _, name := range uniqueNames {
			if name == "" {
				continue
			}
			if creates && strings.Contains(body, name) {
				private.CreatedNames[name] = true
			}
			for _, value := range q {
				if creates && len(value) > 0 && value[0] == name {
					private.CreatedNames[name] = true
				}
			}
		}
		for _, e := range step.Extract {
			if !e.Owned {
				continue
			}
			safe := false
			if e.MatchField != "" {
				match, _ := interpolate(e.MatchValue, private.Variables)
				safe = private.CreatedNames[match]
				for _, owned := range private.Owned {
					if owned == match {
						safe = true
					}
				}
			} else if creates {
				safe = step.Upload && step.Guard != ""
				for _, name := range uniqueNames {
					if name != "" && (strings.Contains(body, name) || strings.Contains(u.RawQuery, name)) {
						safe = true
					}
				}
			}
			if !safe {
				return fail(fmt.Errorf("资源 %s 缺少本次创建来源，不能标为可写资源", e.Name))
			}
		}
		extracted, err := extractValues(step.Extract, document, private)
		if err != nil {
			return fail(err)
		}
		result.Extracted = redactMap(extracted, private.Variables)
		result.Status = "passed"
	} else {
		result.Error = "实际响应不符合业务检查标准"
	}
	result.DurationMs = time.Since(started).Milliseconds()
	return result
}
func SummarizeRun(run *TaskRun) {
	summary := RunSummary{Total: len(run.Task.Steps)}
	for _, r := range run.Results {
		switch r.Status {
		case "passed":
			summary.Passed++
		case "failed":
			summary.Failed++
		case "blocked":
			summary.Blocked++
		default:
			summary.NotRun++
		}
	}
	summary.NotRun += summary.Total - len(run.Results)
	run.Summary = summary
}
func RunTaskPlan(ctx context.Context, client *http.Client, task Task, private *RunPrivate, file []byte, previous []StepResult, progress func(StepResult) error) error {
	if err := ValidatePlan(task.Steps, task.BaseURL); err != nil {
		return err
	}
	old := map[string]StepResult{}
	for _, r := range previous {
		old[r.ID] = r
	}
	statuses := map[string]string{}
	for _, step := range task.Steps {
		if err := ctx.Err(); err != nil {
			return err
		}
		result := StepResult{ID: step.ID, Name: step.Name, Status: "blocked", Error: "前置步骤未通过"}
		blocked := false
		for _, dep := range step.DependsOn {
			if statuses[dep] != "passed" {
				blocked = true
			}
		}
		if !blocked {
			if prior, ok := old[step.ID]; ok && prior.Status == "passed" {
				result = prior
				result.Reused = true
			} else {
				if err := progress(StepResult{ID: step.ID, Name: step.Name, Status: "running"}); err != nil {
					return err
				}
				result = ExecuteTaskStep(ctx, client, task, step, private, file)
			}
		}
		statuses[step.ID] = result.Status
		if err := progress(result); err != nil {
			return err
		}
	}
	return nil
}
