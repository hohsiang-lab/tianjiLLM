package handler

import (
	"context"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
	"github.com/stretchr/testify/require"
)

func TestResolveProviderRoute_OpenAIUsesGlobalSubscriptionPool(t *testing.T) {
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"legacy-credential": {AccessToken: "legacy-access", AccountID: "legacy-account"},
		"global-a":          {AccessToken: "global-access-a", AccountID: "global-account-a"},
		"global-b":          {AccessToken: "global-access-b", AccountID: "global-account-b"},
	})
	store, ok := h.DB.(*mockStore)
	require.True(t, ok)
	store.listCredentialsFn = func(context.Context) ([]db.CredentialTable, error) {
		return []db.CredentialTable{
			{CredentialID: "global-a", CredentialType: CredentialTypeOpenAISubscription},
			{CredentialID: "global-b", CredentialType: CredentialTypeOpenAISubscription},
		}, nil
	}
	h.Config.ModelList = []config.ModelConfig{{
		ModelName: "gpt-4o",
		TianjiParams: config.TianjiParams{
			Model:                           "openai/gpt-4o",
			APIKey:                          stringPtr("legacy-api-key"),
			OpenAISubscriptionCredentialIDs: []string{"legacy-credential"},
			OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportDirectOpenAIHTTP,
		},
	}}

	route, err := h.resolveProviderFromConfigRouteWithContext(context.Background(), "gpt-4o")

	require.NoError(t, err)
	require.True(t, route.ChatGPTCodexBackend)
	require.Len(t, route.SubscriptionCandidates, 2)
	require.Equal(t, "global-a", route.SubscriptionCandidates[0].CredentialID)
	require.Equal(t, "global-b", route.SubscriptionCandidates[1].CredentialID)
	require.Empty(t, route.APIKey)
}

func TestResolveOpenAISubscriptionRouteForParams_OfficialLegacyFieldsUseGlobalCodexPool(t *testing.T) {
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"global-credential": {AccessToken: "global-access", AccountID: "global-account"},
	})
	legacyAPIKey := "[REDACTED]"

	route, err := h.resolveOpenAISubscriptionRouteForParams(context.Background(), config.TianjiParams{
		Model:                           "openai/gpt-4o",
		APIKey:                          &legacyAPIKey,
		OpenAISubscriptionCredentialIDs: []string{"legacy-credential"},
		OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportDirectOpenAIHTTP,
	})

	require.NoError(t, err)
	require.Empty(t, route.APIKey)
	require.Len(t, route.Candidates, 1)
	require.Equal(t, "global-credential", route.Candidates[0].CredentialID)
}

func TestNormalizeOfficialOpenAIParamsUsesProviderParser(t *testing.T) {
	apiKey := "legacy-api-key"
	ids := []string{"legacy-credential"}
	base := "https://platform.example/v1"
	cases := []struct {
		name     string
		model    string
		official bool
	}{
		{name: "openai qualified", model: "openai/gpt-4o", official: true},
		{name: "openai wildcard", model: "openai/*", official: true},
		{name: "bare gpt wildcard", model: "gpt-*", official: true},
		{name: "bare gpt model", model: "gpt-4o", official: true},
		{name: "openai compatible", model: "openaicompat/gpt-4o", official: false},
		{name: "ollama embedding", model: "ollama/coderank-embed", official: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeOfficialOpenAIParams(config.TianjiParams{
				Model:                           tc.model,
				APIKey:                          &apiKey,
				APIBase:                         &base,
				OpenAISubscriptionCredentialIDs: ids,
				OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportDirectOpenAIHTTP,
			})
			if tc.official {
				require.Nil(t, got.APIKey)
				require.Nil(t, got.APIBase)
				require.Nil(t, got.OpenAISubscriptionCredentialIDs)
				require.Equal(t, config.OpenAISubscriptionTransportChatGPTCodexBackend, got.OpenAISubscriptionTransport)
				return
			}
			require.Equal(t, &apiKey, got.APIKey)
			require.Equal(t, &base, got.APIBase)
			require.Equal(t, ids, got.OpenAISubscriptionCredentialIDs)
			require.Equal(t, config.OpenAISubscriptionTransportDirectOpenAIHTTP, got.OpenAISubscriptionTransport)
		})
	}
}

func TestOpenAISubscriptionCredentialInRequestScope_RejectsOrgBoundWithoutContext(t *testing.T) {
	orgID := "org-bound"
	orgBound := db.CredentialTable{
		CredentialID:   "org-bound-credential",
		CredentialType: CredentialTypeOpenAISubscription,
		OrganizationID: &orgID,
	}
	global := db.CredentialTable{
		CredentialID:   "global-credential",
		CredentialType: CredentialTypeOpenAISubscription,
	}

	require.False(t, openAISubscriptionCredentialInRequestScope(orgBound, "", false))
	require.True(t, openAISubscriptionCredentialInRequestScope(global, "", false))
	require.True(t, openAISubscriptionCredentialInRequestScope(orgBound, orgID, true))
	require.False(t, openAISubscriptionCredentialInRequestScope(orgBound, "other-org", true))
}

func TestResolveProviderRoute_OpenAIGlobalPoolFiltersOrganizations(t *testing.T) {
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"global": {AccessToken: "global-access", AccountID: "global-account"},
		"org-a":  {AccessToken: "org-a-access", AccountID: "org-a-account"},
		"org-b":  {AccessToken: "org-b-access", AccountID: "org-b-account"},
	})
	store, ok := h.DB.(*mockStore)
	require.True(t, ok)
	orgA, orgB := "org-a", "org-b"
	store.listCredentialsFn = func(context.Context) ([]db.CredentialTable, error) {
		return []db.CredentialTable{
			{CredentialID: "global", CredentialType: CredentialTypeOpenAISubscription},
			{CredentialID: "org-a", CredentialType: CredentialTypeOpenAISubscription, OrganizationID: &orgA},
			{CredentialID: "org-b", CredentialType: CredentialTypeOpenAISubscription, OrganizationID: &orgB},
		}, nil
	}
	h.Config.ModelList = []config.ModelConfig{{
		ModelName: "gpt-4o",
		TianjiParams: config.TianjiParams{
			Model: "openai/gpt-4o",
		},
	}}

	cases := []struct {
		name string
		ctx  context.Context
		want []string
	}{
		{name: "no organization", ctx: context.Background(), want: []string{"global"}},
		{name: "organization a", ctx: context.WithValue(context.Background(), middleware.ContextKeyOrgID, "org-a"), want: []string{"global", "org-a"}},
		{name: "organization b", ctx: context.WithValue(context.Background(), middleware.ContextKeyOrgID, "org-b"), want: []string{"global", "org-b"}},
		{name: "unknown organization", ctx: context.WithValue(context.Background(), middleware.ContextKeyOrgID, "org-unknown"), want: []string{"global"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			route, err := h.resolveProviderFromConfigRouteWithContext(tc.ctx, "gpt-4o")
			require.NoError(t, err)
			got := make([]string, 0, len(route.SubscriptionCandidates))
			for _, candidate := range route.SubscriptionCandidates {
				got = append(got, candidate.CredentialID)
			}
			require.Equal(t, tc.want, got)
		})
	}
}
