package main

import (
	"ai-api-test-platform-v1/backend/internal/security"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

type resourceResolver struct{ db *sql.DB }

func (s resourceResolver) Resolve(ctx context.Context, kind string, id int64) (security.ResourceScope, error) {
	scope := security.ResourceScope{Enabled: true}
	if kind == "model" {
		err := s.db.QueryRowContext(ctx, "SELECT enabled FROM model_profiles WHERE id=$1", id).Scan(&scope.Enabled)
		return scope, err
	}
	table := map[string]string{"api": "apis", "case": "test_cases", "document": "project_documents", "task": "project_test_tasks", "project_run": "project_test_runs", "evaluation_suite": "evaluation_suites", "evaluation_run": "evaluation_suite_runs", "check_report": "project_check_reports", "run": "runs"}[kind]
	if table == "" {
		return scope, fmt.Errorf("unknown resource kind")
	}
	query := "SELECT project_id FROM " + table + " WHERE id=$1"
	if kind == "task" {
		var raw []byte
		err := s.db.QueryRowContext(ctx, "SELECT project_id,config FROM project_test_tasks WHERE id=$1", id).Scan(&scope.ProjectID, &raw)
		var config struct {
			Permission string `json:"permission"`
		}
		_ = json.Unmarshal(raw, &config)
		scope.Auto = config.Permission == "auto"
		return scope, err
	}
	if kind == "project_run" {
		var raw []byte
		err := s.db.QueryRowContext(ctx, "SELECT project_id,report FROM project_test_runs WHERE id=$1", id).Scan(&scope.ProjectID, &raw)
		var report struct {
			Task struct {
				Permission string `json:"permission"`
			} `json:"task"`
		}
		_ = json.Unmarshal(raw, &report)
		scope.Auto = report.Task.Permission == "auto"
		return scope, err
	}
	return scope, s.db.QueryRowContext(ctx, query, id).Scan(&scope.ProjectID)
}
