package callback

import "github.com/praxisllmlab/tianjiLLM/internal/security/redact"

// OpenAISubscriptionAttribution carries only stable, non-secret subscription facts.
type OpenAISubscriptionAttribution struct {
	CredentialID   string
	Provider       string
	OrganizationID string
	Action         string
	Status         string
	ReasonCode     string
	RequestID      string
	Metadata       map[string]any
}

func (a OpenAISubscriptionAttribution) SafeMetadata() map[string]any {
	out := make(map[string]any)
	if a.CredentialID != "" {
		out["credential_id"] = a.CredentialID
	}
	if a.Provider != "" {
		out["provider"] = a.Provider
	}
	if a.OrganizationID != "" {
		out["organization_id"] = a.OrganizationID
	}
	if a.Action != "" {
		out["action"] = a.Action
	}
	if a.Status != "" {
		out["status"] = a.Status
	}
	if a.ReasonCode != "" {
		out["reason_code"] = a.ReasonCode
	}
	if a.RequestID != "" {
		out["request_id"] = a.RequestID
	}
	if safeMetadata, ok := redact.JSONValue(a.Metadata).(map[string]any); ok {
		for key, value := range safeMetadata {
			if _, exists := out[key]; !exists {
				out[key] = value
			}
		}
	}
	return out
}
