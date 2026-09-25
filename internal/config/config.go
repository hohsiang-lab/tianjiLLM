package config

// NativeUpstreamStrategy controls how native passthrough selects among multiple OAuth upstreams.
type NativeUpstreamStrategy string

const (
	StrategyRoundRobin        NativeUpstreamStrategy = "round_robin"
	StrategyLowestUtilization NativeUpstreamStrategy = "lowest_utilization"
	StrategySticky            NativeUpstreamStrategy = "sticky"
)

// ProxyConfig represents the top-level proxy_config.yaml structure.
type ProxyConfig struct {
	ModelList            []ModelConfig         `yaml:"model_list"`
	TianjiSettings       TianjiSettings        `yaml:"tianji_settings"`
	GeneralSettings      GeneralSettings       `yaml:"general_settings"`
	RouterSettings       *RouterSettings       `yaml:"router_settings,omitempty"`
	EnvironmentVariables map[string]string     `yaml:"environment_variables,omitempty"`
	Guardrails           []GuardrailConfig     `yaml:"guardrails,omitempty"`
	PassThroughEndpoints []PassThroughEndpoint `yaml:"pass_through_endpoints,omitempty"`
	AssistantSettings    *AssistantSettings    `yaml:"assistant_settings,omitempty"`

	// MCP server configurations.
	MCPServers map[string]MCPServerConfig `yaml:"mcp_servers,omitempty"`

	// Discord rate limit alerting.
	DiscordWebhookURL         string  `yaml:"discord_webhook_url,omitempty"`
	RatelimitAlertThreshold   float64 `yaml:"ratelimit_alert_threshold,omitempty"`
	CodexUsageWeeklyThreshold float64 `yaml:"codex_usage_weekly_threshold,omitempty"`

	// Native upstream selection strategy for OAuth token routing.
	NativeUpstreamStrategy NativeUpstreamStrategy `yaml:"native_upstream_strategy,omitempty"`

	// Overflow captures any unknown top-level YAML fields.
	// Enables loading any Python proxy_config.yaml without parse errors (FR-029).
	Overflow map[string]any `yaml:",inline"`
}

// ModelConfig represents a single model entry in model_list.
type ModelConfig struct {
	ModelName    string       `yaml:"model_name"`
	TianjiParams TianjiParams `yaml:"tianji_params"`
	ModelInfo    *ModelInfo   `yaml:"model_info,omitempty"`
	Tags         []string     `yaml:"tags,omitempty"`
}

// TianjiParams holds provider-specific parameters for a model.
type TianjiParams struct {
	Model                           string   `yaml:"model"`
	APIKey                          *string  `yaml:"api_key,omitempty"`
	APIBase                         *string  `yaml:"api_base,omitempty"`
	APIVersion                      *string  `yaml:"api_version,omitempty"`
	OpenAISubscriptionCredentialIDs []string `yaml:"openai_subscription_credential_ids,omitempty"`
	OpenAISubscriptionTransport     string   `yaml:"openai_subscription_transport,omitempty"`
	TPM                             *int64   `yaml:"tpm,omitempty"`
	RPM                             *int64   `yaml:"rpm,omitempty"`
	Timeout                         *int     `yaml:"timeout,omitempty"`
	Region                          string   `yaml:"region,omitempty"`

	// AutoRouter configuration (for model prefix "auto_router/").
	AutoRouterConfig         string `yaml:"auto_router_config,omitempty"`
	AutoRouterConfigPath     string `yaml:"auto_router_config_path,omitempty"`
	AutoRouterDefaultModel   string `yaml:"auto_router_default_model,omitempty"`
	AutoRouterEmbeddingModel string `yaml:"auto_router_embedding_model,omitempty"`

	// Overflow captures provider-specific params not explicitly modeled.
	Overflow map[string]any `yaml:",inline"`
}

// ModelInfo holds optional metadata about a model.
type ModelInfo struct {
	ID              string   `yaml:"id,omitempty"`
	Mode            string   `yaml:"mode,omitempty"`
	InputCost       *float64 `yaml:"input_cost_per_token,omitempty"`
	OutputCost      *float64 `yaml:"output_cost_per_token,omitempty"`
	MaxTokens       *int     `yaml:"max_tokens,omitempty"`
	MaxInputTokens  *int     `yaml:"max_input_tokens,omitempty"`
	MaxOutputTokens *int     `yaml:"max_output_tokens,omitempty"`
}

