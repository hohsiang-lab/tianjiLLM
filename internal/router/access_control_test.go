package router

import (
	"context"
	"errors"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
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
	assert.True(t, errors.Is(err, ErrAccessDenied))
}

func TestRouter_AccessControl_TeamOnly(t *testing.T) {
	apiKey := "sk-test"
	models := []config.ModelConfig{
		{
			ModelName:    "gpt-4o",
			TianjiParams: config.TianjiParams{Model: "openai/gpt-4o", APIKey: &apiKey},
			AccessControl: &config.AccessControl{
				AllowedTeams: []string{"team_ml"},
			},
		},
	}

	r := New(models, &roundRobinStrategy{}, RouterSettings{})
	req := &model.ChatCompletionRequest{Model: "gpt-4o"}

	// Matching team
	ctx := context.WithValue(context.Background(), middleware.ContextKeyTeamID, "team_ml")
	ctx = context.WithValue(ctx, middleware.ContextKeyIsMasterKey, false)
	d, _, err := r.Route(ctx, "gpt-4o", req)
	require.NoError(t, err)
	assert.NotNil(t, d)

	// Non-matching team
	ctx2 := context.WithValue(context.Background(), middleware.ContextKeyTeamID, "team_other")
	ctx2 = context.WithValue(ctx2, middleware.ContextKeyIsMasterKey, false)
	_, _, err = r.Route(ctx2, "gpt-4o", req)
	assert.Error(t, err)
}

func TestRouter_AccessControl_KeyOnly(t *testing.T) {
	apiKey := "sk-test"
	models := []config.ModelConfig{
		{
			ModelName:    "gpt-4o",
			TianjiParams: config.TianjiParams{Model: "openai/gpt-4o", APIKey: &apiKey},
			AccessControl: &config.AccessControl{
				AllowedKeys: []string{"sk-hash-abc"},
			},
		},
	}

	r := New(models, &roundRobinStrategy{}, RouterSettings{})
	req := &model.ChatCompletionRequest{Model: "gpt-4o"}

	// Matching key
	ctx := context.WithValue(context.Background(), middleware.ContextKeyTokenHash, "sk-hash-abc")
	ctx = context.WithValue(ctx, middleware.ContextKeyIsMasterKey, false)
	d, _, err := r.Route(ctx, "gpt-4o", req)
	require.NoError(t, err)
	assert.NotNil(t, d)

	// Non-matching key
	ctx2 := context.WithValue(context.Background(), middleware.ContextKeyTokenHash, "sk-other")
	ctx2 = context.WithValue(ctx2, middleware.ContextKeyIsMasterKey, false)
	_, _, err = r.Route(ctx2, "gpt-4o", req)
	assert.Error(t, err)
}

func TestRouter_ListModelGroups_ACFilter(t *testing.T) {
	apiKey := "sk-test"
	models := []config.ModelConfig{
		{
			ModelName:    "gpt-4o",
			TianjiParams: config.TianjiParams{Model: "openai/gpt-4o", APIKey: &apiKey},
			// public
		},
		{
			ModelName:    "claude",
			TianjiParams: config.TianjiParams{Model: "openai/gpt-4o", APIKey: &apiKey},
			AccessControl: &config.AccessControl{
				AllowedOrgs: []string{"org_acme"},
			},
		},
	}

	r := New(models, &roundRobinStrategy{}, RouterSettings{})

	// Unauthorized caller only sees public model
	ctx := context.WithValue(context.Background(), middleware.ContextKeyOrgID, "org_other")
	ctx = context.WithValue(ctx, middleware.ContextKeyIsMasterKey, false)
	groups := r.ListModelGroups(ctx)
	assert.Contains(t, groups, "gpt-4o")
	assert.NotContains(t, groups, "claude")

	// Authorized caller sees both
	ctx2 := context.WithValue(context.Background(), middleware.ContextKeyOrgID, "org_acme")
	ctx2 = context.WithValue(ctx2, middleware.ContextKeyIsMasterKey, false)
	groups2 := r.ListModelGroups(ctx2)
	assert.Contains(t, groups2, "gpt-4o")
	assert.Contains(t, groups2, "claude")

	// Master key sees all
	ctx3 := context.WithValue(context.Background(), middleware.ContextKeyIsMasterKey, true)
	groups3 := r.ListModelGroups(ctx3)
	assert.Contains(t, groups3, "gpt-4o")
	assert.Contains(t, groups3, "claude")
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
