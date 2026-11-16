package security

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestTargetRestrictions(t *testing.T) {
	p, _ := NewPolicy([]string{"http://localhost:8082", "https://provider.example"}, false)
	for _, tc := range []struct {
		url   string
		allow bool
	}{
		{"http://127.0.0.1:8082/login", true}, {"https://provider.example/v1", true},
		{"http://localhost:5432", false}, {"https://other.example", false}, {"file:///etc/passwd", false},
		{"https://user:password@provider.example", false}, {"http://169.254.169.254/latest", false},
	} {
		u, _ := url.Parse(tc.url)
		if (p.checkURL(u) == nil) != tc.allow {
			t.Errorf("target %s unexpected outcome", tc.url)
		}
	}
	public, _ := NewPolicy(nil, true)
	for _, ip := range []string{"127.0.0.1", "10.1.2.3", "169.254.169.254", "100.100.100.200", "100.64.0.1", "198.18.0.1", "::1", "fe80::1", "64:ff9b::a00:1"} {
		u, _ := url.Parse("https://provider.example")
		if public.checkIP(u, net.ParseIP(ip)) == nil {
			t.Errorf("unexpected address accepted: %s", ip)
		}
	}
}

func TestDNSCheckedBeforeConnect(t *testing.T) {
	p, _ := NewPolicy(nil, true)
	p.lookup = func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}, {IP: net.ParseIP("127.0.0.1")}}, nil
	}
	dialed := false
	p.dial = func(context.Context, string, string) (net.Conn, error) { dialed = true; return nil, nil }
	_, err := NewClient(p, time.Second).Get("https://provider.example/test")
	if err == nil || dialed {
		t.Fatal("mixed public/private DNS reached connection")
	}
}

func TestRedirectNeverLeaksCredentials(t *testing.T) {
	called := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 302) }))
	defer source.Close()
	p, _ := NewPolicy([]string{source.URL, target.URL}, false)
	req, _ := http.NewRequest("GET", source.URL, nil)
	req.Header.Set("api-key", "private-test-key")
	_, err := NewClient(p, time.Second).Do(req)
	if err == nil || called {
		t.Fatal("cross-origin redirected credential")
	}
}

func TestSameOriginRedirectAndProxyBypass(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/ok", 302)
			return
		}
		io.WriteString(w, "ok")
	}))
	defer s.Close()
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	p, _ := NewPolicy([]string{s.URL}, false)
	r, err := NewClient(p, time.Second).Get(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	if string(b) != "ok" {
		t.Fatal("same-origin redirect failed")
	}
}
