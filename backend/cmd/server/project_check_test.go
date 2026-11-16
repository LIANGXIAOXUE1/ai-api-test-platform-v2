package main

import (
	"testing"
)

func TestEndpointCandidateScorePrefersNamedBusinessIntent(t *testing.T) {
	registration := Endpoint{ID: 1, Name: "注册账号", Method: "POST", URL: "http://localhost:8082/api/auth/register"}
	profile := Endpoint{ID: 2, Name: "更新个人资料", Method: "PUT", URL: "http://localhost:8082/api/user/profile"}
	unrelated := Endpoint{ID: 3, Name: "permanentDelete", Method: "POST", URL: "http://localhost:8082/api/recycle/permanentDelete"}
	registrationScore, _ := endpointCandidateScore("带我配置注册账号接口，我需要注册一个测试账号", registration, profile.ID)
	profileScore, _ := endpointCandidateScore("带我配置注册账号接口，我需要注册一个测试账号", profile, profile.ID)
	unrelatedScore, _ := endpointCandidateScore("带我配置注册账号接口，我需要注册一个测试账号", unrelated, profile.ID)
	if registrationScore <= profileScore {
		t.Fatalf("registration score = %.2f, profile score = %.2f; explicit business intent must outrank unrelated selected endpoint", registrationScore, profileScore)
	}
	if registrationScore <= unrelatedScore {
		t.Fatalf("registration score = %.2f, unrelated score = %.2f; language normalization must not make unrelated endpoints tie", registrationScore, unrelatedScore)
	}
}

func TestAgentRiskRequiresReviewForWriteAndDelete(t *testing.T) {
	read := agentRiskForEndpoint(Endpoint{Method: "GET"}, "test_api")
	if read.Level != "low" {
		t.Fatalf("GET risk = %q, want low", read.Level)
	}
	write := agentRiskForEndpoint(Endpoint{Method: "POST"}, "test_api")
	if write.Level != "medium" {
		t.Fatalf("POST risk = %q, want medium", write.Level)
	}
	remove := agentRiskForEndpoint(Endpoint{Method: "GET"}, "delete_api")
	if remove.Level != "high" {
		t.Fatalf("delete intent risk = %q, want high", remove.Level)
	}
}
