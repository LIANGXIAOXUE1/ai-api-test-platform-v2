package main

import (
	"context"
	"strings"
	"testing"
)

func TestRetrievalKeepsCompleteShortRules(t *testing.T) {
	content := "文件上传规则\n请求参数必须包含parentId和file。\n成功时HTTP 200，业务码0。\n同名文件拒绝，业务码409。"
	_, snippet := searchDocumentText(content, "请生成文件上传测试用例")
	if snippet != content {
		t.Fatalf("short rule lost conditions: %s", snippet)
	}
}
func TestRetrievalChineseSentenceAndNeighbours(t *testing.T) {
	content := strings.Repeat("无关内容占据文档前部。\n", 400) + "文件夹名称规则\n同一目录下不允许创建同名文件夹。\n失败返回HTTP 409。\n" + strings.Repeat("其他模块说明。\n", 400)
	score, snippet := retrieveDocument(content, "请帮我测试同名文件夹", 2000)
	if score <= 0 || !strings.Contains(snippet, "同一目录") || !strings.Contains(snippet, "HTTP 409") {
		t.Fatalf("lost relevant rule/context: %s", snippet)
	}
}
func TestBusinessDraftDoesNotInventExecutableEndpoint(t *testing.T) {
	for _, design := range []bool{true, false} {
		result := businessCasesResult{Cases: []BusinessTestCase{{Name: "订单创建", Request: RequestSpec{Method: "POST", URL: "/orders"}, Assertions: []Assertion{{Type: "status", Expected: "200"}}}}}
		validateBusinessCases(&result, map[string]int{"POST /api/demo/echo": 1}, design)
		if design {
			if len(result.Cases) != 1 || result.Cases[0].Status != "needs_configuration" || result.Cases[0].Request.URL != "" {
				t.Fatal("design emitted invented request")
			}
		} else if len(result.Cases) != 0 || len(result.MissingInformation) == 0 {
			t.Fatal("execution did not require missing contract")
		}
	}
}
func TestBusinessCasesKeepValidPartialResults(t *testing.T) {
	result := businessCasesResult{MissingInformation: []string{"缺少退款契约"}, Cases: []BusinessTestCase{{Name: "Echo", Request: RequestSpec{Method: "POST", URL: "/api/demo/echo"}, Assertions: []Assertion{{Type: "status", Expected: "200"}}}}}
	validateBusinessCases(&result, map[string]int{"POST /api/demo/echo": 10}, false)
	if len(result.Cases) != 1 || result.Cases[0].APIID != 10 || result.Cases[0].Status != "ready_for_review" || len(result.MissingInformation) != 1 {
		t.Fatal("discarded valid partial cases")
	}
}
func TestAssistantModelContextIsIsolated(t *testing.T) {
	s := &server{}
	one := context.WithValue(context.Background(), assistantProfileContextKey{}, ModelProfile{ID: 1, Model: "first", Enabled: true, Verified: true})
	two := context.WithValue(context.Background(), assistantProfileContextKey{}, ModelProfile{ID: 2, Model: "second", Enabled: true, Verified: true})
	p, ok := s.activeAssistantProfile(one)
	q, ok2 := s.activeAssistantProfile(two)
	if !ok || !ok2 || p.Model != "first" || q.Model != "second" {
		t.Fatal("request model leaked")
	}
	if _, ok := s.activeAssistantProfile(); ok {
		t.Fatal("request model changed global default")
	}
}
