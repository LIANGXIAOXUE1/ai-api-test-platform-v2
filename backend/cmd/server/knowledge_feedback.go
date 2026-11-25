package main

import (
	"net/http"
	"strings"
)

type documentAssessment struct {
	Status      string   `json:"status"`
	Reason      string   `json:"reason"`
	Suggestions []string `json:"suggestions"`
}

func knowledgeFailure(w http.ResponseWriter, httpStatus int, phase, code, message string, reasons, steps []string) {
	status := "cannot_generate"
	if phase == "upload" {
		status = "upload_failed"
	}
	writeJSON(w, httpStatus, map[string]any{"status": status, "phase": phase, "code": code, "error": message, "message": message, "missingInformation": reasons, "nextSteps": steps, "cases": []BusinessTestCase{}})
}

func knowledgeGenerationFeedback(result businessCasesResult) (status, code, message string, nextSteps []string, ready, drafts int) {
	for _, item := range result.Cases {
		if item.Status == "ready_for_review" {
			ready++
		} else {
			drafts++
		}
	}
	if len(result.Cases) == 0 {
		if result.InvalidCaseCount > 0 {
			return "cannot_generate", "knowledge_model_case_invalid", "无法生成可审核请求：模型返回的用例格式未通过校验", []string{"重新生成；若仍失败，切换另一已验证模型", "检查提示词中的输出要求是否与用例结构一致，文档已上传的数据仍会保留"}, 0, 0
		}
		return "needs_information", "knowledge_missing_expectations", "暂时无法生成测试用例：资料中缺少明确的业务规则、预期结果，或与所选接口不匹配", []string{"查看文档提取内容，补充要测试的功能、前置条件与预期结果", "确认接口范围属于当前业务；执行用例需要先导入对应OpenAPI", "补充后点击继续生成"}, 0, 0
	}
	if ready == 0 {
		return "drafts_only", "knowledge_no_executable_cases", "已生成业务草稿，但当前没有可执行用例：接口、请求数据或断言尚未确认", []string{"查看每条草稿的待补充项", "导入对应接口，填写测试数据及预期后，选择可执行用例重新生成"}, ready, drafts
	}
	if drafts > 0 || len(result.MissingInformation) > 0 {
		return "partial_generated", "knowledge_partial_generation", "仅部分用例已具备可审核请求；其余内容仍需补充配置", []string{"可以先核对并保存可审核请求", "按下面说明补齐资料，再继续生成剩余用例"}, ready, drafts
	}
	return "generated", "knowledge_generated", "测试用例已生成，请核对业务预期、请求数据和断言后保存", nil, ready, drafts
}

func businessMaterialStatus(assessment documentAssessment) bool {
	return assessment.Status == "usable" || assessment.Status == "insufficient" || assessment.Status == "irrelevant"
}
func assessmentProblem(assessment documentAssessment) (code, message string) {
	if assessment.Status == "irrelevant" {
		return "knowledge_document_not_relevant", "无法生成测试用例：文档内容与当前测试目标无关"
	}
	if assessment.Status == "insufficient" {
		return "knowledge_document_has_no_rules", "无法生成测试用例：文档没有提供可验证的业务规则或预期行为"
	}
	return "knowledge_invalid_model_output", "无法完成用例生成：模型没有返回有效的资料适用性判断，请重试或更换已验证模型"
}
func assessmentReasons(assessment documentAssessment, missing []string) []string {
	reasons := []string{}
	if strings.TrimSpace(assessment.Reason) != "" {
		reasons = append(reasons, assessment.Reason)
	}
	for _, reason := range missing {
		if strings.TrimSpace(reason) != "" {
			reasons = append(reasons, reason)
		}
	}
	return reasons
}
