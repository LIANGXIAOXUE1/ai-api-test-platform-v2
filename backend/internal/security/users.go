package security

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/pbkdf2"
)

const RoleAdmin = "administrator"
const RoleTester = "tester"
const RoleViewer = "viewer"

type User struct {
	ID         int64     `json:"id"`
	Username   string    `json:"username"`
	Role       string    `json:"role"`
	ProjectIDs []int     `json:"projectIds"`
	AllowAuto  bool      `json:"allowAuto"`
	Enabled    bool      `json:"enabled"`
	Version    int64     `json:"-"`
	CreatedAt  time.Time `json:"createdAt"`
}

func (u User) CanAccess(project int) bool {
	if u.Role == RoleAdmin {
		return true
	}
	for _, id := range u.ProjectIDs {
		if project > 0 && id == project {
			return true
		}
	}
	return false
}
func (u User) CanExecute() bool { return u.Role == RoleAdmin || u.Role == RoleTester }
func (u User) CanAuto() bool    { return u.Role == RoleAdmin || (u.Role == RoleTester && u.AllowAuto) }

type UserRepository interface {
	Authenticate(context.Context, string, string) (User, bool, error)
	User(context.Context, int64) (User, error)
}
type UserStore struct{ db *sql.DB }

const userSchema = `CREATE TABLE IF NOT EXISTS platform_users (
 id BIGSERIAL PRIMARY KEY, username TEXT NOT NULL UNIQUE, password_hash TEXT NOT NULL,
 role TEXT NOT NULL CHECK(role IN ('administrator','tester','viewer')),
 allow_auto BOOLEAN NOT NULL DEFAULT FALSE, enabled BOOLEAN NOT NULL DEFAULT TRUE,
 version BIGINT NOT NULL DEFAULT 1, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
 CREATE TABLE IF NOT EXISTS platform_user_projects (
 user_id BIGINT NOT NULL REFERENCES platform_users(id) ON DELETE CASCADE,
 project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE, PRIMARY KEY(user_id,project_id));
 CREATE TABLE IF NOT EXISTS platform_operation_audits (
 id BIGSERIAL PRIMARY KEY, actor_id BIGINT REFERENCES platform_users(id) ON DELETE SET NULL,
 actor_name TEXT NOT NULL, method TEXT NOT NULL, path TEXT NOT NULL,
 project_id INTEGER, resource_id BIGINT NOT NULL DEFAULT 0, status_code INTEGER NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
 CREATE INDEX IF NOT EXISTS idx_platform_operation_audits ON platform_operation_audits(created_at DESC);`

func NewUserStore(ctx context.Context, db *sql.DB, username, password string) (*UserStore, error) {
	if _, err := db.ExecContext(ctx, userSchema); err != nil {
		return nil, err
	}
	s := &UserStore{db: db}
	if username == "" {
		return s, nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "LOCK TABLE platform_users IN SHARE ROW EXCLUSIVE MODE"); err != nil {
		return nil, err
	}
	var count int
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM platform_users").Scan(&count); err != nil {
		return nil, err
	}
	if count == 0 {
		hash, err := HashPassword(password)
		if err != nil {
			return nil, err
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO platform_users(username,password_hash,role,allow_auto) VALUES($1,$2,$3,true)", username, hash, RoleAdmin); err != nil {
			return nil, err
		}
	}
	return s, tx.Commit()
}

