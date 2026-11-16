package security

import (
	"regexp"
	"strings"
)

var secretField = regexp.MustCompile(`(?i)(^|[._-])(password|passwd|secret|token|authorization|cookie)([._-]|$)|^(api[-_]?key|apiKeyMasked|accessToken|refreshToken|authToken|cloudDiskToken|cloud_disk_token|passwordHash|currentPassword)$`)
var privateField = regexp.MustCompile(`(?i)^(request|response|requestBody|responseBody|body|requestHeaders|responseHeaders|headers|variables|initialVariables|extracted|fileData|private|markdown)$`)

// View-only users receive checks, state and timing, never raw requests or
// responses that may contain arbitrary credentials. No keyword guess promises
// to detect every secret in an unstructured response body.
func Sanitize(value any, viewer bool) any {
	switch v := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, child := range v {
			if secretField.MatchString(key) {
				if _, isBool := child.(bool); isBool {
					out[key] = child
				} else {
					out[key] = "[已脱敏]"
				}
				continue
			}
			if viewer && privateField.MatchString(key) {
				if _, isMap := child.(map[string]any); isMap {
					out[key] = map[string]any{}
				} else {
					out[key] = "[只读账号不显示原始内容]"
				}
				continue
			}
			if viewer && (key == "errorMessage" || key == "requestBody" || key == "error") && child != "" {
				out[key] = "请求未完成，请由测试员查看原始错误详情"
				continue
			}
			if viewer && key == "checks" {
				out[key] = sanitizeChecks(child)
				continue
			}
			if key == "apiKeyMasked" || strings.EqualFold(key, "apiKey") {
				out[key] = ""
				continue
			}
			out[key] = Sanitize(child, viewer)
		}
		return out
	case []any:
		out := []any{}
		for _, child := range v {
			out = append(out, Sanitize(child, viewer))
		}
		return out
	default:
		return value
	}
}
func sanitizeChecks(value any) any {
	items, ok := value.([]any)
	if !ok {
		return value
	}
	out := []any{}
	for _, child := range items {
		item, ok := child.(map[string]any)
		if !ok {
			continue
		}
		copy := Sanitize(item, true).(map[string]any)
		path, _ := item["path"].(string)
		if secretField.MatchString(path) {
			copy["actual"] = "[已脱敏]"
			copy["expected"] = "[已脱敏]"
		}
		out = append(out, copy)
	}
	return out
}