// TianjiSettings holds global TianjiLLM behavior settings.
// Maps to tianji_settings in proxy_config.yaml.
type TianjiSettings struct {
	// Callbacks
	Callbacks []string `yaml:"callbacks,omitempty"`

	// Core behavior
	Cache                            bool         `yaml:"cache"`
	CacheParams                      *CacheParams `yaml:"cache_params,omitempty"`
	NumRetries                       *int         `yaml:"num_retries,omitempty"`
	AllowedFails                     *int         `yaml:"allowed_fails,omitempty"`
	CodexUsageRefreshIntervalSeconds *int         `yaml:"codex_usage_refresh_interval_seconds,omitempty"`
	CodexUsageAsyncTimeoutSeconds    *int         `yaml:"codex_usage_async_timeout_seconds,omitempty"`

	// Fallbacks
	Fallbacks              []map[string][]string `yaml:"fallbacks,omitempty"`
	ContextWindowFallbacks []map[string][]string `yaml:"context_window_fallbacks,omitempty"`
	DefaultFallbacks       []string              `yaml:"default_fallbacks,omitempty"`

	// Structured callback configs (alternative to string list)
	CallbackConfigs []CallbackConfig `yaml:"callback_configs,omitempty"`

	// Overflow captures any tianji_settings fields not explicitly modeled.
	Overflow map[string]any `yaml:",inline"`
}

// CacheParams holds cache configuration.
type CacheParams struct {
	Type     string `yaml:"type"`
	Mode     string `yaml:"mode,omitempty"`
	Host     string `yaml:"host,omitempty"`
	Port     int    `yaml:"port,omitempty"`
	Password string `yaml:"password,omitempty"`

	// Redis Cluster
	Addrs []string `yaml:"addrs,omitempty"`

	// Semantic cache
	EmbeddingModel string `yaml:"embedding_model,omitempty"`

	// Overflow captures cache params not explicitly modeled (S3, GCS, etc.).
	Overflow map[string]any `yaml:",inline"`
}

type SocialProviderConfig struct {
	ClientID     string `yaml:"client_id,omitempty"`
	ClientSecret string `yaml:"client_secret,omitempty"`
}

type SocialAuthConfig struct {
	Enabled                bool                 `yaml:"enabled,omitempty"`
	PublicBaseURL          string               `yaml:"public_base_url,omitempty"`
	AllowVerifiedEmailLink bool                 `yaml:"allow_verified_email_link,omitempty"`
	MasterKeyLoginEnabled  *bool                `yaml:"master_key_login_enabled,omitempty"`
	GitHub                 SocialProviderConfig `yaml:"github,omitempty"`
	Discord                SocialProviderConfig `yaml:"discord,omitempty"`
}

// GeneralSettings holds proxy server settings.
type GeneralSettings struct {
	// Core
	MasterKey   string `yaml:"master_key"`
	DatabaseURL string `yaml:"database_url,omitempty"`
	Port        int    `yaml:"port,omitempty"`

	// Rate limiting
	MaxParallelRequests *int `yaml:"max_parallel_requests,omitempty"`

	// Spend tracking
	StorePromptsInSpendLogs bool `yaml:"store_prompts_in_spend_logs"`

	// Audit logging
	StoreAuditLogs bool `yaml:"store_audit_logs"`

	// Management webhooks
	ManagementWebhookURL string `yaml:"management_webhook_url,omitempty"`

	// UI
	UISessionSecure *bool            `yaml:"ui_session_secure,omitempty"`
	SocialAuth      SocialAuthConfig `yaml:"social_auth,omitempty"`

	// Pass-through endpoints (also at top level)
	PassThroughEndpoints []PassThroughEndpoint `yaml:"pass_through_endpoints,omitempty"`

	// OpenAI OAuth subscription auth
	OpenAIOAuth OpenAIOAuthConfig `yaml:"openai_oauth,omitempty"`

	// SSO/OIDC
	SSOClientID     string            `yaml:"sso_client_id,omitempty"`
	SSOClientSecret string            `yaml:"sso_client_secret,omitempty"`
	SSOIssuerURL    string            `yaml:"sso_issuer_url,omitempty"`
	SSORedirectURI  string            `yaml:"sso_redirect_uri,omitempty"`
	SSOScopes       []string          `yaml:"sso_scopes,omitempty"`
	SSORoleMapping  map[string]string `yaml:"sso_role_mapping,omitempty"`

	// Overflow captures any general_settings fields not explicitly modeled.
	Overflow map[string]any `yaml:",inline"`
}

