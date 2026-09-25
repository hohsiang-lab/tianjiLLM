package router

import (
	"context"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
	"github.com/stretchr/testify/assert"
)

func TestGeneralFallback_DelayBetweenAttempts(t *testing.T) {
	// T055: Verify 500ms delay between fallback attempts.
	// Create 3 fallback models that all fail (unknown provider).
	apiKey := "sk-test"
	models := []config.ModelConfig{
		{
			ModelName: "fb-1",
			TianjiParams: config.TianjiParams{
				Model:  "nonexistent_provider/fb-1",
				APIKey: &apiKey,
			},
		},
		{
			ModelName: "fb-2",
			TianjiParams: config.TianjiParams{
				Model:  "nonexistent_provider/fb-2",
				APIKey: &apiKey,
			},
		},
		{
			ModelName: "fb-3",
			TianjiParams: config.TianjiParams{
				Model:  "nonexistent_provider/fb-3",
				APIKey: &apiKey,
			},
		},
	}

	r := New(models, &roundRobinStrategy{}, RouterSettings{
		NumRetries: 0, // no retries within each Route() call — fast
		Fallbacks: map[string][]string{
			"primary": {"fb-1", "fb-2", "fb-3"},
		},
	})

	start := time.Now()
	_, _, err := r.GeneralFallback(context.Background(), "primary")
	elapsed := time.Since(start)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "all fallbacks exhausted")
	// 3 fallbacks, 2 gaps of 500ms each = ~1s minimum
	assert.GreaterOrEqual(t, elapsed, 900*time.Millisecond,
		"should have 500ms delay between fallback attempts")
}

func TestContentPolicyFallback_DelayBetweenAttempts(t *testing.T) {
	apiKey := "sk-test"
	models := []config.ModelConfig{
		{
			ModelName: "cp-1",
			TianjiParams: config.TianjiParams{
				Model:  "nonexistent_provider/cp-1",
				APIKey: &apiKey,
			},
		},
		{
			ModelName: "cp-2",
			TianjiParams: config.TianjiParams{
				Model:  "nonexistent_provider/cp-2",
				APIKey: &apiKey,
			},
		},
	}

	r := New(models, &roundRobinStrategy{}, RouterSettings{
		NumRetries: 0,
		ContentPolicyFallbacks: map[string][]string{
			"primary": {"cp-1", "cp-2"},
		},
	})

	start := time.Now()
	_, _, err := r.ContentPolicyFallback(context.Background(), "primary")
	elapsed := time.Since(start)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "all content policy fallbacks exhausted")
	// 2 fallbacks, 1 gap of 500ms = ~500ms minimum
	assert.GreaterOrEqual(t, elapsed, 400*time.Millisecond,
		"should have 500ms delay between content policy fallback attempts")
}
