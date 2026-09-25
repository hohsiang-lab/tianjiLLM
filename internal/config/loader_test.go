package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMinimalConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "proxy_config.yaml")
	content := `
model_list:
  - model_name: gpt-4
    tianji_params:
      model: gpt-4
general_settings:
  master_key: sk-test
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.GeneralSettings.MasterKey != "sk-test" {
		t.Fatalf("master_key: got %q, want sk-test", cfg.GeneralSettings.MasterKey)
	}
	if cfg.GeneralSettings.Port != 4000 {
		t.Fatalf("port: got %d, want 4000 (default)", cfg.GeneralSettings.Port)
	}
	if len(cfg.ModelList) != 1 {
		t.Fatalf("model_list: got %d, want 1", len(cfg.ModelList))
	}
	if cfg.ModelList[0].ModelName != "gpt-4" {
		t.Fatalf("model_name: got %q", cfg.ModelList[0].ModelName)
	}
}

func TestLoad_LegacyModelAccessControlDoesNotBlock(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "proxy_config.yaml")
	content := `
model_list:
  - model_name: legacy-model
    tianji_params:
      model: openai/gpt-4o
    model_info:
      mode: chat
      access_control:
        allowed_orgs: [org-private]
    access_control:
      allowed_orgs: [org-private]
general_settings:
  master_key: test
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("legacy model access_control must not block config loading: %v", err)
	}
	if len(cfg.ModelList) != 1 {
		t.Fatalf("model_list: got %d, want 1", len(cfg.ModelList))
	}
	if got := cfg.ModelList[0].ModelInfo; got == nil || got.Mode != "chat" {
		t.Fatalf("supported model_info metadata was not preserved: %#v", got)
	}
}

func TestLoadWithEnvVars(t *testing.T) {
	os.Setenv("TEST_MASTER_KEY", "from-env")
	defer os.Unsetenv("TEST_MASTER_KEY")

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "proxy_config.yaml")
	content := `
model_list: []
general_settings:
  master_key: os.environ/TEST_MASTER_KEY
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GeneralSettings.MasterKey != "from-env" {
		t.Fatalf("got %q, want from-env", cfg.GeneralSettings.MasterKey)
	}
}

func TestLoad_OpenAISubscriptionCredentialIDs(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "proxy_config.yaml")
	content := `
model_list:
  - model_name: gpt-4o-sub
    tianji_params:
      model: openai/gpt-4o
      openai_subscription_credential_ids:
        - cred_a
        - cred_b
      openai_subscription_transport: direct_openai_http
general_settings:
  master_key: test
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	got := cfg.ModelList[0].TianjiParams
	if got.OpenAISubscriptionCredentialIDs != nil {
		t.Fatalf("legacy per-model IDs should be normalized away: %#v", got.OpenAISubscriptionCredentialIDs)
	}
	if got.OpenAISubscriptionTransport != OpenAISubscriptionTransportChatGPTCodexBackend {
		t.Fatalf("expected Codex transport after normalization, got %q", got.OpenAISubscriptionTransport)
	}
}

func TestLoad_OpenAISubscriptionTransportChatGPTCodexBackend(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "proxy_config.yaml")
	content := `
model_list:
  - model_name: chatgpt-gpt-5
    tianji_params:
      model: openai/gpt-5.5
      openai_subscription_credential_ids: [cred_a]
      openai_subscription_transport: chatgpt_codex_backend
general_settings:
  master_key: test
  openai_oauth:
    codex_backend_base_url: http://127.0.0.1:18080/backend-api
    codex_backend_originator: custom_origin
    codex_backend_client_version: 0.145.0
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	if got := cfg.ModelList[0].TianjiParams.OpenAISubscriptionTransport; got != OpenAISubscriptionTransportChatGPTCodexBackend {
		t.Fatalf("openai_subscription_transport: got %q", got)
	}
	if got := cfg.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL; got != "http://127.0.0.1:18080/backend-api" {
		t.Fatalf("codex_backend_base_url: got %q", got)
	}
	if got := cfg.GeneralSettings.OpenAIOAuth.CodexBackendOriginator; got != "custom_origin" {
		t.Fatalf("codex_backend_originator: got %q", got)
	}
	if got := cfg.GeneralSettings.OpenAIOAuth.CodexBackendClientVersion; got != "0.145.0" {
		t.Fatalf("codex_backend_client_version: got %q", got)
	}
}

func TestLoad_CodexUsageSettings(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "proxy_config.yaml")
	content := `
