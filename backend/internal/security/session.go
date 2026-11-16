package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const sessionCookie = "pulse_session"
const sessionLifetime = 8 * time.Hour
const loginWindow = time.Minute
const maxAuthEntries = 1024

type loginAttempt struct {
	count int
	until time.Time
}

func sessionKey(r *http.Request) ([32]byte, bool) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || len(cookie.Value) != 43 {
		return [32]byte{}, false
	}
	return sha256.Sum256([]byte(cookie.Value)), true
}

func (a *Access) sessionAuthenticated(r *http.Request) bool {
	key, ok := sessionKey(r)
	if !ok {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	expires, ok := a.sessions[key]
	if ok && !time.Now().Before(expires) {
		delete(a.sessions, key)
		return false
	}
	return ok
}

func authJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (a *Access) writeSessionCookie(w http.ResponseWriter, r *http.Request, value string, maxAge int) {
	cookie := &http.Cookie{Name: sessionCookie, Value: value, Path: "/", HttpOnly: true,
		Secure: a.Production || r.TLS != nil, SameSite: http.SameSiteStrictMode, MaxAge: maxAge}
	if maxAge < 0 {
		cookie.Expires = time.Unix(1, 0)
	}
	http.SetCookie(w, cookie)
}

func (a *Access) revokeSession(r *http.Request) {
	if key, ok := sessionKey(r); ok {
		a.mu.Lock()
		delete(a.sessions, key)
		a.mu.Unlock()
	}
}

// Attempts use the actual peer address, never a caller-controlled forwarded
// header. Behind a reverse proxy the allowance is shared by that proxy.
func (a *Access) allowLogin(r *http.Request) bool {
	peer, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		peer = r.RemoteAddr
	}
	now := time.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.attempts == nil {
		a.attempts = make(map[string]loginAttempt)
	}
	for key, entry := range a.attempts {
		if !now.Before(entry.until) {
			delete(a.attempts, key)
		}
	}
	entry, exists := a.attempts[peer]
	if entry.count >= 10 || (!exists && len(a.attempts) >= maxAuthEntries) {
		return false
	}
	if !exists {
		entry.until = now.Add(loginWindow)
	}
	entry.count++
	a.attempts[peer] = entry
	return true
}

func (a *Access) serveAuth(w http.ResponseWriter, r *http.Request) bool {
	path := r.URL.Path
	if path != "/api/auth/session" && path != "/api/auth/login" && path != "/api/auth/logout" {
		return false
	}
	method := http.MethodPost
	if path == "/api/auth/session" {
		method = http.MethodGet
	}
	if r.Method != method {
		w.Header().Set("Allow", method)
		authJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "请求方法不正确"})
		return true
	}
	if path == "/api/auth/session" {
		loggedIn := !a.Enabled || a.sessionAuthenticated(r)
		var user any
		if a.Enabled && loggedIn {
			user = map[string]string{"username": a.User, "role": "administrator"}
		}
		authJSON(w, http.StatusOK, map[string]any{"authenticationRequired": a.Enabled, "authenticated": loggedIn, "user": user})
		return true
	}
	if path == "/api/auth/logout" {
		a.revokeSession(r)
		a.writeSessionCookie(w, r, "", -1)
		authJSON(w, http.StatusOK, map[string]bool{"authenticated": false})
		return true
	}
	if !a.Enabled {
		authJSON(w, http.StatusOK, map[string]bool{"authenticated": true})
		return true
	}
	if !a.allowLogin(r) {
		w.Header().Set("Retry-After", "60")
		authJSON(w, http.StatusTooManyRequests, map[string]string{"error": "登录尝试过于频繁，请等待1分钟后重试"})
		return true
	}
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		authJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "请使用登录表单提交"})
		return true
	}
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		authJSON(w, http.StatusBadRequest, map[string]string{"error": "请填写有效的账号和密码"})
		return true
	}
	if decoder.Decode(new(any)) != io.EOF {
		authJSON(w, http.StatusBadRequest, map[string]string{"error": "登录请求格式不正确"})
		return true
	}
	if !a.validCredentials(strings.TrimSpace(input.Username), input.Password) {
		authJSON(w, http.StatusUnauthorized, map[string]string{"error": "账号或密码不正确，请检查后重试"})
		return true
	}
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		authJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "暂时无法建立登录会话，请重试"})
		return true
	}
	token := base64.RawURLEncoding.EncodeToString(bytes)
	now := time.Now()
	a.mu.Lock()
	if a.sessions == nil {
		a.sessions = make(map[[32]byte]time.Time)
	}
	for key, expires := range a.sessions {
		if !now.Before(expires) {
			delete(a.sessions, key)
		}
	}
	if key, ok := sessionKey(r); ok {
		delete(a.sessions, key)
	}
	if len(a.sessions) >= maxAuthEntries {
		a.mu.Unlock()
		authJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "登录会话已达上限，请稍后重试"})
		return true
	}
	a.sessions[sha256.Sum256([]byte(token))] = now.Add(sessionLifetime)
	a.mu.Unlock()
	a.writeSessionCookie(w, r, token, int(sessionLifetime.Seconds()))
	authJSON(w, http.StatusOK, map[string]any{"authenticated": true, "user": map[string]string{"username": a.User, "role": "administrator"}})
	return true
}
