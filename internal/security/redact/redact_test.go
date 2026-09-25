package redact

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testJWT = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJhY2N0XzEyMyJ9.KYrx8y6xdp00qL1SzDMTqSz1KxB1GpQcqj61iP7rU3I"

func TestRedactString_RemovesBearerTokenAndJWT(t *testing.T) {
	input := "upstream failed: Authorization: Bearer " + testJWT + " api_key=sk-test-secret"

	got := String(input)

	assert.NotContains(t, got, testJWT)
	assert.NotContains(t, got, "sk-test-secret")
	assert.NotContains(t, got, "Bearer")
	assert.Contains(t, got, Marker)
}

func TestRedactJSONValue_RemovesOAuthOneTimeFields(t *testing.T) {
	input := map[string]any{
		"authorization_code": "redact-this-auth-value",
		"code_verifier":      "redact-this-verifier-value",
		"code_challenge":     "redact-this-challenge-value",
		"device_auth_id":     "redact-this-device-value",
		"user_code":          "redact-this-user-code",
	}

	body, err := json.Marshal(JSONValue(input))
	require.NoError(t, err)

	assert.NotContains(t, string(body), "redact-this-auth-value")
	assert.NotContains(t, string(body), "redact-this-verifier-value")
	assert.NotContains(t, string(body), "redact-this-challenge-value")
	assert.NotContains(t, string(body), "redact-this-device-value")
	assert.NotContains(t, string(body), "redact-this-user-code")
	assert.Contains(t, string(body), Marker)
}

func TestRedactJSONValue_RemovesNestedCredentialFields(t *testing.T) {
	input := map[string]any{
		"email":  "owner@example.com",
		"status": "refresh_failed",
		"oauth": map[string]any{
			"Access_Token":  "access-secret",
			"refresh_token": "refresh-secret",
			"account": []any{
				map[string]any{"id_token": testJWT, "label": "kept"},
			},
		},
		"disabled_reason": "token expired: " + testJWT,
	}

	got := JSONValue(input)
	body, err := json.Marshal(got)
	require.NoError(t, err)

	assert.NotContains(t, string(body), "access-secret")
	assert.NotContains(t, string(body), "refresh-secret")
	assert.NotContains(t, string(body), testJWT)
	assert.Contains(t, string(body), Marker)
	assert.Contains(t, string(body), "owner@example.com")
	assert.Contains(t, string(body), "refresh_failed")
	assert.Contains(t, string(body), "kept")
}

func TestRawJSON_RedactsOAuthFieldsInMalformedBody(t *testing.T) {
	raw := []byte("authorization_code=auth-value code_verifier=verifier-value code_challenge=challenge-value device_auth_id=device-value user_code=user-value")

	got := RawJSON(raw)

	for _, value := range []string{"auth-value", "verifier-value", "challenge-value", "device-value", "user-value"} {
		assert.NotContains(t, string(got), value)
	}
	assert.Contains(t, string(got), Marker)
}

func TestJSONValue_RedactsOAuthFieldsInsideNestedStrings(t *testing.T) {
	input := map[string]any{
		"details": map[string]any{
			"message": "authorization_code=auth-value user_code=user-value",
		},
	}

	body, err := json.Marshal(JSONValue(input))
	require.NoError(t, err)

	assert.NotContains(t, string(body), "auth-value")
	assert.NotContains(t, string(body), "user-value")
	assert.Contains(t, string(body), Marker)
}
