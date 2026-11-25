package main

import (
	"context"
	"fmt"
)

type assistantProfileContextKey struct{}

// Selection is request scoped: testers can use shared models without changing
// the administrator's default assistant or another user's in-flight request.
func (s *server) withAssistantProfile(ctx context.Context, id int) (context.Context, error) {
	if id == 0 {
		return ctx, nil
	}
	profile, ok := s.store.modelProfileByID(ctx, 0, id)
	if !ok || !profile.Enabled || !profile.Verified {
		return ctx, fmt.Errorf("所选助手模型未启用或未通过验证，请在模型池测试连接")
	}
	return context.WithValue(ctx, assistantProfileContextKey{}, profile), nil
}
