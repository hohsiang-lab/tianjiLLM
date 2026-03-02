package model

import (
	"encoding/json"
	"testing"
)

// TestUsageUnmarshalCacheTokens verifies that model.Usage can unmarshal
// cache_read_input_tokens and cache_creation_input_tokens from JSON.
// Verifies cache fields exist on Usage struct.
func TestUsageUnmarshalCacheTokens(t *testing.T) {
	raw := `{
		"prompt_tokens": 1000,
		"completion_tokens": 200,
		"total_tokens": 1200,
		"cache_read_input_tokens": 800,
		"cache_creation_input_tokens": 150
	}`

	var u Usage
	if err := json.Unmarshal([]byte(raw), &u); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if u.CacheReadInputTokens != 800 {
		t.Errorf("CacheReadInputTokens = %d, want 800", u.CacheReadInputTokens)
	}
	if u.CacheCreationInputTokens != 150 {
		t.Errorf("CacheCreationInputTokens = %d, want 150", u.CacheCreationInputTokens)
	}
}
