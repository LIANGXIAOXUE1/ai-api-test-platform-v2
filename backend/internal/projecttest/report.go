package projecttest

import (
	"encoding/json"
	"fmt"
	"strings"
)

func businessCode(responseBody string) string {
	var value struct {
		Code any `json:"code"`
	}
	if json.Unmarshal([]byte(responseBody), &value) != nil || value.Code == nil {
		return ""
	}
	return fmt.Sprint(value.Code)
}

func projectCheckMarkdown(output Report) string {
	var document strings.Builder
	fmt.Fprintln(&document, "# 项目接口联通性报告")
	fmt.Fprintf(&document, "\n- 生成时间：%s\n", output.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&document, "- 项目地址：`%s`\n", output.BaseURL)
	if strings.TrimSpace(output.Task) != "" {
		fmt.Fprintf(&document, "- 测试任务：%s\n", strings.TrimSpace(output.Task))
	}
	fmt.Fprintf(&document, "- 执行权限：%s\n", map[string]string{"review": "人工审核", "auto": "安全自动执行"}[output.Permission])
	fmt.Fprintf(&document, "- 统计：共 %d 个接口；已联通 %d；不可达 %d；待审核 %d。\n\n", output.Summary.Total, output.Summary.Reachable, output.Summary.Unavailable, output.Summary.Pending)
	fmt.Fprintln(&document, "| 接口 | 方法 | 风险 | 结果 | HTTP | 业务码 | 说明 |")
	fmt.Fprintln(&document, "| --- | --- | --- | --- | ---: | ---: | --- |")
	for _, item := range output.Items {
		result := "待审核"
		if item.Reachable {
			result = "已联通"
		} else if item.Decision == "已执行" {
			result = "不可达"
		}
		note := strings.ReplaceAll(firstNonEmpty(item.Error, item.Reason, "—"), "|", "\\|")
		name := strings.ReplaceAll(item.Name, "|", "\\|")
		fmt.Fprintf(&document, "| %s | %s | %s | %s | %d | %s | %s |\n", name, item.Method, item.Risk, result, item.StatusCode, firstNonEmpty(item.BusinessCode, "—"), note)
	}
	fmt.Fprintln(&document, "\n> 说明：联通性巡检不携带 Token、API Key 或请求体。它证明路由与服务可到达；需要账号、文件、上传 ID 或真实业务数据的功能性回归，应在人工审核后作为业务链路执行。")
	return document.String()
}

func firstNonEmpty(items ...string) string {
	for _, item := range items {
		if strings.TrimSpace(item) != "" {
			return item
		}
	}
	return ""
}
