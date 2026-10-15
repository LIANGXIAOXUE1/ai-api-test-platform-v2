package main

// Pulse API MCP Server
//
// 这是一个零外部依赖的 MCP stdio 适配器。它把 Pulse API 已有的 HTTP 能力
// 暴露给 WorkBuddy、Cursor、Claude Desktop 等支持 MCP 的客户端。

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type toolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

var backendURL = strings.TrimRight(firstNonEmpty(os.Getenv("PULSE_API_URL"), "http://localhost:8081"), "/")

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024), 4<<20)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var request rpcRequest
		if err := json.Unmarshal(line, &request); err != nil {
			writeResponse(rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: err.Error()}})
			continue
		}
		// notifications 不需要响应。
		if len(request.ID) == 0 || string(request.ID) == "null" {
			if request.Method == "notifications/initialized" || request.Method == "notifications/cancelled" {
				continue
			}
		}
		result, err := handleRPC(request.Method, request.Params)
		response := rpcResponse{JSONRPC: "2.0", ID: request.ID}
		if err != nil {
			response.Error = &rpcError{Code: -32603, Message: err.Error()}
		} else {
			response.Result = result
		}
		writeResponse(response)
	}
}

func writeResponse(response rpcResponse) {
	data, _ := json.Marshal(response)
	_, _ = os.Stdout.Write(append(data, '\n'))
}

func handleRPC(method string, raw json.RawMessage) (any, error) {
	switch method {
	case "initialize":
		return map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]string{"name": "pulse-api-mcp", "version": "1.0.0"},
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": toolDefinitions()}, nil
	case "tools/call":
		var params toolCallParams
		if err := json.Unmarshal(raw, &params); err != nil {
			return nil, err
		}
		return callTool(params.Name, params.Arguments)
	default:
		return nil, fmt.Errorf("unsupported MCP method: %s", method)
	}
}

func toolDefinitions() []map[string]any {
	object := func(properties map[string]any, required []string) map[string]any {
		return map[string]any{"type": "object", "properties": properties, "required": required}
	}
	stringField := func(description string) map[string]any {
		return map[string]any{"type": "string", "description": description}
	}
	return []map[string]any{
		{"name": "list_apis", "description": "查询 Pulse API 当前项目的接口列表。只读操作。", "inputSchema": object(map[string]any{"projectId": map[string]any{"type": "integer", "description": "项目 ID"}}, []string{"projectId"})},
		{"name": "get_api_schema", "description": "读取指定接口的请求方法、URL、Headers、Body 和接口类型。只读操作。", "inputSchema": object(map[string]any{"projectId": map[string]any{"type": "integer"}, "apiId": map[string]any{"type": "integer"}}, []string{"projectId", "apiId"})},
		{"name": "create_api_draft", "description": "根据自然语言生成新增接口或测试请求草稿，不写数据库、不发送请求。", "inputSchema": object(map[string]any{"projectId": map[string]any{"type": "integer"}, "apiId": map[string]any{"type": "integer", "description": "可选的当前接口 ID"}, "prompt": stringField("用户想完成的接口配置或测试目标")}, []string{"projectId", "prompt"})},
		{"name": "create_endpoint", "description": "把已确认的接口配置写入 Pulse API。必须显式传 confirm=true。", "inputSchema": object(map[string]any{"confirm": map[string]any{"type": "boolean"}, "projectId": map[string]any{"type": "integer"}, "name": stringField("接口名称"), "method": stringField("GET、POST 等"), "url": stringField("接口 URL"), "kind": stringField("REST、LLM 或 Multimodal"), "headers": map[string]any{"type": "object"}, "query": map[string]any{"type": "object"}, "body": stringField("请求体")}, []string{"confirm", "projectId", "name", "method", "url"})},
		{"name": "run_api_test", "description": "执行一次接口测试。必须显式传 confirm=true，执行前应让用户确认。", "inputSchema": object(map[string]any{"confirm": map[string]any{"type": "boolean"}, "projectId": map[string]any{"type": "integer"}, "apiId": map[string]any{"type": "integer"}, "name": stringField("测试名称"), "prompt": stringField("测试目标"), "request": map[string]any{"type": "object"}, "assertions": map[string]any{"type": "array"}, "saveCase": map[string]any{"type": "boolean"}}, []string{"confirm", "projectId", "request"})},
		{"name": "analyze_response", "description": "根据状态码、错误信息和响应 Body 分析接口失败原因。只读操作。", "inputSchema": object(map[string]any{"statusCode": map[string]any{"type": "integer"}, "errorMessage": stringField("错误信息"), "responseBody": stringField("响应内容")}, []string{"statusCode"})},
		{"name": "create_regression_case", "description": "把一次已确认的接口请求保存为回归用例。必须显式传 confirm=true。", "inputSchema": object(map[string]any{"confirm": map[string]any{"type": "boolean"}, "projectId": map[string]any{"type": "integer"}, "apiId": map[string]any{"type": "integer"}, "name": stringField("用例名称"), "prompt": stringField("测试目标"), "request": map[string]any{"type": "object"}, "assertions": map[string]any{"type": "array"}}, []string{"confirm", "projectId", "request"})},
	}
}

