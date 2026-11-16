package security

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

type Policy struct {
	Allowed map[string]bool
	Public  bool
	lookup  func(context.Context, string) ([]net.IPAddr, error)
	dial    func(context.Context, string, string) (net.Conn, error)
}

var ErrTargetDenied = errors.New("目标地址被访问策略拒绝")

func denied(message string) error { return fmt.Errorf("%w：%s", ErrTargetDenied, message) }

func authority(u *url.URL) string {
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if host == "localhost" || (net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback()) {
		host = "loopback"
	}
	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	return strings.ToLower(u.Scheme) + "://" + net.JoinHostPort(host, port)
}

func NewPolicy(entries []string, public bool) (*Policy, error) {
	d := &net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}
	p := &Policy{Allowed: map[string]bool{}, Public: public, lookup: net.DefaultResolver.LookupIPAddr, dial: d.DialContext}
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		u, err := url.Parse(entry)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
			return nil, fmt.Errorf("允许的测试目标必须是 http/https 来源地址，不含路径或凭证")
		}
		p.Allowed[authority(u)] = true
	}
	return p, nil
}

func LoadPolicy(production bool, port string) (*Policy, error) {
	entries := strings.Split(os.Getenv("PLATFORM_ALLOWED_TARGETS"), ",")
	public := !production
	if value := strings.TrimSpace(os.Getenv("PLATFORM_ALLOW_PUBLIC_TARGETS")); value != "" {
		if value != "true" && value != "false" {
			return nil, fmt.Errorf("PLATFORM_ALLOW_PUBLIC_TARGETS 必须为 true 或 false")
		}
		public = value == "true"
	}
	if production && strings.TrimSpace(os.Getenv("PLATFORM_ALLOWED_TARGETS")) == "" {
		return nil, fmt.Errorf("正式环境必须配置 PLATFORM_ALLOWED_TARGETS")
	}
	if production && public {
		return nil, fmt.Errorf("正式环境必须按目标清单准入，不能开启任意公网请求")
	}
	// Only the built-in demo origin is automatically trusted in production.
	entries = append(entries, "http://localhost:"+port)
	if !production {
		entries = append(entries, "http://localhost:8081", "http://localhost:8082")
	}
	return NewPolicy(entries, public)
}

func (p *Policy) checkURL(u *url.URL) error {
	if u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || strings.Contains(u.Hostname(), "%") {
		return denied("测试目标只允许不含凭证的 http/https 地址")
	}
	if !p.Allowed[authority(u)] && !p.Public {
		return denied("目标地址未获准，请在服务器的 PLATFORM_ALLOWED_TARGETS 中配置该来源")
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil {
		return p.checkIP(u, ip)
	}
	return nil
}

func (p *Policy) checkIP(u *url.URL, ip net.IP) error {
	if ip == nil || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.Equal(net.ParseIP("100.100.100.200")) {
		return denied("禁止访问元数据或特殊网络地址")
	}
	loopbackTarget := strings.HasPrefix(authority(u), u.Scheme+"://loopback:")
	if loopbackTarget && !ip.IsLoopback() {
		return denied("回环目标解析到了非回环地址")
	}
	reserved := false
	for _, cidr := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32", "64:ff9b::/96", "64:ff9b:1::/48"} {
		_, block, _ := net.ParseCIDR(cidr)
		if block.Contains(ip) {
			reserved = true
			break
		}
	}
	if (ip.IsLoopback() || ip.IsPrivate() || reserved || !ip.IsGlobalUnicast()) && !p.Allowed[authority(u)] {
		return denied("内部网络目标需要显式授权")
	}
	return nil
}

type requestTargetKey struct{}
type guardedTransport struct {
	policy *Policy
	base   *http.Transport
}

func (t *guardedTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if err := t.policy.checkURL(r.URL); err != nil {
		return nil, err
	}
	return t.base.RoundTrip(r.Clone(context.WithValue(r.Context(), requestTargetKey{}, r.URL)))
}

func (p *Policy) Transport() http.RoundTripper {
	base := (&http.Transport{TLSHandshakeTimeout: 15 * time.Second, IdleConnTimeout: 90 * time.Second, MaxIdleConns: 100, ForceAttemptHTTP2: true})
	if defaultTransport, ok := http.DefaultTransport.(*http.Transport); ok {
		base = defaultTransport.Clone()
	}
	// Environment proxies must not bypass DNS/IP validation. Route the host via
	// the operator's VPN/network when the external provider requires a proxy.
	base.Proxy = nil
	base.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		u, ok := ctx.Value(requestTargetKey{}).(*url.URL)
		if !ok {
			return nil, fmt.Errorf("缺少已验证的目标上下文")
		}
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		if !strings.EqualFold(strings.TrimSuffix(host, "."), strings.TrimSuffix(u.Hostname(), ".")) {
			return nil, fmt.Errorf("连接地址与请求目标不一致")
		}
		ips, err := p.lookup(ctx, host)
		if err != nil {
			return nil, err
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("目标没有可用地址")
		}
		for _, item := range ips {
			if err := p.checkIP(u, item.IP); err != nil {
				return nil, err
			}
		}
		var last error
		for _, item := range ips {
			conn, err := p.dial(ctx, network, net.JoinHostPort(item.IP.String(), port))
			if err == nil {
				return conn, nil
			}
			last = err
		}
		return nil, last
	}
	return &guardedTransport{policy: p, base: base}
}

func NewClient(p *Policy, timeout time.Duration) *http.Client {
	return &http.Client{Transport: p.Transport(), Timeout: timeout, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("重定向过多")
		}
		if len(via) > 0 && authority(r.URL) != authority(via[0].URL) {
			return fmt.Errorf("目标跳转到其他来源，已停止；请使用最终地址并单独授权")
		}
		return p.checkURL(r.URL)
	}}
}

var current atomic.Pointer[Policy]

func SetPolicy(p *Policy) *Policy { return current.Swap(p) }
func HTTPClient(timeout time.Duration) *http.Client {
	p := current.Load()
	if p == nil {
		p, _ = NewPolicy([]string{"http://localhost:8080", "http://localhost:8081", "http://localhost:8082"}, true)
	}
	return NewClient(p, timeout)
}
