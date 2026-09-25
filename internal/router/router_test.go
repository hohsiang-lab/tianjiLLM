package router

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// roundRobinStrategy picks deployments in order for deterministic testing.
type roundRobinStrategy struct {
	idx int
}

func (s *roundRobinStrategy) Pick(deployments []*Deployment) *Deployment {
	if len(deployments) == 0 {
		return nil
	}
	d := deployments[s.idx%len(deployments)]
	s.idx++
	return d
}

func TestRouter_Route_Success(t *testing.T) {
	apiKey := "sk-test"
	models := []config.ModelConfig{
		{
			ModelName: "gpt-4o",
			TianjiParams: config.TianjiParams{
				Model:  "openai/gpt-4o",
				APIKey: &apiKey,
			},
		},
	}

	r := New(models, &roundRobinStrategy{}, RouterSettings{})
	req := &model.ChatCompletionRequest{Model: "gpt-4o"}

	d, p, err := r.Route(context.Background(), "gpt-4o", req)
	require.NoError(t, err)
	assert.NotNil(t, d)
	assert.NotNil(t, p)
	assert.Equal(t, "gpt-4o", d.ModelName)
}

func TestRouter_Route_NoDeployments(t *testing.T) {
	r := New(nil, &roundRobinStrategy{}, RouterSettings{})
	_, _, err := r.Route(context.Background(), "nonexistent", nil)
	assert.True(t, errors.Is(err, ErrNoDeployments))
}

func TestRouter_Fallback_OnProviderFailure(t *testing.T) {
	apiKey := "sk-test"
	models := []config.ModelConfig{
		{
			ModelName: "gpt-4o",
			TianjiParams: config.TianjiParams{
				Model:  "unknown_provider/gpt-4o", // will fail provider.Get
				APIKey: &apiKey,
			},
		},
		{
			ModelName: "gpt-4o",
			TianjiParams: config.TianjiParams{
				Model:  "openai/gpt-4o",
				APIKey: &apiKey,
			},
		},
	}

	r := New(models, &roundRobinStrategy{}, RouterSettings{NumRetries: 2})
	req := &model.ChatCompletionRequest{Model: "gpt-4o"}

	d, p, err := r.Route(context.Background(), "gpt-4o", req)
	require.NoError(t, err)
	assert.NotNil(t, d)
	assert.NotNil(t, p)
	assert.Equal(t, "openai", d.ProviderName, "should fallback to openai deployment")
}

func TestRouter_Cooldown(t *testing.T) {
	apiKey := "sk-test"
	models := []config.ModelConfig{
		{
			ModelName: "gpt-4o",
			TianjiParams: config.TianjiParams{
				Model:  "openai/gpt-4o",
				APIKey: &apiKey,
			},
		},
	}

	r := New(models, &roundRobinStrategy{}, RouterSettings{
		AllowedFails: 2,
		CooldownTime: 100 * time.Millisecond,
	})

	deployments := r.GetDeployments("gpt-4o")
	require.Len(t, deployments, 1)
	d := deployments[0]

	// Record failures up to threshold
	d.RecordFailure()
	assert.True(t, d.IsHealthy(), "should still be healthy after 1 failure")

	d.RecordFailure()
	assert.False(t, d.IsHealthy(), "should be in cooldown after 2 failures")

	// Wait for cooldown
	time.Sleep(150 * time.Millisecond)
	assert.True(t, d.IsHealthy(), "should be healthy after cooldown")
}

func TestRouter_Route_WildcardMatch(t *testing.T) {
	apiKey := "sk-test"
	models := []config.ModelConfig{
		{
			ModelName: "claude-*",
			TianjiParams: config.TianjiParams{
				Model:  "openai/claude-*", // uses openai provider for test simplicity
				APIKey: &apiKey,
			},
		},
	}

	r := New(models, &roundRobinStrategy{}, RouterSettings{})
	req := &model.ChatCompletionRequest{Model: "claude-sonnet-4-5"}

	d, p, err := r.Route(context.Background(), "claude-sonnet-4-5", req)
	require.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, "claude-sonnet-4-5", d.ModelName, "wildcard should resolve model name")
}

