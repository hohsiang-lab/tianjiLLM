package router

import (
	"context"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRouter_AccessControl_FiltersByOrg(t *testing.T) {
	apiKey := "sk-test"
	models := []config.ModelConfig{
		{
			ModelName:    "gpt-4o",
			TianjiParams: config.TianjiParams{Model: "openai/gpt-4o", APIKey: &apiKey},
			// public deployment
		},
		{
			ModelName:    "gpt-4o",
			TianjiParams: config.TianjiParams{Model: "openai/gpt-4o", APIKey: &apiKey},
			AccessControl: &config.AccessControl{
				AllowedOrgs: []string{"org_acme"},
			},
		},
	}

	r := New(models, &roundRobinStrategy{}, RouterSettings{})
	req := &model.ChatCompletionRequest{Model: "gpt-4o"}

	// Caller from org_acme can access both deployments
	ctx := context.WithValue(context.Background(), middleware.ContextKeyOrgID, "org_acme")
	ctx = context.WithValue(ctx, middleware.ContextKeyIsMasterKey, false)
	d, _, err := r.Route(ctx, "gpt-4o", req)
	require.NoError(t, err)
	assert.NotNil(t, d)

	// Caller from org_other can only access public deployment
	ctx2 := context.WithValue(context.Background(), middleware.ContextKeyOrgID, "org_other")
	ctx2 = context.WithValue(ctx2, middleware.ContextKeyIsMasterKey, false)
	d2, _, err := r.Route(ctx2, "gpt-4o", req)
	require.NoError(t, err)
	assert.NotNil(t, d2)
}

func TestRouter_AccessControl_AllRestricted_NoAccess(t *testing.T) {
	apiKey := "sk-test"
	models := []config.ModelConfig{
		{
			ModelName:    "gpt-4o",
			TianjiParams: config.TianjiParams{Model: "openai/gpt-4o", APIKey: &apiKey},
			AccessControl: &config.AccessControl{
				AllowedOrgs: []string{"org_acme"},
			},
		},
	}

	r := New(models, &roundRobinStrategy{}, RouterSettings{})
	req := &model.ChatCompletionRequest{Model: "gpt-4o"}

	// Unauthorized caller
	ctx := context.WithValue(context.Background(), middleware.ContextKeyOrgID, "org_other")
	ctx = context.WithValue(ctx, middleware.ContextKeyIsMasterKey, false)
	_, _, err := r.Route(ctx, "gpt-4o", req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no accessible deployments")
}

func TestRouter_AccessControl_MasterKeyBypasses(t *testing.T) {
	apiKey := "sk-test"
	models := []config.ModelConfig{
		{
			ModelName:    "gpt-4o",
			TianjiParams: config.TianjiParams{Model: "openai/gpt-4o", APIKey: &apiKey},
			AccessControl: &config.AccessControl{
				AllowedOrgs: []string{"org_acme"},
			},
		},
	}

	r := New(models, &roundRobinStrategy{}, RouterSettings{})
	req := &model.ChatCompletionRequest{Model: "gpt-4o"}

	// Master key should bypass
	ctx := context.WithValue(context.Background(), middleware.ContextKeyIsMasterKey, true)
	d, _, err := r.Route(ctx, "gpt-4o", req)
	require.NoError(t, err)
	assert.NotNil(t, d)
}
