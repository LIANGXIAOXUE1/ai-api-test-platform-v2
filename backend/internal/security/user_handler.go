package security

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
)

type userContextKey struct{}

func WithUser(ctx context.Context, user User) context.Context {
	return context.WithValue(ctx, userContextKey{}, user)
}
func CurrentUser(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(userContextKey{}).(User)
	return u, ok
}
func userRead(w http.ResponseWriter, r *http.Request, dst any) error {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (a *Access) serveUsers(w http.ResponseWriter, r *http.Request, user User) bool {
	p := r.URL.Path
	if p != "/api/admin/users" && p != "/api/admin/users/reset-password" && p != "/api/admin/audit" && p != "/api/auth/password" {
		return false
	}
	if a.users == nil || !a.Enabled {
		authJSON(w, 409, map[string]string{"error": "请开启平台登录后使用用户管理"})
		return true
	}
	if p != "/api/auth/password" && user.Role != RoleAdmin {
		_ = a.users.Record(r.Context(), user, p, r.Method, 0, 0, 403)
		authJSON(w, 403, map[string]string{"error": "需要管理员权限"})
		return true
	}
	status := 200
	resource := int64(0)
	var auditID int64
	if r.Method != "GET" {
		var err error
		auditID, err = a.users.BeginOperation(r.Context(), user, p, r.Method, 0)
		if err != nil {
			authJSON(w, 503, map[string]string{"error": "操作日志服务暂时不可用，本次操作未执行"})
			return true
		}
	}
	defer func() {
		if r.Method != "GET" {
			_ = a.users.FinishOperation(r.Context(), auditID, resource, status)
		}
	}()
	fail := func(code int, message string) { status = code; authJSON(w, code, map[string]string{"error": message}) }
	if p == "/api/admin/audit" {
		if r.Method != "GET" {
			fail(405, "请求方法不正确")
			return true
		}
		items, err := a.users.Audits(r.Context())
		if err != nil {
			fail(503, "操作日志暂时不可用")
		} else {
			authJSON(w, 200, items)
		}
		return true
	}
	if p == "/api/admin/users" {
		if r.Method == "GET" {
			items, err := a.users.Users(r.Context())
			if err != nil {
				fail(503, "用户列表暂时不可用")
			} else {
				authJSON(w, 200, items)
			}
			return true
		}
		if r.Method != "POST" && r.Method != "PUT" {
			fail(405, "请使用新建或编辑用户，不支持删除账号")
			return true
		}
		var input UserInput
		if userRead(w, r, &input) != nil {
			fail(400, "用户配置格式不正确")
			return true
		}
		if r.Method == "PUT" && (input.ID <= 0 || input.Password != "") {
			fail(400, "编辑用户需要有效编号；密码请通过重置功能修改")
			return true
		}
		u, err := a.users.Save(r.Context(), input, r.Method == "POST")
		if err != nil {
			fail(400, err.Error())
			return true
		}
		resource = u.ID
		if r.Method == "POST" {
			status = 201
		}
		authJSON(w, status, u)
		return true
	}
	if r.Method != "POST" {
		fail(405, "请求方法不正确")
		return true
	}
	var input struct {
		ID              int64  `json:"id"`
		CurrentPassword string `json:"currentPassword"`
		Password        string `json:"password"`
	}
	if userRead(w, r, &input) != nil {
		fail(400, "密码请求格式不正确")
		return true
	}
	reset := p == "/api/admin/users/reset-password"
	if !reset {
		input.ID = user.ID
	}
	resource = input.ID
	if input.ID <= 0 {
		fail(400, "用户编号不正确")
		return true
	}
	if err := a.users.Password(r.Context(), input.ID, input.CurrentPassword, input.Password, reset); err != nil {
		fail(400, err.Error())
		return true
	}
	if input.ID == user.ID {
		a.revokeSession(r)
		a.writeSessionCookie(w, r, "", -1)
	}
	authJSON(w, 200, map[string]any{"updated": true, "reauthenticate": input.ID == user.ID})
	return true
}