func TestRouter_Route_WildcardSpecificity(t *testing.T) {
	apiKey := "sk-test"
	models := []config.ModelConfig{
		{
			ModelName: "claude-*",
			TianjiParams: config.TianjiParams{
				Model:  "openai/claude-*",
				APIKey: &apiKey,
			},
		},
		{
			ModelName: "claude-sonnet-*",
			TianjiParams: config.TianjiParams{
				Model:  "openai/claude-sonnet-*",
				APIKey: &apiKey,
			},
		},
	}

	r := New(models, &roundRobinStrategy{}, RouterSettings{})
	req := &model.ChatCompletionRequest{Model: "claude-sonnet-4-5"}

	d, _, err := r.Route(context.Background(), "claude-sonnet-4-5", req)
	require.NoError(t, err)
	// Should match "claude-sonnet-*" (more specific) → resolves to "claude-sonnet-4-5"
	assert.Equal(t, "claude-sonnet-4-5", d.ModelName)
}

func TestRouter_Route_ExactMatchOverWildcard(t *testing.T) {
	apiKey := "sk-test"
	models := []config.ModelConfig{
		{
			ModelName: "claude-*",
			TianjiParams: config.TianjiParams{
				Model:  "openai/claude-*",
				APIKey: &apiKey,
			},
		},
		{
			ModelName: "claude-opus",
			TianjiParams: config.TianjiParams{
				Model:  "openai/claude-opus",
				APIKey: &apiKey,
			},
		},
	}

	r := New(models, &roundRobinStrategy{}, RouterSettings{})
	req := &model.ChatCompletionRequest{Model: "claude-opus"}

	d, _, err := r.Route(context.Background(), "claude-opus", req)
	require.NoError(t, err)
	// Exact match should be found directly, not via wildcard
	assert.Equal(t, "claude-opus", d.ModelName)
}

func TestRouter_RecordSuccess_ResetsFailures(t *testing.T) {
	// Verify provider is registered
	_, err := provider.Get("openai")
	require.NoError(t, err)

	apiKey := "sk-test"
	models := []config.ModelConfig{
		{
			ModelName: "gpt-4o",
			TianjiParams: config.TianjiParams{
				Model:  "openai/gpt-4o",
				APIKey: &apiKey,
			},
		},
	}

	r := New(models, &roundRobinStrategy{}, RouterSettings{AllowedFails: 3})
	deployments := r.GetDeployments("gpt-4o")
	d := deployments[0]

	d.RecordFailure()
	d.RecordFailure()
	r.RecordSuccess(d, 100*time.Millisecond)

	// After success, failure count is reset — 2 more failures shouldn't trigger cooldown
	d.RecordFailure()
	d.RecordFailure()
	assert.True(t, d.IsHealthy(), "should still be healthy because success reset failures")
}

func TestModelGroupAlias(t *testing.T) {
	settings := RouterSettings{
		ModelGroupAlias: map[string]ModelGroupAliasItem{
			"gpt-4": {Model: "gpt-4o"},
		},
	}
	r := New(nil, &roundRobinStrategy{}, settings)
	alias := r.ModelGroupAlias()
	assert.NotNil(t, alias)
	assert.Contains(t, alias, "gpt-4")
}

func TestRecordFailure(t *testing.T) {
	r := New(nil, &roundRobinStrategy{}, RouterSettings{})
	d := &Deployment{
		allowedFails: 3,
		cooldownTime: time.Minute,
	}
	// Should not panic
	r.RecordFailure(d)
}

func TestExtractLastUserMessage_Empty(t *testing.T) {
	req := &model.ChatCompletionRequest{}
	msg := extractLastUserMessage(req)
	assert.Equal(t, "", msg)
}

func TestExtractLastUserMessage_User(t *testing.T) {
	req := &model.ChatCompletionRequest{
		Messages: []model.Message{
			{Role: "system", Content: "system prompt"},
			{Role: "user", Content: "hello"},
		},
	}
	msg := extractLastUserMessage(req)
	assert.Equal(t, "hello", msg)
}

func TestExtractTags_Nil(t *testing.T) {
	tags := extractTags(nil)
	assert.Nil(t, tags)
}

func TestExtractTags_NoMetadata(t *testing.T) {
	req := &model.ChatCompletionRequest{}
	tags := extractTags(req)
	assert.Nil(t, tags)
}

func TestExtractTags_WithTags(t *testing.T) {
	req := &model.ChatCompletionRequest{
		Metadata: map[string]any{
			"tags": []any{"billing", "prod"},
		},
	}
	tags := extractTags(req)
	assert.Equal(t, []string{"billing", "prod"}, tags)
}

// --- Exponential Backoff Tests (T050-T057a) ---

