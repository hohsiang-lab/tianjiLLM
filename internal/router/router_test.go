package router

import (
	"context"
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
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no deployments")
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
