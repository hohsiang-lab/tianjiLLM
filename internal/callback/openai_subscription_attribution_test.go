package callback

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAISubscriptionAttributionSafeMetadataRedactsSecrets(t *testing.T) {
	attr := OpenAISubscriptionAttribution{
		CredentialID:   "cred-a",
		Provider:       "openai",
		OrganizationID: "org-a",
		Action:         "completion",
		Status:         "success",
		ReasonCode:     "",
		Metadata: map[string]any{
			"account_id":       "acct-a",
			"access_token":     "access-secret",
			"authorization":    "Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.signature",
			"credential_value": "encrypted-secret",
			"nested": map[string]any{
				"refresh_token": "refresh-secret",
			},
		},
	}

	got := attr.SafeMetadata()
	body, err := json.Marshal(got)
	require.NoError(t, err)

	assert.Equal(t, "cred-a", got["credential_id"])
	assert.Equal(t, "openai", got["provider"])
	assert.Equal(t, "org-a", got["organization_id"])
	assert.Equal(t, "completion", got["action"])
	assert.Equal(t, "success", got["status"])
	assert.Contains(t, string(body), "acct-a")
	assert.NotContains(t, string(body), "access-secret")
	assert.NotContains(t, string(body), "refresh-secret")
	assert.NotContains(t, string(body), "encrypted-secret")
	assert.NotContains(t, string(body), "eyJhbGci")
}
