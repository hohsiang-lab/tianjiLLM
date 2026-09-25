package spend

import (
	"encoding/json"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/stretchr/testify/assert"
)

func TestMergeOpenAISubscriptionAttributionPreservesCallerMetadata(t *testing.T) {
	rec := SpendRecord{
		Metadata: map[string]any{
			"caller":        "kept",
			"credential_id": "caller-value",
		},
		OpenAISubscription: &callback.OpenAISubscriptionAttribution{
			CredentialID:   "cred-b",
			Provider:       "openai",
			OrganizationID: "org-a",
			Action:         "completion",
			Status:         "success",
		},
	}

	got := rec.metadataWithOpenAISubscriptionAttribution()

	assert.Equal(t, "kept", got["caller"])
	assert.Equal(t, "caller-value", got["credential_id"])
	subscription, ok := got["openai_subscription"].(map[string]any)
	if assert.True(t, ok) {
		assert.Equal(t, "cred-b", subscription["credential_id"])
		assert.Equal(t, "openai", subscription["provider"])
		assert.Equal(t, "org-a", subscription["organization_id"])
		assert.Equal(t, "completion", subscription["action"])
		assert.Equal(t, "success", subscription["status"])
	}
}

func TestMetadataWithoutOpenAISubscriptionPreservesNilShape(t *testing.T) {
	rec := SpendRecord{}

	got := rec.metadataWithOpenAISubscriptionAttribution()
	body, err := json.Marshal(got)

	assert.NoError(t, err)
	assert.Equal(t, "null", string(body))
}

func TestLogSuccessCopiesOpenAISubscriptionAttributionToSpendRecord(t *testing.T) {
	data := callback.LogData{
		Model:    "gpt-4o",
		Provider: "openai",
		OpenAISubscription: &callback.OpenAISubscriptionAttribution{
			CredentialID:   "cred-a",
			Provider:       "openai",
			OrganizationID: "org-a",
			Status:         "success",
		},
	}

	rec := spendRecordFromLogData(data)

	if assert.NotNil(t, rec.OpenAISubscription) {
		assert.Equal(t, "cred-a", rec.OpenAISubscription.CredentialID)
		assert.Equal(t, "openai", rec.OpenAISubscription.Provider)
		assert.Equal(t, "org-a", rec.OpenAISubscription.OrganizationID)
	}
}