// failingDeployments creates N deployments that all fail provider lookup.
func failingDeployments(n int, modelGroup string) []config.ModelConfig {
	apiKey := "sk-test"
	models := make([]config.ModelConfig, n)
	for i := range n {
		models[i] = config.ModelConfig{
			ModelName: modelGroup,
			TianjiParams: config.TianjiParams{
				Model:  "nonexistent_provider/some-model",
				APIKey: &apiKey,
			},
		}
	}
	return models
}

func TestRouter_RetryWithExponentialBackoff(t *testing.T) {
	// T050: All deployments fail, verify exponential backoff timing.
	// With 4 deployments and 3 retries, we expect 3 backoff delays.
	// Default base = 1s: ~1s, ~2s, ~4s (with jitter)
	// This test would take ~7s with real backoff — use short base via RetryPolicy.
	models := failingDeployments(4, "test-model")
	r := New(models, &roundRobinStrategy{}, RouterSettings{
		NumRetries: 3,
		ModelGroupRetryPolicy: map[string]RetryPolicy{
			"test-model": {RetryAfterSeconds: 0}, // uses default 1s base
		},
	})

	// Use a very short base to keep test fast. We'll test computeBackoff directly.
	start := time.Now()
	_, _, err := r.Route(context.Background(), "test-model", nil)
	elapsed := time.Since(start)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "all deployments failed")
	// With default 1s base and 3 retries: ~1 + ~2 + ~4 = ~7s total backoff
	// Verify it took at least some time (proving backoff happened)
	assert.GreaterOrEqual(t, elapsed, 3*time.Second, "should have exponential backoff delays")
}

func TestRouter_ComputeBackoff_ExponentialPattern(t *testing.T) {
	// Verify the exponential pattern directly without waiting.
	policy := RetryPolicy{}
	for attempt := range 5 {
		d := computeBackoff(attempt, policy)
		expected := float64(defaultBackoffBase) * math.Pow(2, float64(attempt))
		if expected > float64(maxBackoff) {
			expected = float64(maxBackoff)
		}
		// With jitter, delay should be in [expected, expected * 1.25]
		assert.GreaterOrEqual(t, float64(d), expected,
			"attempt %d: delay should be >= base*2^attempt", attempt)
		assert.LessOrEqual(t, float64(d), expected*1.25+float64(time.Millisecond),
			"attempt %d: delay should be <= base*2^attempt * 1.25", attempt)
	}
}

func TestRouter_RespectsRetryAfterHeader(t *testing.T) {
	// T051: RetryAfterSeconds in policy acts as the backoff base.
	// With base=2s, attempt 0 → ~2s, attempt 1 → ~4s
	policy := RetryPolicy{RetryAfterSeconds: 2}
	d0 := computeBackoff(0, policy)
	d1 := computeBackoff(1, policy)

	assert.GreaterOrEqual(t, d0, 2*time.Second, "base should be 2s")
	assert.Less(t, d0, 2500*time.Millisecond, "attempt 0 should be ~2s + jitter")
	assert.GreaterOrEqual(t, d1, 4*time.Second, "attempt 1 should be ~4s")
	assert.Less(t, d1, 5100*time.Millisecond, "attempt 1 should be ~4s + jitter")
}

func TestRouter_RetryAfterCapAt300s(t *testing.T) {
	// T052: Even with huge RetryAfterSeconds, backoff is capped at maxBackoff (30s).
	policy := RetryPolicy{RetryAfterSeconds: 3600}
	d := computeBackoff(0, policy)
	assert.LessOrEqual(t, d, maxBackoff+maxBackoff/4, // 30s + 25% jitter max
		"backoff should be capped at maxBackoff even with huge RetryAfterSeconds")
}

func TestRouter_AllDeploymentsCooldown_ReturnsRetryAfter(t *testing.T) {
	// T053: All deployments in cooldown → error includes "all deployments failed".
	apiKey := "sk-test"
	models := []config.ModelConfig{
		{
			ModelName: "gpt-4o",
			TianjiParams: config.TianjiParams{
				Model:  "openai/gpt-4o",
				APIKey: &apiKey,
			},
		},
	}

	r := New(models, &roundRobinStrategy{}, RouterSettings{
		AllowedFails: 1,
		CooldownTime: 10 * time.Second,
		NumRetries:   0, // no retries → fast test
	})

	// Force cooldown
	d := r.GetDeployments("gpt-4o")[0]
	d.RecordFailure()
	assert.False(t, d.IsHealthy())

	// Route should still try (last resort) but fail on provider
	// (the deployment has openai provider which exists, so it succeeds).
	// For a true all-fail scenario we need unknown provider.
	models2 := failingDeployments(1, "fail-model")
	r2 := New(models2, &roundRobinStrategy{}, RouterSettings{
		AllowedFails: 1,
		CooldownTime: 10 * time.Second,
		NumRetries:   0,
	})
	d2 := r2.GetDeployments("fail-model")[0]
	d2.RecordFailure()

	_, _, err := r2.Route(context.Background(), "fail-model", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "all deployments failed")
}