model_list: []
general_settings:
  master_key: test
codex_usage_weekly_threshold: 0.75
tianji_settings:
  codex_usage_refresh_interval_seconds: 15
  codex_usage_async_timeout_seconds: 20
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TianjiSettings.CodexUsageRefreshIntervalSeconds == nil || *cfg.TianjiSettings.CodexUsageRefreshIntervalSeconds != 15 {
		t.Fatalf("codex_usage_refresh_interval_seconds: got %#v", cfg.TianjiSettings.CodexUsageRefreshIntervalSeconds)
	}
	if cfg.TianjiSettings.CodexUsageAsyncTimeoutSeconds == nil || *cfg.TianjiSettings.CodexUsageAsyncTimeoutSeconds != 20 {
		t.Fatalf("codex_usage_async_timeout_seconds: got %#v", cfg.TianjiSettings.CodexUsageAsyncTimeoutSeconds)
	}
	if cfg.CodexUsageWeeklyThreshold != 0.75 {
		t.Fatalf("codex_usage_weekly_threshold: got %v, want 0.75", cfg.CodexUsageWeeklyThreshold)
	}
}

func TestLoad_OpenAISubscriptionIDsEmptyDropsLegacyAPIKey(t *testing.T) {
	os.Setenv("TEST_OPENAI_API_KEY", "sk-env-openai")
	defer os.Unsetenv("TEST_OPENAI_API_KEY")

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "proxy_config.yaml")
	content := `
model_list:
  - model_name: gpt-4o-key
    tianji_params:
      model: openai/gpt-4o
      api_key: os.environ/TEST_OPENAI_API_KEY
      openai_subscription_credential_ids: []
general_settings:
  master_key: test
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.ModelList[0].TianjiParams.APIKey != nil {
		t.Fatalf("official OpenAI api_key should be normalized away: got %#v", cfg.ModelList[0].TianjiParams.APIKey)
	}
	if cfg.ModelList[0].TianjiParams.OpenAISubscriptionTransport != OpenAISubscriptionTransportChatGPTCodexBackend {
		t.Fatalf("expected Codex transport after normalization: got %q", cfg.ModelList[0].TianjiParams.OpenAISubscriptionTransport)
	}
	if len(cfg.ModelList[0].TianjiParams.OpenAISubscriptionCredentialIDs) != 0 {
		t.Fatalf("empty subscription IDs should stay empty: got %#v", cfg.ModelList[0].TianjiParams.OpenAISubscriptionCredentialIDs)
	}
}

func TestLoad_NormalizesOfficialOpenAICustomBaseAndLegacyIDs(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "proxy_config.yaml")
	content := `
model_list:
  - model_name: gpt-4o-sub
    tianji_params:
      model: openai/gpt-4o
      api_base: https://proxy.example.com/v1
      openai_subscription_credential_ids: [cred_a]
      openai_subscription_transport: direct_openai_http
general_settings:
  master_key: test
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("expected legacy official OpenAI config to normalize, got %v", err)
	}
	params := cfg.ModelList[0].TianjiParams
	if params.APIBase != nil || params.OpenAISubscriptionCredentialIDs != nil {
		t.Fatalf("expected custom base and legacy IDs to be removed: %#v", params)
	}
	if params.OpenAISubscriptionTransport != OpenAISubscriptionTransportChatGPTCodexBackend {
		t.Fatalf("expected Codex transport, got %q", params.OpenAISubscriptionTransport)
	}
}

