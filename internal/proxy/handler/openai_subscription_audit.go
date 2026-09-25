package handler

import (
	"context"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
)

func (h *Handlers) auditOpenAISubscriptionLifecycle(ctx context.Context, attr callback.OpenAISubscriptionAttribution) {
	if attr.Provider == "" {
		attr.Provider = "openai"
	}
	payload := attr.SafeMetadata()
	payload["timestamp"] = time.Now().UTC().Format(time.RFC3339)
	h.createAuditLog(ctx, attr.Action, "CredentialTable", attr.CredentialID, "", "", nil, payload)
}

func openAISubscriptionCredentialOrgID(organizationID *string) string {
	if organizationID == nil {
		return ""
	}
	return *organizationID
}

func openAISubscriptionAttributionForAction(attr *callback.OpenAISubscriptionAttribution, action string) *callback.OpenAISubscriptionAttribution {
	if attr == nil {
		return nil
	}
	copied := *attr
	copied.Action = action
	return &copied
}

func applyOpenAISubscriptionLogAttribution(data *callback.LogData, attr *callback.OpenAISubscriptionAttribution) {
	if data == nil || attr == nil {
		return
	}
	data.OpenAISubscription = attr
	if data.UpstreamTokenKey == "" {
		data.UpstreamTokenKey = attr.CredentialID
	}
}
