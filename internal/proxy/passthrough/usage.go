package passthrough

import (
	"encoding/json"
	"io"
)

// AnthropicUsage extracts usage from an Anthropic response.
type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ParseAnthropicUsage extracts token usage from a response body.
func ParseAnthropicUsage(body io.Reader) (*AnthropicUsage, error) {
	var resp struct {
		Usage AnthropicUsage `json:"usage"`
	}

	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		return nil, err
	}

	return &resp.Usage, nil
}
