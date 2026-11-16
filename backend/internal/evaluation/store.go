package evaluation

import (
	"context"
	"database/sql"
	"encoding/json"
)

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) (*Store, error) {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS evaluation_suites(
		id BIGSERIAL PRIMARY KEY,project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,config JSONB NOT NULL,updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
	CREATE TABLE IF NOT EXISTS evaluation_suite_runs(
		id BIGSERIAL PRIMARY KEY,project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,report JSONB NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
	CREATE INDEX IF NOT EXISTS idx_evaluation_suite_runs ON evaluation_suite_runs(project_id,created_at DESC);
	UPDATE evaluation_suite_runs SET report=jsonb_set(jsonb_set(report,'{status}','"interrupted"'),'{error}','"服务重启，执行中断；已完成结果保留"')
	WHERE report->>'status' IN ('queued','running');`)
	return &Store{db: db}, err
}
func (s *Store) SaveSuite(ctx context.Context, suite Suite) (Suite, error) {
	raw, err := json.Marshal(suite)
	if err != nil {
		return suite, err
	}
	if suite.ID == 0 {
		err = s.db.QueryRowContext(ctx, "INSERT INTO evaluation_suites(project_id,config) VALUES($1,$2) RETURNING id", suite.ProjectID, raw).Scan(&suite.ID)
	} else {
		err = s.db.QueryRowContext(ctx, "UPDATE evaluation_suites SET config=$3,updated_at=NOW() WHERE project_id=$1 AND id=$2 RETURNING id", suite.ProjectID, suite.ID, raw).Scan(&suite.ID)
	}
	return suite, err
}
func (s *Store) Suites(ctx context.Context, project int) ([]Suite, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,config FROM evaluation_suites WHERE project_id=$1 ORDER BY updated_at DESC", project)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Suite{}
	for rows.Next() {
		var suite Suite
		var raw []byte
		var id int64
		if err = rows.Scan(&id, &raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &suite); err != nil {
			return nil, err
		}
		suite.ID = id
		out = append(out, suite)
	}
	return out, rows.Err()
}
func (s *Store) NewRun(ctx context.Context, run Run) (Run, error) {
	raw, err := json.Marshal(run)
	if err != nil {
		return run, err
	}
	err = s.db.QueryRowContext(ctx, "INSERT INTO evaluation_suite_runs(project_id,report) VALUES($1,$2) RETURNING id,created_at", run.ProjectID, raw).Scan(&run.ID, &run.CreatedAt)
	return run, err
}
func (s *Store) Update(ctx context.Context, run Run) error {
	raw, err := json.Marshal(run)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, "UPDATE evaluation_suite_runs SET report=$3 WHERE project_id=$1 AND id=$2", run.ProjectID, run.ID, raw)
	return err
}
func (s *Store) Runs(ctx context.Context, project int) ([]Run, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,report FROM evaluation_suite_runs WHERE project_id=$1 ORDER BY created_at DESC LIMIT 40", project)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Run{}
	for rows.Next() {
		var run Run
		var raw []byte
		var id int64
		if err = rows.Scan(&id, &raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &run); err != nil {
			return nil, err
		}
		run.ID = id
		out = append(out, run)
	}
	return out, rows.Err()
}
func (s *Store) Run(ctx context.Context, project int, id int64) (Run, error) {
	var run Run
	var raw []byte
	err := s.db.QueryRowContext(ctx, "SELECT report FROM evaluation_suite_runs WHERE project_id=$1 AND id=$2", project, id).Scan(&raw)
	if err != nil {
		return run, err
	}
	err = json.Unmarshal(raw, &run)
	run.ID = id
	return run, err
}
