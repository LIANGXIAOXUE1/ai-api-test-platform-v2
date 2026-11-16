package main

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExpectedErrorVerdicts(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     int
		assertions []Assertion
		readErr    error
		want       bool
	}{
		{"normal200", 200, nil, nil, true},
		{"expected401", 401, []Assertion{{Type: "status", Expected: "401"}}, nil, true},
		{"expected409", 409, []Assertion{{Type: "status", Expected: "409"}, {Type: "json_value", Path: "code", Expected: "409"}}, nil, true},
		{"wrong status", 409, []Assertion{{Type: "status", Expected: "200"}}, nil, false},
		{"business failure", 200, []Assertion{{Type: "json_value", Path: "code", Expected: "200"}}, nil, false},
		{"unexpected error", 500, nil, nil, false},
		{"json only does not bless500", 500, []Assertion{{Type: "json_field", Path: "code"}}, nil, false},
		{"read failure", 401, []Assertion{{Type: "status", Expected: "401"}}, errors.New("read failed"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, _, _, category, _ := runVerdict(tc.assertions, tc.status, `{"code":409}`, tc.readErr)
			if got != tc.want {
				t.Fatalf("got %t want %t", got, tc.want)
			}
			if got && category != "" {
				t.Fatal("passed negative test mislabeled failure")
			}
		})
	}
}

func TestAssistantWithoutModelCannotCreateRuleDraft(t *testing.T) {
	s := &server{}
	r := httptest.NewRequest("POST", "/api/agent/draft", strings.NewReader(`{"prompt":"新增一个 POST 注册接口 http://localhost:8082/register"}`))
	w := httptest.NewRecorder()
	s.agentDraft(w, r)
	if w.Code != 409 || !strings.Contains(w.Body.String(), "assistant_needs_configuration") {
		t.Fatal(w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "rule-agent") || strings.Contains(w.Body.String(), `"request"`) {
		t.Fatal("fabricated draft")
	}
}