func TestValidate_OpenAISubscriptionRejectsCustomProvider(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "proxy_config.yaml")
	content := `
model_list:
  - model_name: azure-gpt
    tianji_params:
      model: azure/gpt-4o
      openai_subscription_credential_ids: [cred_a]
      openai_subscription_transport: direct_openai_http
general_settings:
  master_key: test
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(cfgPath)
	if err == nil || !strings.Contains(err.Error(), "official OpenAI") {
		t.Fatalf("expected official OpenAI validation error, got %v", err)
	}
}

func TestLoad_NormalizesOfficialOpenAIAnyLegacyCredentialIDs(t *testing.T) {
	cases := []struct {
		name    string
		idsYAML string
		want    string
	}{
		{name: "empty", idsYAML: `[""]`, want: "empty credential ID"},
		{name: "duplicate", idsYAML: `[cred_a, cred_a]`, want: "duplicate credential ID"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			cfgPath := filepath.Join(dir, "proxy_config.yaml")
			content := `
model_list:
  - model_name: gpt-4o-sub
    tianji_params:
      model: openai/gpt-4o
      openai_subscription_credential_ids: ` + tc.idsYAML + `
      openai_subscription_transport: direct_openai_http
general_settings:
  master_key: test
`
			if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}

			cfg, err := Load(cfgPath)
			if err != nil {
				t.Fatalf("expected legacy IDs to normalize, got %v", err)
			}
			params := cfg.ModelList[0].TianjiParams
			if params.OpenAISubscriptionCredentialIDs != nil || params.OpenAISubscriptionTransport != OpenAISubscriptionTransportChatGPTCodexBackend {
				t.Fatalf("expected normalized Codex/global params, got %#v", params)
			}
		})
	}
}

func TestLoadWithEnvironmentVariablesSection(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "proxy_config.yaml")
	content := `
model_list: []
general_settings:
  master_key: test
environment_variables:
  MY_CUSTOM_VAR: hello_world
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("MY_CUSTOM_VAR") != "hello_world" {
		t.Fatalf("env var not set: got %q", os.Getenv("MY_CUSTOM_VAR"))
	}
	os.Unsetenv("MY_CUSTOM_VAR")
}

func TestLoadWithOpenAIOAuthConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "proxy_config.yaml")
	content := `
model_list: []
general_settings:
  master_key: test
  openai_oauth:
    enabled: true
    issuer_url: http://127.0.0.1:18080
    authorize_url: http://127.0.0.1:18080/custom/authorize
    token_url: http://127.0.0.1:18080/custom/token
    redirect_uri: http://localhost:1455/auth/callback
    client_id: app_test_override
    scopes:
      - openid
      - email
    originator: tianjillm-test
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	oauth := cfg.GeneralSettings.OpenAIOAuth
	if !oauth.Enabled {
		t.Fatal("openai_oauth.enabled should be true")
	}
	if oauth.AuthorizeURL != "http://127.0.0.1:18080/custom/authorize" {
		t.Fatalf("authorize_url: got %q", oauth.AuthorizeURL)
	}
	if oauth.TokenURL != "http://127.0.0.1:18080/custom/token" {
		t.Fatalf("token_url: got %q", oauth.TokenURL)
	}
	if oauth.RedirectURI != "http://localhost:1455/auth/callback" {
		t.Fatalf("redirect_uri: got %q", oauth.RedirectURI)
	}
	if oauth.ClientID != "app_test_override" {
		t.Fatalf("client_id: got %q", oauth.ClientID)
	}
	if len(oauth.Scopes) != 2 || oauth.Scopes[0] != "openid" || oauth.Scopes[1] != "email" {
		t.Fatalf("scopes: got %#v", oauth.Scopes)
	}
	if oauth.Originator != "tianjillm-test" {
		t.Fatalf("originator: got %q", oauth.Originator)
	}
}

func TestLoadWithOpenAIOAuthConfig_DefaultsLocalhostRedirect(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "proxy_config.yaml")
	content := `
