package projecttest

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type TaskStore struct {
	db     *sql.DB
	cipher cipher.AEAD
}

func NewTaskStore(db *sql.DB, directory string) (*TaskStore, error) {
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, err
	}
	keyPath := filepath.Join(directory, "project-test.key")
	key, err := os.ReadFile(keyPath)
	if errors.Is(err, os.ErrNotExist) {
		key = make([]byte, 32)
		if _, err = rand.Read(key); err != nil {
			return nil, err
		}
		f, createErr := os.OpenFile(keyPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if createErr != nil {
			return nil, createErr
		}
		_, err = f.Write(key)
		closeErr := f.Close()
		if err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, closeErr
		}
	} else if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS project_test_tasks (
		id BIGSERIAL PRIMARY KEY, project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
		config JSONB NOT NULL, secret TEXT NOT NULL DEFAULT '', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
	CREATE INDEX IF NOT EXISTS idx_project_test_tasks ON project_test_tasks(project_id);
	CREATE TABLE IF NOT EXISTS project_test_runs (
		id BIGSERIAL PRIMARY KEY, project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
		task_id BIGINT NOT NULL REFERENCES project_test_tasks(id) ON DELETE CASCADE,
		report JSONB NOT NULL, private TEXT NOT NULL DEFAULT '', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
	CREATE INDEX IF NOT EXISTS idx_project_test_runs ON project_test_runs(project_id,created_at DESC);
	UPDATE project_test_runs SET report=jsonb_set(jsonb_set(report,'{status}','"interrupted"'),'{error}','"服务重启，执行中断；可重新运行"')
	WHERE report->>'status' IN ('queued','running');`)
	if err != nil {
		return nil, err
	}
	return &TaskStore{db: db, cipher: aead}, nil
}
func (s *TaskStore) seal(value string) string {
	nonce := make([]byte, s.cipher.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(s.cipher.Seal(nonce, nonce, []byte(value), nil))
}
func (s *TaskStore) open(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(data) < s.cipher.NonceSize() {
		return "", errors.New("测试凭证读取失败")
	}
	n := s.cipher.NonceSize()
	plain, err := s.cipher.Open(nil, data[:n], data[n:], nil)
	return string(plain), err
}
func PublicTask(task Task) Task {
	task.HasPassword = task.Password != "" || task.HasPassword
	task.Password = ""
	task.FileData = ""
	task.InitialVariables = redactMap(task.InitialVariables, task.InitialVariables)
	return task
}
func (s *TaskStore) SaveTask(ctx context.Context, task Task) (Task, error) {
	if task.ID > 0 {
		old, err := s.Task(ctx, task.ProjectID, task.ID)
		if err != nil {
			return Task{}, err
		}
		if task.Password == "" {
			task.Password = old.Password
		}
		for name, value := range task.InitialVariables {
			if value == "••••••" {
				task.InitialVariables[name] = old.InitialVariables[name]
			}
		}
		if task.FileData == "" && task.FileMode == "manual" && task.FileName == old.FileName {
			task.FileData = old.FileData
			task.FileSize = old.FileSize
		}
	}
	password := task.Password
	variables := task.InitialVariables
	secretData, _ := json.Marshal(map[string]any{"password": password, "variables": variables})
	task.Password = ""
	task.InitialVariables = redactMap(variables, variables)
	task.HasPassword = password != ""
	raw, err := json.Marshal(task)
	if err != nil {
		return Task{}, err
	}
	secret := s.seal(string(secretData))
	if task.ID == 0 {
		err = s.db.QueryRowContext(ctx, "INSERT INTO project_test_tasks(project_id,config,secret) VALUES($1,$2,$3) RETURNING id,created_at,updated_at", task.ProjectID, raw, secret).Scan(&task.ID, &task.CreatedAt, &task.UpdatedAt)
	} else {
		err = s.db.QueryRowContext(ctx, "UPDATE project_test_tasks SET config=$3,secret=$4,updated_at=NOW() WHERE project_id=$1 AND id=$2 RETURNING created_at,updated_at", task.ProjectID, task.ID, raw, secret).Scan(&task.CreatedAt, &task.UpdatedAt)
	}
	task.Password = password
	task.InitialVariables = variables
	return PublicTask(task), err
}
func (s *TaskStore) Task(ctx context.Context, project int, id int64) (Task, error) {
	var task Task
	var raw []byte
	var secret string
	err := s.db.QueryRowContext(ctx, "SELECT config,secret,created_at,updated_at FROM project_test_tasks WHERE project_id=$1 AND id=$2", project, id).Scan(&raw, &secret, &task.CreatedAt, &task.UpdatedAt)
	if err != nil {
		return task, err
	}
	created, updated := task.CreatedAt, task.UpdatedAt
	if err = json.Unmarshal(raw, &task); err != nil {
		return task, err
	}
	task.ID = id
	task.CreatedAt = created
	task.UpdatedAt = updated
	plain, openErr := s.open(secret)
	if openErr != nil {
		return task, openErr
	}
	var stored struct {
		Password  string            `json:"password"`
		Variables map[string]string `json:"variables"`
	}
	if json.Unmarshal([]byte(plain), &stored) == nil {
		task.Password = stored.Password
		task.InitialVariables = stored.Variables
	} else {
		task.Password = plain
	}
	return task, err
}
func (s *TaskStore) Tasks(ctx context.Context, project int) ([]Task, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,config,created_at,updated_at FROM project_test_tasks WHERE project_id=$1 ORDER BY updated_at DESC", project)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := []Task{}
	for rows.Next() {
		var task Task
		var raw []byte
		var created, updated time.Time
		if err = rows.Scan(&task.ID, &raw, &created, &updated); err != nil {
			return nil, err
		}
		id := task.ID
		if err = json.Unmarshal(raw, &task); err != nil {
			return nil, err
		}
		task.ID = id
		task.CreatedAt = created
		task.UpdatedAt = updated
		tasks = append(tasks, PublicTask(task))
	}
	return tasks, rows.Err()
}
func (s *TaskStore) NewRun(ctx context.Context, run TaskRun) (TaskRun, error) {
	raw, err := json.Marshal(run)
	if err != nil {
		return run, err
	}
	err = s.db.QueryRowContext(ctx, "INSERT INTO project_test_runs(project_id,task_id,report) VALUES($1,$2,$3) RETURNING id,created_at", run.ProjectID, run.TaskID, raw).Scan(&run.ID, &run.CreatedAt)
	return run, err
}
func (s *TaskStore) UpdateRun(ctx context.Context, run TaskRun, private RunPrivate) error {
	raw, err := json.Marshal(run)
	if err != nil {
		return err
	}
	priv, err := json.Marshal(private)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, "UPDATE project_test_runs SET report=$3,private=$4 WHERE project_id=$1 AND id=$2", run.ProjectID, run.ID, raw, s.seal(string(priv)))
	return err
}
func (s *TaskStore) Run(ctx context.Context, project int, id int64) (TaskRun, RunPrivate, error) {
	var run TaskRun
	var private RunPrivate
	var raw []byte
	var encrypted string
	err := s.db.QueryRowContext(ctx, "SELECT report,private,created_at FROM project_test_runs WHERE project_id=$1 AND id=$2", project, id).Scan(&raw, &encrypted, &run.CreatedAt)
	if err != nil {
		return run, private, err
	}
	created := run.CreatedAt
	if err = json.Unmarshal(raw, &run); err != nil {
		return run, private, err
	}
	run.ID = id
	run.CreatedAt = created
	plain, err := s.open(encrypted)
	if err == nil && plain != "" {
		err = json.Unmarshal([]byte(plain), &private)
	}
	return run, private, err
}
func (s *TaskStore) Runs(ctx context.Context, project int) ([]TaskRun, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,report,created_at FROM project_test_runs WHERE project_id=$1 ORDER BY created_at DESC LIMIT 60", project)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	runs := []TaskRun{}
	for rows.Next() {
		var run TaskRun
		var raw []byte
		var created time.Time
		if err = rows.Scan(&run.ID, &raw, &created); err != nil {
			return nil, err
		}
		id := run.ID
		if err = json.Unmarshal(raw, &run); err != nil {
			return nil, err
		}
		run.ID = id
		run.CreatedAt = created
		runs = append(runs, run)
	}
	return runs, rows.Err()
}
