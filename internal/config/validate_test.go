package config

import (
	"strings"
	"testing"
)

func TestValidateNormalizesOfficialOpenAIParamsToGlobalCodex(t *testing.T) {
	apiKey := "legacy-api-key"
	apiBase := "https://proxy.example.com/v1"
	cfg := &ProxyConfig{
		ModelList: []ModelConfig{{
			ModelName: "gpt-4o",
			TianjiParams: TianjiParams{
				Model:                           "openai/gpt-4o",
				APIKey:                          &apiKey,
				APIBase:                         &apiBase,
				OpenAISubscriptionCredentialIDs: []string{"legacy-credential"},
				OpenAISubscriptionTransport:     OpenAISubscriptionTransportDirectOpenAIHTTP,
			},
		}},
	}

	if err := Validate(cfg); err != nil {
		t.Fatalf("expected legacy official OpenAI fields to normalize, got %v", err)
	}
	params := cfg.ModelList[0].TianjiParams
	if params.APIKey != nil || params.APIBase != nil || params.OpenAISubscriptionCredentialIDs != nil {
		t.Fatalf("expected direct key/base and per-model IDs to be removed: %#v", params)
	}
	if params.OpenAISubscriptionTransport != OpenAISubscriptionTransportChatGPTCodexBackend {
		t.Fatalf("expected Codex transport, got %q", params.OpenAISubscriptionTransport)
	}
}

func TestValidateOfficialOpenAIDoesNotRequirePerModelCredentials(t *testing.T) {
	for _, modelName := range []string{"openai/gpt-5.5", "openai/*", "gpt-*", "gpt-4o"} {
		t.Run(modelName, func(t *testing.T) {
			if err := validateOpenAISubscriptionParams("model_list[0].tianji_params", TianjiParams{Model: modelName}); err != nil {
				t.Fatalf("expected global-pool model to pass without IDs, got %v", err)
			}
		})
	}
}

func TestValidateCompatibilityProviderKeepsGenericFields(t *testing.T) {
	apiKey := "compat-api-key"
	apiBase := "https://proxy.example.com/v1"
	params := TianjiParams{Model: "openaicompat/gpt-4o", APIKey: &apiKey, APIBase: &apiBase}
	if err := validateOpenAISubscriptionParams("model_list[0].tianji_params", params); err != nil {
		t.Fatalf("expected compatibility model without subscription fields to pass, got %v", err)
	}
	if params.APIKey == nil || *params.APIKey != apiKey || params.APIBase == nil || *params.APIBase != apiBase {
		t.Fatalf("generic fields changed unexpectedly: %#v", params)
	}
}

func TestValidateRejectsSubscriptionFieldsForCompatibilityProvider(t *testing.T) {
	err := validateOpenAISubscriptionParams("model_list[0].tianji_params", TianjiParams{
		Model:                           "openaicompat/gpt-4o",
		OpenAISubscriptionCredentialIDs: []string{"cred_a"},
		OpenAISubscriptionTransport:     OpenAISubscriptionTransportChatGPTCodexBackend,
	})
	if err == nil || !strings.Contains(err.Error(), "only supports official OpenAI models") {
		t.Fatalf("expected compatibility subscription validation error, got %v", err)
	}
}

func TestValidateCodexUsageRefreshIntervalSecondsRejectsNonPositive(t *testing.T) {
	zero := 0
	err := Validate(&ProxyConfig{
		TianjiSettings: TianjiSettings{
			CodexUsageRefreshIntervalSeconds: &zero,
		},
	})

	if err == nil || !strings.Contains(err.Error(), "codex_usage_refresh_interval_seconds must be greater than 0") {
		t.Fatalf("expected codex usage refresh interval validation error, got %v", err)
	}
}

func TestValidateCodexUsageAsyncTimeoutSecondsRejectsNonPositive(t *testing.T) {
	zero := 0
	err := Validate(&ProxyConfig{
		TianjiSettings: TianjiSettings{
			CodexUsageAsyncTimeoutSeconds: &zero,
		},
	})

	if err == nil || !strings.Contains(err.Error(), "codex_usage_async_timeout_seconds must be greater than 0") {
		t.Fatalf("expected codex usage async timeout validation error, got %v", err)
	}
}

func TestValidateCodexUsageWeeklyThresholdRejectsOutOfRange(t *testing.T) {
	for _, value := range []float64{-0.1, 1.1} {
		err := Validate(&ProxyConfig{CodexUsageWeeklyThreshold: value})
		if err == nil || !strings.Contains(err.Error(), "codex_usage_weekly_threshold must be between 0 and 1") {
			t.Fatalf("expected codex usage weekly threshold validation error for %v, got %v", value, err)
		}
	}
}

func TestValidateCodexUsageWeeklyThresholdAllowsUnsetDefault(t *testing.T) {
	if err := Validate(&ProxyConfig{}); err != nil {
		t.Fatalf("expected unset codex usage weekly threshold to pass validation, got %v", err)
	}
}
