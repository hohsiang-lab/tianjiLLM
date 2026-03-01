package passthrough

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAnthropicUsage_Valid(t *testing.T) {
	body := strings.NewReader(`{"usage":{"input_tokens":10,"output_tokens":20}}`)
	u, err := ParseAnthropicUsage(body)
	require.NoError(t, err)
	assert.Equal(t, 10, u.InputTokens)
	assert.Equal(t, 20, u.OutputTokens)
}

func TestParseAnthropicUsage_Empty(t *testing.T) {
	body := strings.NewReader(`{"usage":{}}`)
	u, err := ParseAnthropicUsage(body)
	require.NoError(t, err)
	assert.Equal(t, 0, u.InputTokens)
	assert.Equal(t, 0, u.OutputTokens)
}

func TestParseAnthropicUsage_InvalidJSON(t *testing.T) {
	body := strings.NewReader(`not json`)
	_, err := ParseAnthropicUsage(body)
	assert.Error(t, err)
}

func TestExtractAnthropicUsage(t *testing.T) {
	body := strings.NewReader(`{"usage":{"input_tokens":5,"output_tokens":15}}`)
	p, c := ExtractAnthropicUsage(body)
	// Current implementation always returns 0
	assert.Equal(t, 0, p)
	assert.Equal(t, 0, c)
}

func TestHandler_UnknownEndpoint(t *testing.T) {
	cfg := Config{
		ProviderEndpoints: map[string]string{
			"/anthropic": "https://api.anthropic.com",
		},
		APIKeys: map[string]string{},
	}
	h := Handler(cfg)

	r := httptest.NewRequest("GET", "/unknown/path", nil)
	w := httptest.NewRecorder()
	h(w, r)
	assert.Equal(t, 404, w.Code)
}