// RouterSettings holds load balancing configuration.
type RouterSettings struct {
	// Strategy
	RoutingStrategy string `yaml:"routing_strategy,omitempty"`

	// Retries
	NumRetries *int `yaml:"num_retries,omitempty"`
	RetryAfter *int `yaml:"retry_after,omitempty"`

	// Failure handling
	AllowedFails *int `yaml:"allowed_fails,omitempty"`
	CooldownTime *int `yaml:"cooldown_time,omitempty"`

	// Timeouts
	Timeout *int `yaml:"timeout,omitempty"`

	// Fallbacks
	Fallbacks              []map[string]any `yaml:"fallbacks,omitempty"`
	ContextWindowFallbacks []map[string]any `yaml:"context_window_fallbacks,omitempty"`
	ContentPolicyFallbacks []map[string]any `yaml:"content_policy_fallbacks,omitempty"`
	DefaultFallbacks       []string         `yaml:"default_fallbacks,omitempty"`

	// Model aliases
	ModelGroupAlias map[string]any `yaml:"model_group_alias,omitempty"`

	// Retry policies
	RetryPolicy           map[string]any `yaml:"retry_policy,omitempty"`
	ModelGroupRetryPolicy map[string]any `yaml:"model_group_retry_policy,omitempty"`

	// Tag filtering
	EnableTagFiltering   bool `yaml:"enable_tag_filtering"`
	TagFilteringMatchAny bool `yaml:"tag_filtering_match_any"`

	// Overflow captures any router_settings fields not explicitly modeled.
	Overflow map[string]any `yaml:",inline"`
}

// GuardrailConfig represents a guardrail entry in the top-level guardrails list.
type GuardrailConfig struct {
	GuardrailName string         `yaml:"guardrail_name"`
	TianjiParams  map[string]any `yaml:"tianji_params,omitempty"`
	FailurePolicy string         `yaml:"failure_policy,omitempty"` // "fail_open" or "fail_closed"

	// Overflow captures guardrail-specific fields.
	Overflow map[string]any `yaml:",inline"`
}

// CallbackConfig holds structured configuration for a callback instance.
type CallbackConfig struct {
	Type      string `yaml:"type"`
	Bucket    string `yaml:"bucket,omitempty"`
	Prefix    string `yaml:"prefix,omitempty"`
	BatchSize *int   `yaml:"batch_size,omitempty"`
	APIKey    string `yaml:"api_key,omitempty"`
	BaseURL   string `yaml:"base_url,omitempty"`
	Project   string `yaml:"project,omitempty"`
	Entity    string `yaml:"entity,omitempty"`
	Region    string `yaml:"region,omitempty"`
	QueueURL  string `yaml:"queue_url,omitempty"`
	TableName string `yaml:"table_name,omitempty"`

	// Overflow captures callback-specific fields.
	Overflow map[string]any `yaml:",inline"`
}

// AssistantSettings holds configuration for the Assistants API pass-through.
type AssistantSettings struct {
	APIBase string `yaml:"api_base"`
	APIKey  string `yaml:"api_key,omitempty"`
}

// PassThroughEndpoint defines a custom pass-through endpoint.
type PassThroughEndpoint struct {
	Path    string            `yaml:"path"`
	Target  string            `yaml:"target"`
	Headers map[string]string `yaml:"headers,omitempty"`

	// Overflow captures pass-through-specific fields.
	Overflow map[string]any `yaml:",inline"`
}

// MCPServerConfig represents an upstream MCP server defined in mcp_servers config.
type MCPServerConfig struct {
	Transport       string            `yaml:"transport"`           // "stdio", "sse", "http"
	URL             string            `yaml:"url,omitempty"`       // Required for sse/http
	Command         string            `yaml:"command,omitempty"`   // Required for stdio
	Args            []string          `yaml:"args,omitempty"`      // Command arguments
	AuthType        string            `yaml:"auth_type,omitempty"` // "api_key", "bearer_token", "basic", "oauth2"
	AuthToken       string            `yaml:"authentication_token,omitempty"`
	StaticHeaders   map[string]string `yaml:"static_headers,omitempty"`
	AllowedTools    []string          `yaml:"allowed_tools,omitempty"`
	DisallowedTools []string          `yaml:"disallowed_tools,omitempty"`
}
