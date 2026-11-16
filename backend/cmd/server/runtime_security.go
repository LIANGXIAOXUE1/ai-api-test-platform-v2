package main

import (
	"ai-api-test-platform-v1/backend/internal/security"
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func databaseURL() (string, error) {
	if dsn, err := security.ReadSecret("DATABASE_URL"); err != nil || dsn != "" {
		return dsn, err
	}
	password, err := security.ReadSecret("POSTGRES_PASSWORD")
	if err != nil {
		return "", err
	}
	if password == "" {
		if os.Getenv("APP_ENV") == "production" {
			return "", fmt.Errorf("正式环境必须提供数据库凭证")
		}
		return defaultDatabaseURL, nil
	}
	u := &url.URL{Scheme: "postgres", Host: firstNonEmpty(os.Getenv("POSTGRES_HOST"), "db:5432"), Path: "/" + firstNonEmpty(os.Getenv("POSTGRES_DB"), "api_test"), User: url.UserPassword(firstNonEmpty(os.Getenv("POSTGRES_USER"), "api_user"), password)}
	u.RawQuery = "sslmode=" + firstNonEmpty(os.Getenv("POSTGRES_SSLMODE"), "disable")
	return u.String(), nil
}

// Migrate all keys in one transaction; an incorrect key never causes a partial
// rewrite or a silent empty credential. Legacy project-task encryption is kept.
func (s *store) migrateModelKeys(ctx context.Context) error {
	// Validate existing task capsules before sealing legacy model credentials.
	// A wrong key must not create new model ciphertext beside unreadable tasks.
	for _, table := range []string{"project_test_tasks", "project_test_runs"} {
		var exists bool
		if err := s.db.QueryRowContext(ctx, `SELECT to_regclass($1) IS NOT NULL`, table).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			continue
		}
		field := "secret"
		if table == "project_test_runs" {
			field = "private"
		}
		rows, err := s.db.QueryContext(ctx, `SELECT `+field+` FROM `+table+` WHERE `+field+` <> ''`)
		if err != nil {
			return err
		}
		for rows.Next() {
			var value string
			if err = rows.Scan(&value); err != nil {
				rows.Close()
				return err
			}
			if err = s.vault.CheckLegacy(value); err != nil {
				rows.Close()
				return err
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT id,api_key FROM model_profiles FOR UPDATE`)
	if err != nil {
		return err
	}
	type update struct {
		id    int
		value string
	}
	var updates []update
	for rows.Next() {
		var id int
		var value string
		if err = rows.Scan(&id, &value); err != nil {
			rows.Close()
			return err
		}
		if value == "" {
			continue
		}
		if strings.HasPrefix(value, "enc:v1:") {
			if _, err = s.vault.Unseal(value); err != nil {
				rows.Close()
				return fmt.Errorf("模型密钥检查失败：%w", err)
			}
			continue
		}
		sealed, sealErr := s.vault.Seal(value)
		if sealErr != nil {
			rows.Close()
			return sealErr
		}
		updates = append(updates, update{id, sealed})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, item := range updates {
		if _, err = tx.ExecContext(ctx, `UPDATE model_profiles SET api_key=$2 WHERE id=$1`, item.id, item.value); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *store) decodeModelKey(item *ModelProfile) error {
	var err error
	item.APIKey, err = s.vault.Unseal(item.APIKey)
	return err
}

func ensureExistingKey(db *sql.DB, directory string) error {
	if _, err := os.Stat(filepath.Join(directory, "project-test.key")); !os.IsNotExist(err) {
		return err
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM model_profiles WHERE api_key LIKE 'enc:v1:%'`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("已有模型密文，但原加密密钥缺失；请恢复备份中的 project-test.key")
	}
	for _, table := range []string{"project_test_tasks", "project_test_runs"} {
		var exists bool
		if err := db.QueryRow(`SELECT to_regclass($1) IS NOT NULL`, table).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			continue
		}
		field := "secret"
		if table == "project_test_runs" {
			field = "private"
		}
		if err := db.QueryRow(`SELECT COUNT(*) FROM ` + table + ` WHERE ` + field + ` <> ''`).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			return fmt.Errorf("已有项目任务密文，但原加密密钥缺失；请恢复备份中的 project-test.key")
		}
	}
	return nil
}

func (s *server) agentFailure(w http.ResponseWriter, phase string, err error) {
	s.llmMu.RLock()
	key := s.llmAPIKey
	s.llmMu.RUnlock()
	message := err.Error()
	if key != "" {
		message = strings.ReplaceAll(message, key, "••••••")
	}
	if len(message) > 1200 {
		message = message[:1200]
	}
	writeJSON(w, http.StatusBadGateway, map[string]any{"error": phase + "失败：" + message + "。本次未生成草稿，请检查模型连接后重试。", "code": "assistant_model_failed", "retryable": true, "mode": "llm-error"})
}

func runVerdict(assertions []Assertion, status int, body string, readErr error) (bool, int, int, string, string) {
	pass, total := evaluateAssertions(assertions, status, body)
	explicitStatus := false
	for _, a := range assertions {
		if a.Type == "status" {
			explicitStatus = true
		}
	}
	if readErr != nil {
		category, message := classifyHTTP(status, body, readErr)
		return false, pass, total, category, message
	}
	if !explicitStatus && (status < 200 || status >= 300) {
		category, message := classifyHTTP(status, body, nil)
		return false, pass, total, category, message
	}
	if pass != total {
		return false, pass, total, "断言失败", fmt.Sprintf("%d/%d 条断言通过，请核对预期状态码与业务字段", pass, total)
	}
	return true, pass, total, "", ""
}

func safeProviderError(value, key string) string {
	value = strings.TrimSpace(value)
	if key != "" {
		value = strings.ReplaceAll(value, key, "••••••")
	}
	if len(value) > 1200 {
		value = value[:1200]
	}
	return value
}
