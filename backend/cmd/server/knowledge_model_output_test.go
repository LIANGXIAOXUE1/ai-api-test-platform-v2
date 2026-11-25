package main

import (
	"strings"
	"testing"
)

func TestModelCasesAcceptNativeJSONValuesWithoutLosingAssertions(t *testing.T) {
	var result businessCasesResult
	err := parseJSONObject(`{"documentAssessment":{"status":"usable"},"cases":[{"name":"保留JSON类型","request":{"method":"POST","url":"/api/demo/echo","body":{"count":2,"enabled":true}},"assertions":[{"type":"status","expected":200},{"type":"json_value","path":"received","expected":{"enabled":true,"count":2}},{"type":"json_value","path":"received.enabled","expected":true},{"type":"json_value","path":"received.count","expected":2}]}]}`, &result)
	if err != nil || len(result.Cases) != 1 {
		t.Fatalf("native model JSON rejected: %v", err)
	}
	item := result.Cases[0]
	if item.Request.Body != `{"count":2,"enabled":true}` {
		t.Fatalf("body changed meaning: %s", item.Request.Body)
	}
	pass, total := evaluateAssertions(item.Assertions, 200, `{"received":{"enabled":true,"count":2}}`)
	if pass != 4 || total != 4 {
		t.Fatalf("normalized assertions fail: %d/%d", pass, total)
	}
}

func TestModelMalformedCaseDoesNotDiscardValidSibling(t *testing.T) {
	var result businessCasesResult
	err := parseJSONObject(`{"documentAssessment":{"status":"usable"},"cases":[{"name":"bad","request":false},{"name":"valid","request":{"method":"POST","url":"/api/demo/echo","body":"{}"},"assertions":[{"type":"status","expected":"200"}]}]}`, &result)
	if err != nil || len(result.Cases) != 1 || result.Cases[0].Name != "valid" || len(result.MissingInformation) != 1 || result.InvalidCaseCount != 1 || !strings.Contains(result.MissingInformation[0], "第1条") {
		t.Fatal("malformed sibling hides valid result or lacks actionable reason")
	}
}

func TestAllMalformedModelCasesExplainFormatFailure(t *testing.T) {
	var result businessCasesResult
	err := parseJSONObject(`{"documentAssessment":{"status":"usable"},"cases":[{"name":"bad","request":false}]}`, &result)
	status, code, message, steps, _, _ := knowledgeGenerationFeedback(result)
	if err != nil || status != "cannot_generate" || code != "knowledge_model_case_invalid" || !strings.Contains(message, "格式") || len(steps) == 0 {
		t.Fatal("model format failure mistaken for insufficient business rules")
	}
}
