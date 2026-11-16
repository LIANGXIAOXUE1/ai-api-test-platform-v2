package main

import (
	"ai-api-test-platform-v1/backend/internal/projecttest"
	"context"
)

// executeProjectProbe is the only bridge to the existing request executor.
// Saved endpoint headers, query parameters and bodies are not replayed.
func (s *server) executeProjectProbe(ctx context.Context, input projecttest.ProbeRequest) (projecttest.ProbeResult, error) {
	run := s.execute(ctx, runInput{
		ProjectID: input.ProjectID, APIID: input.APIID, Name: input.Name,
		Prompt: "无凭证只读连通性巡检",
		Request: RequestSpec{
			Method: input.Method, URL: input.URL,
			Headers: map[string]string{}, Query: map[string]string{}, Body: "",
		},
	})
	run.ProjectID, run.APIID, run.CaseName = input.ProjectID, input.APIID, input.Name
	stored, err := s.store.addRun(ctx, run)
	if err != nil {
		return projecttest.ProbeResult{}, err
	}
	return projecttest.ProbeResult{
		StatusCode: stored.StatusCode, DurationMs: stored.DurationMs,
		ResponseBody: stored.ResponseBody, ErrorMessage: stored.ErrorMessage,
	}, nil
}
