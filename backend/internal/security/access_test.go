package security

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAccessGate(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("PLATFORM_AUTH_USER", "owner")
	t.Setenv("PLATFORM_AUTH_PASSWORD", "testing-password-123456")
	t.Setenv("PLATFORM_AUTH_PASSWORD_FILE", "")
	t.Setenv("PLATFORM_ALLOWED_ORIGINS", "https://platform.example")
	a, err := LoadAccess()
	if err != nil {
		t.Fatal(err)
	}
	h := a.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	for _, tc := range []struct {
		name, path, origin, remote, password string
		want                                 int
	}{
		{"anonymous api", "/api/projects", "", "10.0.0.2:1234", "", 401},
		{"incorrect password", "/api/runs", "", "10.0.0.2:1234", "bad", 401},
		{"logged in", "/api/projects", "https://platform.example", "10.0.0.2:1234", "testing-password-123456", 200},
		{"external site", "/api/runs", "https://evil.example", "10.0.0.2:1234", "testing-password-123456", 403},
		{"access probe", "/api/access", "", "10.0.0.2:1234", "testing-password-123456", 204},
		{"minimal health", "/api/health", "", "10.0.0.2:1234", "", 200},
		{"demo remotely blocked", "/api/demo/auth", "", "10.0.0.2:1234", "", 401},
		{"demo loopback permitted", "/api/demo/auth", "", "127.0.0.1:1234", "", 200},
		{"loopback management protected", "/api/models", "", "127.0.0.1:1234", "", 401},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", tc.path, nil)
			r.RemoteAddr = tc.remote
			r.Header.Set("Origin", tc.origin)
			if tc.password != "" {
				r.SetBasicAuth("owner", tc.password)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("status %d want %d", w.Code, tc.want)
			}
			if w.Header().Get("Access-Control-Allow-Origin") == "*" {
				t.Fatal("wildcard CORS")
			}
		})
	}
}

func TestProductionCannotStartUnprotected(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("PLATFORM_AUTH_USER", "")
	t.Setenv("PLATFORM_AUTH_PASSWORD", "")
	t.Setenv("PLATFORM_AUTH_PASSWORD_FILE", "")
	if _, err := LoadAccess(); err == nil {
		t.Fatal("unprotected production accepted")
	}
	t.Setenv("LISTEN_HOST", "0.0.0.0")
	if _, err := (&Access{}).ListenAddress("8080"); err == nil {
		t.Fatal("unprotected public binding accepted")
	}
}
