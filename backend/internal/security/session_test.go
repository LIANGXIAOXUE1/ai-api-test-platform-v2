package security

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testSessionAccess(t *testing.T) (*Access, http.Handler) {
	t.Helper()
	t.Setenv("APP_ENV", "development")
	t.Setenv("PLATFORM_AUTH_USER", "owner")
	t.Setenv("PLATFORM_AUTH_PASSWORD", "testing-password-123456")
	t.Setenv("PLATFORM_AUTH_PASSWORD_FILE", "")
	t.Setenv("PLATFORM_ALLOWED_ORIGINS", "")
	a, err := LoadAccess()
	if err != nil {
		t.Fatal(err)
	}
	return a, a.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
}

func authCall(h http.Handler, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://platform.example"+path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Sec-Fetch-Mode", "cors")
	r.RemoteAddr = "10.0.0.2:1234"
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func loginCookie(t *testing.T, h http.Handler, previous *http.Cookie) *http.Cookie {
	t.Helper()
	w := authCall(h, "POST", "/api/auth/login", `{"username":"owner","password":"testing-password-123456"}`, previous)
	if w.Code != 200 {
		t.Fatalf("login %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "testing-password") {
		t.Fatal("password disclosed")
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies=%d", len(cookies))
	}
	c := cookies[0]
	if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Path != "/" || c.MaxAge != 28800 || len(c.Value) != 43 {
		t.Fatal("insecure or malformed session cookie")
	}
	return c
}

func TestSessionLoginLogoutAndExpiry(t *testing.T) {
	a, h := testSessionAccess(t)
	for _, path := range []string{"/api/projects", "/api/models", "/api/runs"} {
		w := authCall(h, "GET", path, "", nil)
		if w.Code != 401 || w.Header().Get("WWW-Authenticate") != "" {
			t.Fatal("anonymous access or native challenge")
		}
	}
	w := authCall(h, "POST", "/api/auth/login", `{"username":"owner","password":"wrong"}`, nil)
	if w.Code != 401 || len(w.Result().Cookies()) != 0 {
		t.Fatal("bad credentials accepted")
	}
	c := loginCookie(t, h, nil)
	if c.Secure {
		t.Fatal("local HTTP session marked Secure")
	}
	if authCall(h, "GET", "/api/projects", "", c).Code != 200 {
		t.Fatal("cookie not accepted")
	}
	var state struct {
		Authenticated bool
		User          struct{ Role string }
	}
	_ = json.Unmarshal(authCall(h, "GET", "/api/auth/session", "", c).Body.Bytes(), &state)
	if !state.Authenticated || state.User.Role != "administrator" {
		t.Fatal("invalid session state")
	}
	rotated := loginCookie(t, h, c)
	if rotated.Value == c.Value || authCall(h, "GET", "/api/projects", "", c).Code != 401 {
		t.Fatal("old session not rotated")
	}
	w = authCall(h, "POST", "/api/auth/logout", `{}`, rotated)
	if w.Code != 200 || w.Result().Cookies()[0].MaxAge != -1 {
		t.Fatal("logout did not clear cookie")
	}
	if authCall(h, "GET", "/api/projects", "", rotated).Code != 401 {
		t.Fatal("logged-out token replay accepted")
	}
	c = loginCookie(t, h, nil)
	key, _ := sessionKey((&http.Request{Header: http.Header{"Cookie": []string{c.String()}}}))
	a.mu.Lock()
	a.sessions[key] = time.Now().Add(-time.Second)
	a.mu.Unlock()
	if authCall(h, "GET", "/api/projects", "", c).Code != 401 {
		t.Fatal("expired token accepted")
	}
	_, fresh := testSessionAccess(t)
	if authCall(fresh, "GET", "/api/projects", "", c).Code != 401 {
		t.Fatal("token survived server restart")
	}
}

func TestSessionOriginAndBasicCompatibility(t *testing.T) {
	_, h := testSessionAccess(t)
	c := loginCookie(t, h, nil)
	for _, path := range []string{"/api/auth/login", "/api/auth/logout", "/api/projects"} {
		r := httptest.NewRequest("POST", "http://platform.example"+path, strings.NewReader(`{}`))
		r.AddCookie(c)
		r.Header.Set("Origin", "https://evil.example")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal("cross-origin mutation accepted")
		}
		r.Header.Del("Origin")
		r.Header.Set("Sec-Fetch-Site", "cross-site")
		w = httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal("fetch metadata not enforced")
		}
	}
	for _, browser := range []bool{false, true} {
		r := httptest.NewRequest("GET", "/api/projects", nil)
		r.SetBasicAuth("owner", "testing-password-123456")
		if browser {
			r.Header.Set("Sec-Fetch-Mode", "cors")
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		want := 200
		if browser {
			want = 401
		}
		if w.Code != want {
			t.Fatal("CLI compatibility or cached browser Basic failed")
		}
		r.URL.Path = "/api/auth/session"
		w = httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if strings.Contains(w.Body.String(), `"authenticated":true`) {
			t.Fatal("session probe trusted Basic")
		}
	}
}

func TestSessionLimitsAndValidation(t *testing.T) {
	a, h := testSessionAccess(t)
	for i := 0; i < 10; i++ {
		if authCall(h, "POST", "/api/auth/login", `{"username":"owner","password":"wrong"}`, nil).Code != 401 {
			t.Fatal("unexpected retry")
		}
	}
	if authCall(h, "POST", "/api/auth/login", `{}`, nil).Code != 429 {
		t.Fatal("rate limit absent")
	}
	a.mu.Lock()
	a.attempts["10.0.0.2"] = loginAttempt{count: 10, until: time.Now().Add(-time.Second)}
	a.mu.Unlock()
	c := loginCookie(t, h, nil)
	a.Production = true
	if !loginCookie(t, h, c).Secure {
		t.Fatal("production session requires Secure")
	}
	for _, body := range []string{`{"username":"owner","password":"testing-password-123456","extra":true}`, `{}`, `{bad`, strings.Repeat("x", 5000), `{} {}`} {
		if authCall(h, "POST", "/api/auth/login", body, nil).Code == 200 {
			t.Fatal("invalid body accepted")
		}
	}
	if authCall(h, "GET", "/api/auth/logout", "", nil).Code != 405 {
		t.Fatal("logout GET allowed")
	}
	if authCall(h, "POST", "/api/auth/session", "", nil).Code != 405 {
		t.Fatal("session POST allowed")
	}
}

func TestSessionOptionalLocalMode(t *testing.T) {
	a := &Access{}
	h := a.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	if !strings.Contains(authCall(h, "GET", "/api/auth/session", "", nil).Body.String(), `"authenticationRequired":false`) {
		t.Fatal("optional local mode broken")
	}
	if authCall(h, "GET", "/api/projects", "", nil).Code != 200 {
		t.Fatal("local gate enabled unexpectedly")
	}
}