func callTool(name string, args map[string]any) (any, error) {
	switch name {
	case "list_apis":
		projectID, err := intArg(args, "projectId")
		if err != nil {
			return toolError(err)
		}
		result, err := backendRequest(http.MethodGet, fmt.Sprintf("/api/apis?projectId=%d", projectID), nil)
		if err != nil {
			return toolError(err)
		}
		return toolResult(result), nil
	case "get_api_schema":
		projectID, err := intArg(args, "projectId")
		if err != nil {
			return toolError(err)
		}
		apiID, err := intArg(args, "apiId")
		if err != nil {
			return toolError(err)
		}
		items, err := backendRequest(http.MethodGet, fmt.Sprintf("/api/apis?projectId=%d", projectID), nil)
		if err != nil {
			return toolError(err)
		}
		list, ok := items.([]any)
		if !ok {
			return toolError(errors.New("接口列表格式不正确"))
		}
		for _, item := range list {
			if object, ok := item.(map[string]any); ok && intValue(object["id"]) == apiID {
				return toolResult(object), nil
			}
		}
		return toolError(fmt.Errorf("找不到接口 %d", apiID))
	case "create_api_draft":
		projectID, err := intArg(args, "projectId")
		if err != nil {
			return toolError(err)
		}
		apiID, _ := intArg(args, "apiId")
		prompt, err := stringArg(args, "prompt")
		if err != nil {
			return toolError(err)
		}
		result, err := backendRequest(http.MethodPost, "/api/agent/draft", map[string]any{"projectId": projectID, "apiId": apiID, "prompt": prompt})
		if err != nil {
			return toolError(err)
		}
		return toolResult(result), nil
	case "create_endpoint":
		if !confirmed(args) {
			return toolError(errors.New("这是写入操作，请在用户确认后再次调用，并传 confirm=true"))
		}
		result, err := backendRequest(http.MethodPost, "/api/apis", map[string]any{"projectId": args["projectId"], "name": args["name"], "method": args["method"], "url": args["url"], "kind": firstNonEmpty(fmt.Sprint(args["kind"]), "REST"), "headers": objectArg(args, "headers"), "query": objectArg(args, "query"), "body": args["body"], "enabled": true})
		if err != nil {
			return toolError(err)
		}
		return toolResult(result), nil
	case "run_api_test", "create_regression_case":
		if !confirmed(args) {
			return toolError(errors.New("这是执行或写入操作，请在用户确认后再次调用，并传 confirm=true"))
		}
		payload := map[string]any{"projectId": args["projectId"], "apiId": args["apiId"], "name": args["name"], "prompt": args["prompt"], "request": args["request"], "assertions": args["assertions"], "saveCase": name == "create_regression_case" || boolArg(args, "saveCase")}
		result, err := backendRequest(http.MethodPost, "/api/runs", payload)
		if err != nil {
			return toolError(err)
		}
		return toolResult(result), nil
	case "analyze_response":
		return toolResult(analyzeResponse(args)), nil
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

func analyzeResponse(args map[string]any) map[string]any {
	status := intValue(args["statusCode"])
	title, suggestion := "请求未通过", "检查 URL、网络和请求参数。"
	switch {
	case status == 401:
		title, suggestion = "鉴权失败", "检查 Authorization、API Key 或 Token 是否缺失、过期。"
	case status == 403:
		title, suggestion = "权限不足", "凭证可能有效，但当前账号没有访问权限。"
	case status == 404:
		title, suggestion = "接口不存在", "检查 URL、接口版本和网关路由。"
	case status == 429:
		title, suggestion = "请求过快或配额不足", "检查限流、模型配额和重试间隔。"
	case status >= 500:
		title, suggestion = "服务端异常", "请求已到达服务端，建议查看上游服务日志。"
	case status >= 200 && status < 300:
		title, suggestion = "请求成功", "可以继续检查响应字段和业务断言。"
	}
	return map[string]any{"title": title, "statusCode": status, "suggestion": suggestion, "errorMessage": args["errorMessage"], "responseBody": args["responseBody"]}
}

func backendRequest(method, path string, payload any) (any, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, backendURL+path, body)
	if err != nil {
		return nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := (&http.Client{}).Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	var result any
	if json.Unmarshal(data, &result) != nil {
		result = string(data)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("Pulse API 返回 HTTP %d: %v", response.StatusCode, result)
	}
	return result, nil
}

func toolResult(value any) map[string]any {
	data, _ := json.Marshal(value)
	return map[string]any{"content": []map[string]string{{"type": "text", "text": string(data)}}, "structuredContent": value}
}

func toolError(err error) (map[string]any, error) {
	return map[string]any{"isError": true, "content": []map[string]string{{"type": "text", "text": err.Error()}}}, nil
}

func confirmed(args map[string]any) bool           { value, ok := args["confirm"].(bool); return ok && value }
func boolArg(args map[string]any, key string) bool { value, _ := args[key].(bool); return value }
func objectArg(args map[string]any, key string) map[string]any {
	value, _ := args[key].(map[string]any)
	if value == nil {
		return map[string]any{}
	}
	return value
}
func stringArg(args map[string]any, key string) (string, error) {
	value, ok := args[key].(string)
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("参数 %s 不能为空", key)
	}
	return value, nil
}
func intArg(args map[string]any, key string) (int, error) {
	value := intValue(args[key])
	if value == 0 {
		return 0, fmt.Errorf("参数 %s 不能为空", key)
	}
	return value, nil
}
func intValue(value any) int {
	switch item := value.(type) {
	case int:
		return item
	case float64:
		return int(item)
	case json.Number:
		result, _ := strconv.Atoi(string(item))
		return result
	case string:
		result, _ := strconv.Atoi(item)
		return result
	}
	return 0
}
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" && value != "<nil>" {
			return value
		}
	}
	return ""
}
