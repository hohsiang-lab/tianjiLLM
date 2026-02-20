package integration

import (
	"context"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/cache"
	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/guardrail"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/router/strategy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEdge_GuardrailUnexpectedFormat_FailOpen(t *testing.T) {
	reg := guardrail.NewRegistry()
	reg.RegisterWithPolicy(guardrail.NewContentFilter(2), true) // fail-open

	req := &model.ChatCompletionRequest{
		Messages: []model.Message{
			{Role: "user", Content: "Normal question about programming"},
		},
	}

	result, err := reg.RunPreCall(context.Background(), []string{"content_filter"}, req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestEdge_GuardrailBlocked_FailClosed(t *testing.T) {
	reg := guardrail.NewRegistry()
	reg.RegisterWithPolicy(guardrail.NewContentFilter(1), false) // fail-closed, low threshold

	req := &model.ChatCompletionRequest{
		Messages: []model.Message{
			{Role: "user", Content: "This contains violence and kill and murder words"},
		},
	}

	_, err := reg.RunPreCall(context.Background(), []string{"content_filter"}, req)
	assert.Error(t, err)
}

func TestEdge_DiskCacheTTLExpiry(t *testing.T) {
	dir := t.TempDir()
	dc, err := cache.NewDiskCache(dir)
	require.NoError(t, err)

	ctx := context.Background()
	_ = dc.Set(ctx, "expire-me", []byte("value"), 50*time.Millisecond)

	time.Sleep(100 * time.Millisecond)

	val, err := dc.Get(ctx, "expire-me")
	assert.NoError(t, err)
	assert.Nil(t, val) // expired
}

func TestEdge_CallbackFactoryUnknownType(t *testing.T) {
	_, err := callback.NewFromConfig("nonexistent", "", "", "", "", "", "", "", "", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown callback type")
}

func TestEdge_StrategyFactoryUnknownStrategy(t *testing.T) {
	_, err := strategy.NewFromConfig("nonexistent-strategy")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown routing strategy")
}

func TestEdge_StrategyFactoryEmptyDefault(t *testing.T) {
	s, err := strategy.NewFromConfig("")
	assert.NoError(t, err)
	assert.NotNil(t, s) // defaults to shuffle
}
