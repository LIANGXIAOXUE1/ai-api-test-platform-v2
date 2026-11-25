package main

import (
	"encoding/json"
	"fmt"
)

// Model values retain their JSON meaning, while the request editor stores body
// and assertion expectations as text. This adapter is local to case generation.
func modelValueText(raw json.RawMessage) (json.RawMessage, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	if value == nil {
		return nil, fmt.Errorf("预期值或请求体为null，需要明确填写")
	}
	if _, ok := value.(string); ok {
		return raw, nil
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return json.Marshal(string(canonical))
}

func normalizeModelCase(raw json.RawMessage) (BusinessTestCase, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return BusinessTestCase{}, err
	}
	if requestRaw := fields["request"]; len(requestRaw) > 0 && string(requestRaw) != "null" {
		var request map[string]json.RawMessage
		if err := json.Unmarshal(requestRaw, &request); err != nil {
			return BusinessTestCase{}, fmt.Errorf("请求配置需要JSON对象")
		}
		if body := request["body"]; len(body) > 0 {
			var err error
			request["body"], err = modelValueText(body)
			if err != nil {
				return BusinessTestCase{}, err
			}
		}
		fields["request"], _ = json.Marshal(request)
	}
	if assertionsRaw := fields["assertions"]; len(assertionsRaw) > 0 && string(assertionsRaw) != "null" {
		var assertions []map[string]json.RawMessage
		if err := json.Unmarshal(assertionsRaw, &assertions); err != nil {
			return BusinessTestCase{}, fmt.Errorf("断言配置需要JSON数组")
		}
		for _, assertion := range assertions {
			if expected := assertion["expected"]; len(expected) > 0 {
				var err error
				assertion["expected"], err = modelValueText(expected)
				if err != nil {
					return BusinessTestCase{}, err
				}
			}
		}
		fields["assertions"], _ = json.Marshal(assertions)
	}
	normalized, err := json.Marshal(fields)
	if err != nil {
		return BusinessTestCase{}, err
	}
	var result BusinessTestCase
	err = json.Unmarshal(normalized, &result)
	return result, err
}

func (result *businessCasesResult) UnmarshalJSON(data []byte) error {
	var envelope struct {
		Cases              []json.RawMessage  `json:"cases"`
		MissingInformation []string           `json:"missingInformation"`
		DocumentAssessment documentAssessment `json:"documentAssessment"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return err
	}
	result.Cases = []BusinessTestCase{}
	result.InvalidCaseCount = 0
	result.MissingInformation = envelope.MissingInformation
	result.DocumentAssessment = envelope.DocumentAssessment
	for index, raw := range envelope.Cases {
		item, err := normalizeModelCase(raw)
		if err != nil {
			result.InvalidCaseCount++
			result.MissingInformation = append(result.MissingInformation, fmt.Sprintf("第%d条模型用例格式不完整，未生成可执行请求；请补充请求和预期后重试", index+1))
			continue
		}
		result.Cases = append(result.Cases, item)
	}
	return nil
}
