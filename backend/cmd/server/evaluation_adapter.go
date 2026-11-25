package main

import (
	"ai-api-test-platform-v1/backend/internal/evaluation"
	"ai-api-test-platform-v1/backend/internal/security"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type evaluationGateway struct{ s *server }

func (g evaluationGateway) Candidate(ctx context.Context, id int) (evaluation.Candidate, error) {
	p, ok := g.s.store.modelProfileByID(ctx, 0, id)
	if !ok || !p.Enabled || !p.Verified {
		return evaluation.Candidate{}, fmt.Errorf("模型配置不可用")
	}
	return evaluation.Candidate{ID: p.ID, Name: p.Name, Provider: p.Provider, Deployment: p.Model}, nil
}
func (g evaluationGateway) DefaultJudge(ctx context.Context) (int, error) {
	p, ok := g.s.activeAssistantProfile()
	if !ok {
		return 0, fmt.Errorf("没有可用助手模型")
	}
	return p.ID, nil
}
func (g evaluationGateway) Complete(ctx context.Context, id int, system, user string) (evaluation.Completion, error) {
	p, ok := g.s.store.modelProfileByID(ctx, 0, id)
	if !ok || !p.Enabled || !p.Verified {
		return evaluation.Completion{}, fmt.Errorf("模型配置不可用")
	}
	ctx, cancel := context.WithTimeout(ctx, 75*time.Second)
	defer cancel()
	payload := map[string]any{"model": p.Model, "messages": []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": user}}}
	applyReasoningOptions(payload, reasoningOptions{Mode: p.ReasoningMode, Effort: p.ReasoningEffort})
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", normalizeChatURL(p.BaseURL), bytes.NewReader(raw))
	if err != nil {
		return evaluation.Completion{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(p.AuthHeader, authValue(p.AuthHeader, p.APIKey))
	resp, err := security.HTTPClient(75 * time.Second).Do(req)
	if err != nil {
		return evaluation.Completion{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return evaluation.Completion{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := string(body)
		if p.APIKey != "" {
			message = strings.ReplaceAll(message, p.APIKey, "••••••")
		}
		if len(message) > 1200 {
			message = message[:1200]
		}
		return evaluation.Completion{}, fmt.Errorf("模型服务 HTTP %d：%s", resp.StatusCode, message)
	}
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage *struct {
			Input  int `json:"prompt_tokens"`
			Output int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err = json.Unmarshal(body, &result); err != nil {
		return evaluation.Completion{}, fmt.Errorf("模型服务返回无效 JSON")
	}
	if len(result.Choices) == 0 || strings.TrimSpace(result.Choices[0].Message.Content) == "" {
		return evaluation.Completion{}, fmt.Errorf("模型未返回文本回答")
	}
	completion := evaluation.Completion{Answer: result.Choices[0].Message.Content, UsageSource: "estimated"}
	if result.Usage != nil {
		completion.InputTokens = result.Usage.Input
		completion.OutputTokens = result.Usage.Output
		completion.UsageSource = "provider"
	} else {
		completion.InputTokens = approxTokens(system + user)
		completion.OutputTokens = approxTokens(completion.Answer)
	}
	return completion, nil
}
func (s *server) registerScenarioEvaluations(mux *http.ServeMux) error {
	store, err := evaluation.NewStore(s.store.db)
	if err != nil {
		return err
	}
	evaluation.Register(mux, evaluation.NewHandler(store, evaluationGateway{s: s}))
	return nil
}
