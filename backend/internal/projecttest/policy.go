package projecttest

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
)

func normalizeCheckBaseURL(value string) (string, error) {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	u, err := url.Parse(value)
	if err != nil || u.Scheme == "" || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", errors.New("请填写可访问的 http:// 或 https:// 项目地址")
	}
	return value, nil
}

// sameCheckTarget keeps the automatic sweep inside the user-selected project.
// localhost, 127.0.0.1 and ::1 are all the same local machine, however OpenAPI
// documents frequently use one while a person types another. Treating that
// specific combination as equivalent avoids a false "external address" result.
func sameCheckTarget(base, endpoint *url.URL) bool {
	if strings.EqualFold(base.Host, endpoint.Host) {
		return true
	}
	if base.Port() != endpoint.Port() {
		return false
	}
	isLoopback := func(host string) bool {
		host = strings.Trim(strings.ToLower(host), "[]")
		if host == "localhost" {
			return true
		}
		ip := net.ParseIP(host)
		return ip != nil && ip.IsLoopback()
	}
	return isLoopback(base.Hostname()) && isLoopback(endpoint.Hostname())
}

func checkRisk(endpoint Endpoint, baseURL string) (string, string) {
	parsedBase, _ := url.Parse(baseURL)
	parsedEndpoint, err := url.Parse(endpoint.URL)
	if err == nil && parsedEndpoint.IsAbs() && !sameCheckTarget(parsedBase, parsedEndpoint) {
		return "外部地址", "接口地址不属于当前项目环境，安全巡检不会请求外部服务"
	}
	if endpoint.Method == http.MethodGet || endpoint.Method == http.MethodHead || endpoint.Method == http.MethodOptions {
		return "只读", "可在自动模式下发起无凭证连通探测"
	}
	label := strings.ToLower(endpoint.Method + " " + endpoint.URL + " " + endpoint.Name)
	for _, marker := range []string{"delete", "clear", "permanent", "batchdelete", "cancel", "logout", "changepassword", "remove"} {
		if strings.Contains(label, marker) {
			return "高风险写入", "可能删除、清空或改变真实业务数据，必须人工审核"
		}
	}
	return "写入", "POST、PUT、PATCH 等写操作需要人工审核后配置业务测试数据"
}

func checkRequestURL(endpointURL, baseURL string) string {
	endpointURL = strings.TrimSpace(endpointURL)
	if parsedEndpoint, err := url.Parse(endpointURL); err == nil && parsedEndpoint.IsAbs() {
		if parsedBase, baseErr := url.Parse(baseURL); baseErr == nil && sameCheckTarget(parsedBase, parsedEndpoint) {
			// Always use the exact address that the user selected. This matters for
			// local projects whose exported OpenAPI says 127.0.0.1 while the service
			// was started and exposed as localhost (or vice versa).
			parsedEndpoint.Scheme, parsedEndpoint.Host = parsedBase.Scheme, parsedBase.Host
		}
		return parsedEndpoint.String()
	}
	if strings.HasPrefix(endpointURL, "/") {
		return baseURL + endpointURL
	}
	return baseURL + "/" + endpointURL
}
