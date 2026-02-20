package integration

import (
	"context"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
	"github.com/praxisllmlab/tianjiLLM/internal/router"
	"github.com/praxisllmlab/tianjiLLM/internal/router/strategy"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func int64Ptr(n int64) *int64       { return &n }
func float64Ptr(f float64) *float64 { return &f }

func TestRouter_MultiDeployment_Fallback(t *testing.T) {
	apiKey := "sk-test"
	models := []config.ModelConfig{
		{
			ModelName: "gpt-4o",
			TianjiParams: config.TianjiParams{
				Model:  "unknown_provider/gpt-4o",
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
		{
			ModelName: "gpt-4o",
			TianjiParams: config.TianjiParams{
				Model:  "openai/gpt-4o-mini",
				APIKey: &apiKey,
			},
		},
	}

	r := router.New(models, strategy.NewShuffle(), router.RouterSettings{
		AllowedFails: 3,
		CooldownTime: time.Second,
		NumRetries:   2,
	})

	req := &model.ChatCompletionRequest{Model: "gpt-4o"}

	// Should succeed by falling back to openai deployments
	d, p, err := r.Route(context.Background(), "gpt-4o", req)
	require.NoError(t, err)
	assert.NotNil(t, d)
	assert.NotNil(t, p)
	assert.Equal(t, "openai", d.ProviderName)
}

func TestRouter_LowestLatency_Selection(t *testing.T) {
	apiKey := "sk-test"
	models := []config.ModelConfig{
		{
			ModelName: "gpt-4o",
			TianjiParams: config.TianjiParams{
				Model:  "openai/gpt-4o",
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

	r := router.New(models, strategy.NewLowestLatency(), router.RouterSettings{})

	// Record different latencies
	deployments := r.GetDeployments("gpt-4o")
	require.Len(t, deployments, 2)

	deployments[0].RecordSuccess(500 * time.Millisecond)
	deployments[1].RecordSuccess(100 * time.Millisecond)

	// Lowest latency should pick the faster one
	d, _, err := r.Route(context.Background(), "gpt-4o", nil)
	require.NoError(t, err)
	assert.Equal(t, deployments[1].ID, d.ID)
}

func TestStrategy_LowestCost_PicksCheapest(t *testing.T) {
	cheapCost := float64Ptr(0.001)
	expensiveCost := float64Ptr(0.01)

	deployments := []*router.Deployment{
		{
			ID:        "expensive",
			ModelName: "gpt-4o",
			Config: &config.ModelConfig{
				ModelInfo: &config.ModelInfo{InputCost: expensiveCost},
			},
		},
		{
			ID:        "cheap",
			ModelName: "gpt-4o-mini",
			Config: &config.ModelConfig{
				ModelInfo: &config.ModelInfo{InputCost: cheapCost},
			},
		},
	}

	s := strategy.NewLowestCost()
	picked := s.Pick(deployments)
	require.NotNil(t, picked)
	assert.Equal(t, "cheap", picked.ID)
}

func TestStrategy_UsageBased_PicksLeastUtilized(t *testing.T) {
	rpm := int64Ptr(100)
	tpm := int64Ptr(100000)

	deployments := []*router.Deployment{
		{
			ID:        "busy",
			ModelName: "gpt-4o",
			Config: &config.ModelConfig{
				TianjiParams: config.TianjiParams{RPM: rpm, TPM: tpm},
			},
		},
		{
			ID:        "idle",
			ModelName: "gpt-4o",
			Config: &config.ModelConfig{
				TianjiParams: config.TianjiParams{RPM: rpm, TPM: tpm},
			},
		},
	}

	s := strategy.NewUsageBased(time.Minute)

	// Record heavy usage on "busy"
	s.RecordUsage("busy", 5000)
	s.RecordUsage("busy", 5000)
	s.RecordUsage("busy", 5000)

	// Record light usage on "idle"
	s.RecordUsage("idle", 100)

	picked := s.Pick(deployments)
	require.NotNil(t, picked)
	assert.Equal(t, "idle", picked.ID)
}

func TestStrategy_TagBased_FiltersCorrectly(t *testing.T) {
	deployments := []*router.Deployment{
		{
			ID:        "us-east",
			ModelName: "gpt-4o",
			Config: &config.ModelConfig{
				Tags: []string{"region:us-east", "tier:premium"},
			},
		},
		{
			ID:        "eu-west",
			ModelName: "gpt-4o",
			Config: &config.ModelConfig{
				Tags: []string{"region:eu-west", "tier:standard"},
			},
		},
		{
			ID:        "us-west",
			ModelName: "gpt-4o",
			Config: &config.ModelConfig{
				Tags: []string{"region:us-west", "tier:premium"},
			},
		},
	}

	inner := strategy.NewShuffle()
	s := strategy.NewTagBased(inner)

	// Filter by premium tier — should exclude eu-west
	picked := s.PickWithTags(deployments, []string{"tier:premium"}, false)
	require.NotNil(t, picked)
	assert.Contains(t, []string{"us-east", "us-west"}, picked.ID)

	// Filter by specific region
	picked = s.PickWithTags(deployments, []string{"region:eu-west"}, false)
	require.NotNil(t, picked)
	assert.Equal(t, "eu-west", picked.ID)

	// No matching tags — falls back to all
	picked = s.PickWithTags(deployments, []string{"nonexistent"}, false)
	require.NotNil(t, picked) // fallback picks from all
}

func TestRouter_ContextWindowFallback(t *testing.T) {
	apiKey := "sk-test"
	models := []config.ModelConfig{
		{
			ModelName: "gpt-3.5-turbo",
			TianjiParams: config.TianjiParams{
				Model:  "openai/gpt-3.5-turbo",
				APIKey: &apiKey,
			},
		},
		{
			ModelName: "gpt-4-turbo",
			TianjiParams: config.TianjiParams{
				Model:  "openai/gpt-4-turbo",
				APIKey: &apiKey,
			},
		},
	}

	r := router.New(models, strategy.NewShuffle(), router.RouterSettings{
		ContextWindowFallbacks: map[string][]string{
			"gpt-3.5-turbo": {"gpt-4-turbo"},
		},
	})

	// Context window fallback should find gpt-4-turbo
	d, p, err := r.ContextWindowFallback("gpt-3.5-turbo")
	require.NoError(t, err)
	assert.NotNil(t, d)
	assert.NotNil(t, p)
	assert.Equal(t, "gpt-4-turbo", d.ModelName)

	// No fallback configured
	_, _, err = r.ContextWindowFallback("gpt-4o")
	assert.Error(t, err)
}

func TestPolicyEngine_MatchesConditions(t *testing.T) {
	routingStrategy := "cost-optimized"
	policies := []router.Policy{
		{
			Name: "premium-team",
			Conditions: router.PolicyCondition{
				TeamAlias: strPtr("premium-*"),
			},
			Guardrails:      []string{"pii_detection"},
			RoutingStrategy: &routingStrategy,
			Priority:        10,
		},
		{
			Name: "gpt4-guardrails",
			Conditions: router.PolicyCondition{
				Model: strPtr("gpt-4*"),
			},
			Guardrails: []string{"content_moderation"},
			Priority:   5,
		},
	}

	engine := router.NewPolicyEngine(policies)

	// Match premium team
	result := engine.Evaluate(router.PolicyRequest{
		TeamAlias: "premium-enterprise",
		Model:     "gpt-4o",
	})
	assert.Equal(t, "cost-optimized", result.RoutingStrategy)
	assert.Contains(t, result.Guardrails, "pii_detection")
	assert.Contains(t, result.Guardrails, "content_moderation")

	// Match only model
	result = engine.Evaluate(router.PolicyRequest{
		TeamAlias: "basic-team",
		Model:     "gpt-4o",
	})
	assert.Empty(t, result.RoutingStrategy) // no team match
	assert.Contains(t, result.Guardrails, "content_moderation")
	assert.NotContains(t, result.Guardrails, "pii_detection")

	// No match
	result = engine.Evaluate(router.PolicyRequest{
		TeamAlias: "basic-team",
		Model:     "claude-3",
	})
	assert.Empty(t, result.RoutingStrategy)
	assert.Empty(t, result.Guardrails)
}
