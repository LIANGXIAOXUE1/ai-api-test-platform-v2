package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMCPToolDefinitionsAndSafety(t *testing.T) {
	result, err := handleRPC("tools/list", nil)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(result)
	if err != nil || !strings.Contains(string(data), `"run_api_test"`) || !strings.Contains(string(data), `"create_endpoint"`) {
		t.Fatalf("工具列表缺少核心工具：%s", string(data))
	}
	blocked, err := callTool("create_endpoint", map[string]any{"projectId": 1, "name": "demo", "method": "GET", "url": "/demo"})
	if err != nil {
		t.Fatal(err)
	}
	blockedData, _ := json.Marshal(blocked)
	if !strings.Contains(string(blockedData), "confirm=true") {
		t.Fatalf("未确认的写入操作没有被阻止：%s", blockedData)
	}
}

func TestMCPHTTPBridgeWrapsToolResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/apis" {
			t.Fatalf("请求路径错误：%s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[ {"id": 1, "name": "Echo"} ]`))
	}))
	defer server.Close()
	original := backendURL
	backendURL = server.URL
	defer func() { backendURL = original }()

	result, err := callTool("list_apis", map[string]any{"projectId": float64(1)})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(result)
	if !strings.Contains(string(data), `"content"`) || !strings.Contains(string(data), `"Echo"`) {
		t.Fatalf("工具结果没有按 MCP content 返回：%s", data)
	}
}
