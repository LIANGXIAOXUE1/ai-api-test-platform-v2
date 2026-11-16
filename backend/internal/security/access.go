package security

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// Access is a single-owner deployment gate, not a multi-tenant user system.
type Access struct {
	Production bool
	User       string
	password   [32]byte
	Enabled    bool
	Origins    map[string]bool
}

func ReadSecret(name string) (string, error) {
	if file := strings.TrimSpace(os.Getenv(name + "_FILE")); file != "" {
		value, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("无法读取 %s_FILE", name)
		}
		return strings.TrimSpace(string(value)), nil
	}
	return strings.TrimSpace(os.Getenv(name)), nil
}

func LoadAccess() (*Access, error) {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	if mode != "" && mode != "development" && mode != "production" {
		return nil, fmt.Errorf("APP_ENV 只能是 development 或 production")
	}
	a := &Access{Production: mode == "production", User: strings.TrimSpace(os.Getenv("PLATFORM_AUTH_USER")), Origins: map[string]bool{}}
	password, err := ReadSecret("PLATFORM_AUTH_PASSWORD")
	if err != nil {
		return nil, err
	}
	if (a.User == "") != (password == "") {
		return nil, fmt.Errorf("平台登录用户名与密码必须同时配置")
	}
	a.Enabled = a.User != ""
	a.password = sha256.Sum256([]byte(password))
	if a.Production && (!a.Enabled || len(password) < 16) {
		return nil, fmt.Errorf("正式环境必须配置平台登录，密码至少16个字符")
	}
	for _, origin := range strings.Split(os.Getenv("PLATFORM_ALLOWED_ORIGINS"), ",") {
		origin = strings.TrimRight(strings.TrimSpace(origin), "/")
		if origin == "" {
			continue
		}
		u, err := url.Parse(origin)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
			return nil, fmt.Errorf("PLATFORM_ALLOWED_ORIGINS 必须是完整来源地址")
		}
		a.Origins[origin] = true
	}
	return a, nil
}

func (a *Access) ListenAddress(port string) (string, error) {
	host := strings.TrimSpace(os.Getenv("LISTEN_HOST"))
	if host == "" {
		host = "127.0.0.1"
	}
	ip := net.ParseIP(host)
	if !a.Enabled && (ip == nil || !ip.IsLoopback()) {
		return "", fmt.Errorf("未开启平台登录时，只能监听回环地址")
	}
	return net.JoinHostPort(host, port), nil
}

func (a *Access) authenticated(r *http.Request) bool {
	if !a.Enabled {
		return true
	}
	user, password, ok := r.BasicAuth()
	userHash, expectedUser := sha256.Sum256([]byte(user)), sha256.Sum256([]byte(a.User))
	passwordHash := sha256.Sum256([]byte(password))
	return ok && subtle.ConstantTimeCompare(userHash[:], expectedUser[:]) == 1 && subtle.ConstantTimeCompare(passwordHash[:], a.password[:]) == 1
}

func (a *Access) allowedOrigin(r *http.Request) bool {
	origin := strings.TrimRight(r.Header.Get("Origin"), "/")
	if origin == "" {
		return r.Header.Get("Sec-Fetch-Site") != "cross-site"
	}
	if a.Origins[origin] {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && u.User == nil && u.Path == "" && (u.Scheme == "http" || u.Scheme == "https") && strings.EqualFold(u.Host, r.Host)
}

func (a *Access) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Vary", "Origin")
		if !a.allowedOrigin(r) {
			http.Error(w, "不允许的请求来源", http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.URL.Path == "/api/health" && r.Method == http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}
		// A server-side test of the harmless built-in examples must retain its
		// target Bearer header. Only numeric loopback connections can bypass the
		// platform login here; the HTTPS frontend still protects /api/demo/.
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() && strings.HasPrefix(r.URL.Path, "/api/demo/") {
			next.ServeHTTP(w, r)
			return
		}
		if !a.authenticated(r) {
			w.Header().Set("WWW-Authenticate", `Basic realm="Pulse API", charset="UTF-8"`)
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"请先登录测试平台"}`))
			return
		}
		if r.URL.Path == "/api/access" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
