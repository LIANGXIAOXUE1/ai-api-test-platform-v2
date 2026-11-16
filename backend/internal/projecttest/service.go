package projecttest

import (
	"context"
	"strings"
	"time"
)

type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }

type Service struct {
	repository Repository
	executor   ProbeExecutor
}

func NewService(repository Repository, executor ProbeExecutor) *Service {
	return &Service{repository: repository, executor: executor}
}

// Run preserves the existing connectivity-check contract. Business acceptance
// execution will be added separately, without changing this legacy endpoint.
func (s *Service) Run(ctx context.Context, input Input) (Report, error) {
	if input.ProjectID <= 0 {
		return Report{}, &ValidationError{Message: "请选择项目后再执行巡检"}
	}
	baseURL, err := normalizeCheckBaseURL(input.BaseURL)
	if err != nil {
		return Report{}, &ValidationError{Message: err.Error()}
	}
	if input.Permission != "auto" {
		input.Permission = "review"
	}
	endpoints, err := s.repository.Endpoints(ctx, input.ProjectID)
	if err != nil {
		return Report{}, err
	}
	output := Report{ProjectID: input.ProjectID, BaseURL: baseURL, Task: strings.TrimSpace(input.Task), Permission: input.Permission, CreatedAt: time.Now(), Items: []Item{}}
	for _, endpoint := range endpoints {
		risk, reason := checkRisk(endpoint, baseURL)
		item := Item{APIID: endpoint.ID, Name: endpoint.Name, Method: endpoint.Method, URL: checkRequestURL(endpoint.URL, baseURL), Risk: risk, Reason: reason, Decision: "等待审核"}
		output.Summary.Total++
		if input.Permission != "auto" || risk != "只读" {
			output.Summary.Pending++
			output.Items = append(output.Items, item)
			continue
		}
		stored, storeErr := s.executor.Execute(ctx, ProbeRequest{
			ProjectID: input.ProjectID, APIID: endpoint.ID,
			Name: "自动联通性巡检 · " + endpoint.Name, Method: endpoint.Method, URL: item.URL,
		})
		item.Decision = "已执行"
		if storeErr != nil {
			item.Error = storeErr.Error()
			output.Summary.Unavailable++
			output.Items = append(output.Items, item)
			continue
		}
		item.StatusCode, item.DurationMs, item.BusinessCode = stored.StatusCode, stored.DurationMs, businessCode(stored.ResponseBody)
		item.Reachable, item.Error = stored.StatusCode > 0, stored.ErrorMessage
		if item.Reachable {
			output.Summary.Reachable++
		} else {
			output.Summary.Unavailable++
		}
		output.Items = append(output.Items, item)
	}
	output.Executed = input.Permission == "auto"
	output.Markdown = projectCheckMarkdown(output)
	return s.repository.SaveReport(ctx, output)
}

func (s *Service) Reports(ctx context.Context, projectID int) ([]Report, error) {
	if projectID <= 0 {
		return nil, &ValidationError{Message: "请选择项目后再查看巡检记录"}
	}
	return s.repository.Reports(ctx, projectID)
}