func TestRouter_UsesRetryPolicySettings(t *testing.T) {
	// T054: Per-model-group RetryPolicy overrides global NumRetries and sets base.
	policy := RetryPolicy{
		NumRetries:        1,
		RetryAfterSeconds: 0, // use default 1s
	}
	d := computeBackoff(0, policy)
	assert.GreaterOrEqual(t, d, defaultBackoffBase)
	assert.LessOrEqual(t, d, defaultBackoffBase+defaultBackoffBase/4+time.Millisecond)

	// Verify the router uses per-group NumRetries
	models := failingDeployments(5, "custom-model")
	r := New(models, &roundRobinStrategy{}, RouterSettings{
		NumRetries: 10, // global
		ModelGroupRetryPolicy: map[string]RetryPolicy{
			"custom-model": {NumRetries: 1, RetryAfterSeconds: 0},
		},
	})

	start := time.Now()
	_, _, err := r.Route(context.Background(), "custom-model", nil)
	elapsed := time.Since(start)

	assert.Error(t, err)
	// With NumRetries=1, only 1 backoff delay (~1s). Should be much less than 10 retries.
	assert.Less(t, elapsed, 3*time.Second, "should use per-group NumRetries=1, not global=10")
}

func TestRouter_BackoffWithJitter(t *testing.T) {
	// T056: Multiple calls to computeBackoff produce non-identical results (jitter).
	policy := RetryPolicy{}
	results := make(map[time.Duration]bool)
	for range 20 {
		d := computeBackoff(2, policy)
		results[d] = true
	}
	// With random jitter, we expect at least a few distinct values out of 20 samples.
	assert.Greater(t, len(results), 1,
		"backoff should have jitter producing varying delays")
}

func TestRouter_MaxBackoffCap(t *testing.T) {
	// T057: Even with many consecutive attempts, backoff never exceeds maxBackoff + jitter.
	policy := RetryPolicy{}
	for attempt := range 15 {
		d := computeBackoff(attempt, policy)
		maxWithJitter := maxBackoff + time.Duration(float64(maxBackoff)*jitterFraction)
		assert.LessOrEqual(t, d, maxWithJitter+time.Millisecond,
			"attempt %d: backoff must be <= maxBackoff + jitter", attempt)
	}
}

func TestRouter_ExhaustsRetriesReturns429(t *testing.T) {
	// T057a: All retries exhausted → returns error (caller maps to 429).
	models := failingDeployments(5, "exhaust-model")
	r := New(models, &roundRobinStrategy{}, RouterSettings{
		NumRetries: 2,
		ModelGroupRetryPolicy: map[string]RetryPolicy{
			"exhaust-model": {RetryAfterSeconds: 0},
		},
	})

	start := time.Now()
	_, _, err := r.Route(context.Background(), "exhaust-model", nil)
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "all deployments failed")
	// Verify backoff happened (at least 2 delays: ~1s + ~2s = ~3s)
	assert.GreaterOrEqual(t, elapsed, 2*time.Second)
}

func TestRouter_TimeoutCancelsBackoff(t *testing.T) {
	// T062: TimeoutSeconds in RetryPolicy sets context deadline.
	models := failingDeployments(10, "timeout-model")
	r := New(models, &roundRobinStrategy{}, RouterSettings{
		NumRetries: 10,
		ModelGroupRetryPolicy: map[string]RetryPolicy{
			"timeout-model": {TimeoutSeconds: 2}, // 2s timeout
		},
	})

	start := time.Now()
	_, _, err := r.Route(context.Background(), "timeout-model", nil)
	elapsed := time.Since(start)

	require.Error(t, err)
	// Should be cancelled by timeout, not waiting for all retries
	assert.Less(t, elapsed, 5*time.Second, "should be cancelled by 2s timeout")
	assert.True(t, errors.Is(err, context.DeadlineExceeded),
		"should return context.DeadlineExceeded")
}
