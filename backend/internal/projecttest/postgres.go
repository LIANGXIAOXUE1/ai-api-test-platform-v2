package projecttest

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// PostgresRepository uses the existing apis and project_check_reports tables.
// Schema creation remains in the application's existing database bootstrap.
type PostgresRepository struct{ db *sql.DB }

func NewPostgresRepository(db *sql.DB) *PostgresRepository { return &PostgresRepository{db: db} }

func (s *PostgresRepository) Endpoints(ctx context.Context, projectID int) ([]Endpoint, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,name,method,url FROM apis WHERE project_id=$1 ORDER BY id", projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var endpoints []Endpoint
	for rows.Next() {
		var item Endpoint
		if err := rows.Scan(&item.ID, &item.Name, &item.Method, &item.URL); err != nil {
			return nil, err
		}
		endpoints = append(endpoints, item)
	}
	return endpoints, rows.Err()
}

func (s *PostgresRepository) SaveReport(ctx context.Context, output Report) (Report, error) {
	data, err := json.Marshal(output)
	if err != nil {
		return output, err
	}
	var createdAt time.Time
	err = s.db.QueryRowContext(ctx, `INSERT INTO project_check_reports(project_id,report) VALUES($1,$2::jsonb) RETURNING id,created_at`, output.ProjectID, data).Scan(&output.ID, &createdAt)
	if err != nil {
		return output, err
	}
	output.CreatedAt = createdAt
	return output, nil
}

func (s *PostgresRepository) Reports(ctx context.Context, projectID int) ([]Report, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,report,created_at FROM project_check_reports WHERE project_id=$1 ORDER BY created_at DESC LIMIT 30`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	reports := []Report{}
	for rows.Next() {
		var item Report
		var raw []byte
		var createdAt time.Time
		if err := rows.Scan(&item.ID, &raw, &createdAt); err != nil {
			return nil, err
		}
		reportID := item.ID
		if err := json.Unmarshal(raw, &item); err != nil {
			return nil, err
		}
		item.ID = reportID
		item.CreatedAt = createdAt
		reports = append(reports, item)
	}
	return reports, rows.Err()
}