func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := pbkdf2.Key([]byte(password), salt, 600000, 32, sha256.New)
	return "pbkdf2-sha256$600000$" + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(hash), nil
}
func CheckPassword(hash, password string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" || parts[1] != "600000" {
		return false
	}
	salt, e1 := base64.RawStdEncoding.DecodeString(parts[2])
	expected, e2 := base64.RawStdEncoding.DecodeString(parts[3])
	if e1 != nil || e2 != nil || len(salt) != 16 || len(expected) != 32 {
		return false
	}
	actual := pbkdf2.Key([]byte(password), salt, 600000, 32, sha256.New)
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func (s *UserStore) User(ctx context.Context, id int64) (User, error) {
	u := User{ProjectIDs: []int{}}
	err := s.db.QueryRowContext(ctx, "SELECT id,username,role,allow_auto,enabled,version,created_at FROM platform_users WHERE id=$1", id).Scan(&u.ID, &u.Username, &u.Role, &u.AllowAuto, &u.Enabled, &u.Version, &u.CreatedAt)
	if err != nil {
		return u, err
	}
	rows, err := s.db.QueryContext(ctx, "SELECT project_id FROM platform_user_projects WHERE user_id=$1 ORDER BY project_id", id)
	if err != nil {
		return u, err
	}
	defer rows.Close()
	for rows.Next() {
		var project int
		if err = rows.Scan(&project); err != nil {
			return u, err
		}
		u.ProjectIDs = append(u.ProjectIDs, project)
	}
	return u, rows.Err()
}
func (s *UserStore) Authenticate(ctx context.Context, username, password string) (User, bool, error) {
	var id int64
	var hash string
	err := s.db.QueryRowContext(ctx, "SELECT id,password_hash FROM platform_users WHERE username=$1", username).Scan(&id, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		// Match the password-verification cost without exposing account existence.
		_ = pbkdf2.Key([]byte(password), make([]byte, 16), 600000, 32, sha256.New)
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, err
	}
	valid := CheckPassword(hash, password)
	u, err := s.User(ctx, id)
	return u, valid && u.Enabled, err
}
func (s *UserStore) Users(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id FROM platform_users ORDER BY id")
	if err != nil {
		return nil, err
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	users := []User{}
	for _, id := range ids {
		u, err := s.User(ctx, id)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

type UserInput struct {
	ID         int64  `json:"id"`
	Username   string `json:"username"`
	Password   string `json:"password,omitempty"`
	Role       string `json:"role"`
	ProjectIDs []int  `json:"projectIds"`
	AllowAuto  bool   `json:"allowAuto"`
	Enabled    bool   `json:"enabled"`
}

var usernamePattern = regexp.MustCompile(`^[\p{L}\p{N}_.-]{3,64}$`)

func ValidateUser(input UserInput, creating bool) error {
	if !usernamePattern.MatchString(input.Username) {
		return fmt.Errorf("账号需要3～64个字母、数字、中文或 ._-，不能含空格")
	}
	if input.Role != RoleAdmin && input.Role != RoleTester && input.Role != RoleViewer {
		return fmt.Errorf("请选择管理员、测试员或只读用户")
	}
	if creating && (len(input.Password) < 12 || len(input.Password) > 1024) {
		return fmt.Errorf("初始密码需要12～1024个字符")
	}
	if len(input.ProjectIDs) > 100 {
		return fmt.Errorf("一次最多分配100个项目")
	}
	for _, id := range input.ProjectIDs {
		if id <= 0 {
			return fmt.Errorf("项目编号不正确")
		}
	}
	return nil
}
func (s *UserStore) Save(ctx context.Context, input UserInput, creating bool) (User, error) {
	input.Username = strings.TrimSpace(input.Username)
	if err := ValidateUser(input, creating); err != nil {
		return User{}, err
	}
	var hash string
	var err error
	if creating {
		hash, err = HashPassword(input.Password)
		if err != nil {
			return User{}, err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "LOCK TABLE platform_users IN SHARE ROW EXCLUSIVE MODE"); err != nil {
		return User{}, err
	}
	if !creating {
		var oldRole string
		var oldEnabled bool
		if err = tx.QueryRowContext(ctx, "SELECT role,enabled FROM platform_users WHERE id=$1", input.ID).Scan(&oldRole, &oldEnabled); err != nil {
			return User{}, fmt.Errorf("用户不存在")
		}
		if oldRole == RoleAdmin && oldEnabled && (input.Role != RoleAdmin || !input.Enabled) {
			var count int
			err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM platform_users WHERE role=$1 AND enabled", RoleAdmin).Scan(&count)
			if err != nil {
				return User{}, err
			}
			if count <= 1 {
				return User{}, fmt.Errorf("至少保留一个启用的管理员，不能禁用或降级最后一个管理员")
			}
		}
	}
	input.AllowAuto = input.Role == RoleAdmin || (input.Role == RoleTester && input.AllowAuto)
	if creating {
		err = tx.QueryRowContext(ctx, "INSERT INTO platform_users(username,password_hash,role,allow_auto,enabled) VALUES($1,$2,$3,$4,$5) RETURNING id", input.Username, hash, input.Role, input.AllowAuto, input.Enabled).Scan(&input.ID)
	} else {
		_, err = tx.ExecContext(ctx, "UPDATE platform_users SET username=$2,role=$3,allow_auto=$4,enabled=$5,version=version+1 WHERE id=$1", input.ID, input.Username, input.Role, input.AllowAuto, input.Enabled)
	}
	if err != nil {
		return User{}, fmt.Errorf("保存用户失败，请检查账号是否重复")
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM platform_user_projects WHERE user_id=$1", input.ID); err != nil {
		return User{}, err
	}
	if input.Role != RoleAdmin {
		seen := map[int]bool{}
		for _, id := range input.ProjectIDs {
			if seen[id] {
				continue
			}
			seen[id] = true
			if _, err = tx.ExecContext(ctx, "INSERT INTO platform_user_projects(user_id,project_id) VALUES($1,$2)", input.ID, id); err != nil {
				return User{}, fmt.Errorf("分配的项目不存在，请刷新项目列表")
			}
		}
	}
	if err = tx.Commit(); err != nil {
		return User{}, err
	}
	return s.User(ctx, input.ID)
}
func (s *UserStore) Password(ctx context.Context, id int64, current, password string, reset bool) error {
	if len(password) < 12 || len(password) > 1024 {
		return fmt.Errorf("新密码需要12～1024个字符")
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var old string
	if err = tx.QueryRowContext(ctx, "SELECT password_hash FROM platform_users WHERE id=$1 FOR UPDATE", id).Scan(&old); err != nil {
		return fmt.Errorf("用户不存在")
	}
	if !reset && !CheckPassword(old, current) {
		return fmt.Errorf("当前密码不正确")
	}
	if _, err = tx.ExecContext(ctx, "UPDATE platform_users SET password_hash=$2,version=version+1 WHERE id=$1", id, hash); err != nil {
		return err
	}
	return tx.Commit()
}

type OperationAudit struct {
	ID         int64     `json:"id"`
	ActorID    int64     `json:"actorId"`
	Actor      string    `json:"actor"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	ProjectID  int       `json:"projectId"`
	ResourceID int64     `json:"resourceId"`
	Status     int       `json:"statusCode"`
	CreatedAt  time.Time `json:"createdAt"`
}

func (s *UserStore) Record(ctx context.Context, u User, path, method string, project int, resource int64, status int) error {
	id, err := s.BeginOperation(ctx, u, path, method, project)
	if err != nil {
		return err
	}
	return s.FinishOperation(ctx, id, resource, status)
}
func (s *UserStore) BeginOperation(ctx context.Context, u User, path, method string, project int) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, "INSERT INTO platform_operation_audits(actor_id,actor_name,path,method,project_id,status_code) VALUES(NULLIF($1,0),$2,$3,$4,NULLIF($5,0),0) RETURNING id", u.ID, u.Username, path, method, project).Scan(&id)
	return id, err
}
func (s *UserStore) FinishOperation(ctx context.Context, id, resource int64, status int) error {
	_, err := s.db.ExecContext(ctx, "UPDATE platform_operation_audits SET resource_id=$2,status_code=$3 WHERE id=$1", id, resource, status)
	return err
}
func (s *UserStore) Audits(ctx context.Context) ([]OperationAudit, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,COALESCE(actor_id,0),actor_name,method,path,COALESCE(project_id,0),resource_id,status_code,created_at FROM platform_operation_audits ORDER BY id DESC LIMIT 200")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OperationAudit{}
	for rows.Next() {
		var a OperationAudit
		if err = rows.Scan(&a.ID, &a.ActorID, &a.Actor, &a.Method, &a.Path, &a.ProjectID, &a.ResourceID, &a.Status, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func isRunPath(path string) bool {
	switch path {
	case "/api/runs", "/api/project-tests/runs", "/api/evaluation-suites/runs", "/api/evaluations/run", "/api/project-check":
		return true
	}
	return false
}
func (s *UserStore) AddActors(ctx context.Context, path string, project int, value any) any {
	if path == "/api/project-tests/export" {
		path = "/api/project-tests/runs"
	}
	if path == "/api/evaluations" {
		path = "/api/evaluations/run"
	}
	if path == "/api/project-check-reports" {
		path = "/api/project-check"
	}
	if !isRunPath(path) {
		return value
	}
	rows, err := s.db.QueryContext(ctx, "SELECT DISTINCT ON(resource_id) resource_id,actor_name FROM platform_operation_audits WHERE path=$1 AND method='POST' AND status_code BETWEEN 200 AND 299 AND resource_id>0 AND ($2=0 OR project_id=$2) ORDER BY resource_id,id", path, project)
	if err != nil {
		return value
	}
	defer rows.Close()
	actors := map[int64]string{}
	for rows.Next() {
		var id int64
		var name string
		if rows.Scan(&id, &name) == nil {
			actors[id] = name
		}
	}
	add := func(raw any) {
		if object, ok := raw.(map[string]any); ok {
			id, _ := number(object["id"])
			if name := actors[id]; name != "" {
				object["createdBy"] = name
			}
		}
	}
	switch root := value.(type) {
	case []any:
		for _, raw := range root {
			add(raw)
		}
	case map[string]any:
		if reports, ok := root["reports"].([]any); ok {
			for _, raw := range reports {
				add(raw)
			}
		} else {
			add(root)
		}
	}
	return value
}

func (a *Access) BindUsers(ctx context.Context, db *sql.DB) error {
	password, err := ReadSecret("PLATFORM_AUTH_PASSWORD")
	if err != nil {
		return err
	}
	store, err := NewUserStore(ctx, db, a.User, password)
	if err != nil {
		return err
	}
	a.users = store
	a.repository = store
	return nil
}
