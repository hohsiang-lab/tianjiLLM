package config

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"sort"
	"strings"

	"github.com/praxisllmlab/tianjiLLM/internal/provider"
)

const (
	OpenAISubscriptionTransportDirectOpenAIHTTP    = "direct_openai_http"
	OpenAISubscriptionTransportChatGPTCodexBackend = "chatgpt_codex_backend"
)

// Validate checks the config for unrecognized fields and logs warnings.
// Enables loading any Python proxy_config.yaml without errors (FR-027, FR-029).
func Validate(cfg *ProxyConfig) error {
	var validationErrs []error
	switch cfg.NativeUpstreamStrategy {
	case "", StrategyRoundRobin:
		cfg.NativeUpstreamStrategy = StrategyRoundRobin
	case StrategyLowestUtilization, StrategySticky:
		// valid
	default:
		log.Printf("[WARNING] Unrecognized native_upstream_strategy %q — resetting to round_robin", cfg.NativeUpstreamStrategy)
		cfg.NativeUpstreamStrategy = StrategyRoundRobin
	}
	if cfg.TianjiSettings.CodexUsageRefreshIntervalSeconds != nil && *cfg.TianjiSettings.CodexUsageRefreshIntervalSeconds <= 0 {
		validationErrs = append(validationErrs, fmt.Errorf("tianji_settings.codex_usage_refresh_interval_seconds must be greater than 0"))
	}
	if cfg.TianjiSettings.CodexUsageAsyncTimeoutSeconds != nil && *cfg.TianjiSettings.CodexUsageAsyncTimeoutSeconds <= 0 {
		validationErrs = append(validationErrs, fmt.Errorf("tianji_settings.codex_usage_async_timeout_seconds must be greater than 0"))
	}
	if cfg.CodexUsageWeeklyThreshold < 0 || cfg.CodexUsageWeeklyThreshold > 1 {
		validationErrs = append(validationErrs, fmt.Errorf("codex_usage_weekly_threshold must be between 0 and 1"))
	}
	if err := validateSocialAuth(cfg.GeneralSettings); err != nil {
		validationErrs = append(validationErrs, err)
	}
	warnOverflow("config", cfg.Overflow)
	warnOverflow("tianji_settings", cfg.TianjiSettings.Overflow)
	warnOverflow("general_settings", cfg.GeneralSettings.Overflow)
	if cfg.RouterSettings != nil {
		warnOverflow("router_settings", cfg.RouterSettings.Overflow)
	}
	if cfg.TianjiSettings.CacheParams != nil {
		warnOverflow("cache_params", cfg.TianjiSettings.CacheParams.Overflow)
	}
	for i := range cfg.ModelList {
		NormalizeOfficialOpenAIParams(&cfg.ModelList[i].TianjiParams)
		m := cfg.ModelList[i]
		section := fmt.Sprintf("model_list[%d].tianji_params(%s)", i, m.ModelName)
		warnOverflow(section, m.TianjiParams.Overflow)
		if err := validateOpenAISubscriptionParams(section, m.TianjiParams); err != nil {
			validationErrs = append(validationErrs, err)
		}
	}
	for i, g := range cfg.Guardrails {
		section := fmt.Sprintf("guardrails[%d](%s)", i, g.GuardrailName)
		warnOverflow(section, g.Overflow)
	}
	for i, p := range cfg.PassThroughEndpoints {
		section := fmt.Sprintf("pass_through_endpoints[%d](%s)", i, p.Path)
		warnOverflow(section, p.Overflow)
	}
	return errors.Join(validationErrs...)
}

