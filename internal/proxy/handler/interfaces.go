package handler

import (
	"context"

	"github.com/praxisllmlab/tianjiLLM/internal/a2a"
	"github.com/praxisllmlab/tianjiLLM/internal/auth"
	"github.com/praxisllmlab/tianjiLLM/internal/policy"
)

// PolicyEvaluator is a mockable interface for policy.Engine.
type PolicyEvaluator interface {
	Evaluate(ctx context.Context, req policy.MatchRequest) (policy.EvaluateResult, error)
}

// AgentRegistryProvider is a mockable interface for a2a.AgentRegistry.
type AgentRegistryProvider interface {
	GetAgentByID(id string) (*a2a.AgentConfig, bool)
}

// MessageSender is a mockable interface for a2a.CompletionBridge.
type MessageSender interface {
	SendMessage(ctx context.Context, agent *a2a.AgentConfig, userMessage string) (*a2a.SendMessageResult, error)
}

// SSOProvider is a mockable interface for auth.SSOHandler.
type SSOProvider interface {
	LoginURL(state string) string
	ExchangeCode(ctx context.Context, code string) (*auth.TokenResponse, error)
	GetUserInfo(ctx context.Context, accessToken string) (*auth.UserInfo, error)
	MapRole(groups []string) auth.Role
}
