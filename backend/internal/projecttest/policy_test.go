package projecttest

import (
	"net/url"
	"testing"
)

func mustCheckURL(t *testing.T, value string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(value)
	if err != nil {
		t.Fatalf("parse URL %q: %v", value, err)
	}
	return parsed
}

func TestSameCheckTargetRecognizesLocalhostAliases(t *testing.T) {
	if !sameCheckTarget(mustCheckURL(t, "http://localhost:8082"), mustCheckURL(t, "http://127.0.0.1:8082/api/health")) {
		t.Fatal("localhost and 127.0.0.1 on the same port should be one local target")
	}
	if sameCheckTarget(mustCheckURL(t, "http://localhost:8082"), mustCheckURL(t, "http://127.0.0.1:8083/api/health")) {
		t.Fatal("different ports must not be considered the same local target")
	}
}

func TestProjectCheckUsesChosenLoopbackBaseURL(t *testing.T) {
	baseURL := "http://localhost:8082"
	endpoint := Endpoint{Method: "GET", URL: "http://127.0.0.1:8082/api/files"}
	risk, _ := checkRisk(endpoint, baseURL)
	if risk != "只读" {
		t.Fatalf("risk = %q, want 只读", risk)
	}
	if got := checkRequestURL(endpoint.URL, baseURL); got != "http://localhost:8082/api/files" {
		t.Fatalf("check request URL = %q", got)
	}
}