func validateSocialAuth(settings GeneralSettings) error {
	cfg := settings.SocialAuth
	if !cfg.Enabled {
		return nil
	}

	var validationErrs []error
	if strings.TrimSpace(settings.DatabaseURL) == "" {
		validationErrs = append(validationErrs, fmt.Errorf("general_settings.social_auth.enabled requires database_url"))
	}

	baseURL := strings.TrimSpace(cfg.PublicBaseURL)
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		validationErrs = append(validationErrs, fmt.Errorf("general_settings.social_auth.public_base_url must be an absolute http or https URL"))
	} else if parsed.User != nil || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		validationErrs = append(validationErrs, fmt.Errorf("general_settings.social_auth.public_base_url must contain only scheme and host"))
	} else if parsed.Scheme == "http" &&
		parsed.Hostname() != "localhost" &&
		parsed.Hostname() != "127.0.0.1" &&
		parsed.Hostname() != "::1" {
		validationErrs = append(validationErrs, fmt.Errorf("general_settings.social_auth.public_base_url must use https unless the host is localhost"))
	}

	configuredProviders := 0
	for name, provider := range map[string]SocialProviderConfig{
		"github":  cfg.GitHub,
		"discord": cfg.Discord,
	} {
		hasClientID := strings.TrimSpace(provider.ClientID) != ""
		hasClientSecret := strings.TrimSpace(provider.ClientSecret) != ""
		if hasClientID != hasClientSecret {
			validationErrs = append(validationErrs, fmt.Errorf("general_settings.social_auth.%s requires both client_id and client_secret", name))
			continue
		}
		if hasClientID {
			configuredProviders++
		}
	}
	if configuredProviders == 0 {
		validationErrs = append(validationErrs, fmt.Errorf("general_settings.social_auth.enabled requires at least one configured provider"))
	}
	if cfg.MasterKeyLoginEnabled != nil && *cfg.MasterKeyLoginEnabled && strings.TrimSpace(settings.MasterKey) == "" {
		validationErrs = append(validationErrs, fmt.Errorf("general_settings.social_auth.master_key_login_enabled requires master_key"))
	}

	return errors.Join(validationErrs...)
}

func validateOpenAISubscriptionParams(section string, params TianjiParams) error {
	if isOfficialOpenAIModel(params.Model) {
		return nil
	}
	transport := strings.TrimSpace(params.OpenAISubscriptionTransport)
	if len(params.OpenAISubscriptionCredentialIDs) == 0 {
		if transport != "" {
			return fmt.Errorf("%s.openai_subscription_transport=%s requires openai_subscription_credential_ids", section, transport)
		}
		return nil
	}

	if transport == "" {
		return fmt.Errorf("%s.openai_subscription_transport is required when openai_subscription_credential_ids is set", section)
	}
	if !IsOpenAISubscriptionTransport(transport) {
		return fmt.Errorf("%s.openai_subscription_transport must be one of %s, %s", section, OpenAISubscriptionTransportDirectOpenAIHTTP, OpenAISubscriptionTransportChatGPTCodexBackend)
	}

	seen := make(map[string]struct{}, len(params.OpenAISubscriptionCredentialIDs))
	for _, id := range params.OpenAISubscriptionCredentialIDs {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("%s.openai_subscription_credential_ids contains empty credential ID", section)
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("%s.openai_subscription_credential_ids contains duplicate credential ID %q", section, id)
		}
		seen[id] = struct{}{}
	}

	if params.APIBase != nil && strings.TrimSpace(*params.APIBase) != "" && !isDefaultOpenAIAPIBase(*params.APIBase) {
		return fmt.Errorf("%s.openai_subscription_credential_ids cannot be combined with custom api_base", section)
	}

	providerName := openAIProviderName(params.Model)
	if providerName != "" && providerName != "openai" {
		return fmt.Errorf("%s.openai_subscription_credential_ids only supports official OpenAI models", section)
	}

	return nil
}

func IsOpenAISubscriptionTransport(transport string) bool {
	switch strings.TrimSpace(transport) {
	case OpenAISubscriptionTransportDirectOpenAIHTTP, OpenAISubscriptionTransportChatGPTCodexBackend:
		return true
	default:
		return false
	}
}

func openAIProviderName(modelName string) string {
	providerName, _, ok := strings.Cut(modelName, "/")
	if !ok {
		return ""
	}
	return providerName
}

func isOfficialOpenAIModel(modelName string) bool {
	if strings.TrimSpace(modelName) == "" {
		return false
	}
	providerName, _ := provider.ParseModelName(modelName)
	return providerName == "openai"
}

// NormalizeOfficialOpenAIParams removes legacy per-model/direct-key settings
// while retaining the fields in the decoded struct for backwards compatibility.
func NormalizeOfficialOpenAIParams(params *TianjiParams) {
	if params == nil || !isOfficialOpenAIModel(params.Model) {
		return
	}
	params.APIKey = nil
	params.APIBase = nil
	params.OpenAISubscriptionCredentialIDs = nil
	params.OpenAISubscriptionTransport = OpenAISubscriptionTransportChatGPTCodexBackend
}

func isDefaultOpenAIAPIBase(apiBase string) bool {
	normalized := strings.TrimRight(strings.TrimSpace(apiBase), "/")
	return normalized == "https://api.openai.com/v1"
}

func warnOverflow(section string, overflow map[string]any) {
	if len(overflow) == 0 {
		return
	}
	keys := make([]string, 0, len(overflow))
	for k := range overflow {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		log.Printf("[WARNING] Unrecognized config field %s.%s — field will be ignored", section, k)
	}
}