model_list: []
general_settings:
  master_key: test
  openai_oauth:
    enabled: true
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	redirectURI, err := ResolveOpenAIOAuthRedirectURI(cfg.GeneralSettings.OpenAIOAuth)
	if err != nil {
		t.Fatal(err)
	}
	if redirectURI != "http://localhost:1455/auth/callback" {
		t.Fatalf("redirect_uri: got %q", redirectURI)
	}
}

func TestLoadInvalidPath(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(cfgPath, []byte("{{invalid yaml"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadWithOverflowFields(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "proxy_config.yaml")
	content := `
model_list: []
general_settings:
  master_key: test
  unknown_field: value
unknown_top_level: true
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Overflow) == 0 {
		t.Fatal("expected overflow fields")
	}
}

func TestLoadWithRemovedTypedFieldsPreservesOverflow(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "proxy_config.yaml")
	content := `
model_list: []
tianji_settings:
  cache: true
  drop_params: true
  cache_params:
    type: redis
    ttl: 30
general_settings:
  master_key: test
  health_check_interval: 15
router_settings:
  max_retries: 2
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.TianjiSettings.Overflow["drop_params"] != true {
		t.Fatalf("drop_params should remain in overflow: %#v", cfg.TianjiSettings.Overflow)
	}
	if cfg.TianjiSettings.CacheParams == nil || cfg.TianjiSettings.CacheParams.Type != "redis" {
		t.Fatalf("active cache type should remain typed: %#v", cfg.TianjiSettings.CacheParams)
	}
	if cfg.TianjiSettings.CacheParams.Overflow["ttl"] != 30 {
		t.Fatalf("cache ttl should remain in overflow: %#v", cfg.TianjiSettings.CacheParams.Overflow)
	}
	if cfg.GeneralSettings.Overflow["health_check_interval"] != 15 {
		t.Fatalf("health_check_interval should remain in overflow: %#v", cfg.GeneralSettings.Overflow)
	}
	if cfg.RouterSettings == nil || cfg.RouterSettings.Overflow["max_retries"] != 2 {
		t.Fatalf("max_retries should remain in overflow: %#v", cfg.RouterSettings)
	}
}

func TestLoadWithCacheParams(t *testing.T) {
	os.Setenv("TEST_CACHE_PW", "cachepw")
	defer os.Unsetenv("TEST_CACHE_PW")

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "proxy_config.yaml")
	content := `
model_list: []
general_settings:
  master_key: test
tianji_settings:
  cache: true
  cache_params:
    type: redis
    password: os.environ/TEST_CACHE_PW
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TianjiSettings.CacheParams == nil {
		t.Fatal("cache_params should not be nil")
	}
	if cfg.TianjiSettings.CacheParams.Password != "cachepw" {
		t.Fatalf("cache password: got %q", cfg.TianjiSettings.CacheParams.Password)
	}
}

func TestValidate(t *testing.T) {
	cfg := &ProxyConfig{
		Overflow: map[string]any{"unknown": true},
		ModelList: []ModelConfig{
			{
				ModelName: "test",
				TianjiParams: TianjiParams{
					Overflow: map[string]any{"custom_param": "val"},
				},
			},
		},
		Guardrails: []GuardrailConfig{
			{GuardrailName: "g1", Overflow: map[string]any{"x": 1}},
		},
		PassThroughEndpoints: []PassThroughEndpoint{
			{Path: "/p", Overflow: map[string]any{"y": 2}},
		},
		RouterSettings: &RouterSettings{
			Overflow: map[string]any{"z": 3},
		},
		TianjiSettings: TianjiSettings{
			CacheParams: &CacheParams{
				Overflow: map[string]any{"w": 4},
			},
		},
	}
	// Should not panic
	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}
