package config

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

	// Metrics configuration.
	Metrics MetricsConfig `yaml:"metrics,omitempty"`

	// MCP server configurations.
	MCPServers map[string]MCPServerConfig `yaml:"mcp_servers,omitempty"`

	// Search tool configurations.
	SearchTools []SearchToolConfig `yaml:"search_tools,omitempty"`

	// Discord rate limit alerting.
	DiscordWebhookURL       string  `yaml:"discord_webhook_url,omitempty"`
	RatelimitAlertThreshold float64 `yaml:"ratelimit_alert_threshold,omitempty"`

	// Overflow captures any unknown top-level YAML fields.
	// Enables loading any Python proxy_config.yaml without parse errors (FR-029).
	Overflow map[string]any `yaml:",inline"`
}

// AccessControl restricts which callers can use a deployment.
// All fields use OR logic: if the caller matches ANY field, access is granted.
// Nil/empty struct = public (no restrictions).
type AccessControl struct {
	AllowedOrgs  []string `yaml:"allowed_orgs,omitempty" json:"allowed_orgs,omitempty"`
	AllowedTeams []string `yaml:"allowed_teams,omitempty" json:"allowed_teams,omitempty"`
	AllowedKeys  []string `yaml:"allowed_keys,omitempty" json:"allowed_keys,omitempty"`
}

func (ac *AccessControl) IsPublic() bool {
	return ac == nil || (len(ac.AllowedOrgs) == 0 && len(ac.AllowedTeams) == 0 && len(ac.AllowedKeys) == 0)
}

// IsAllowed uses OR semantics: matches ANY org, team, or key hash.
func (ac *AccessControl) IsAllowed(orgID, teamID, tokenHash string) bool {
	if ac.IsPublic() {
		return true
	}
	for _, o := range ac.AllowedOrgs {
		if o == orgID {
			return true
		}
	}
	for _, t := range ac.AllowedTeams {
		if t == teamID {
			return true
		}
	}
	for _, k := range ac.AllowedKeys {
		if k == tokenHash {
			return true
		}
	}
	return false
}

// ModelConfig represents a single model entry in model_list.
type ModelConfig struct {
	ModelName     string         `yaml:"model_name"`
	TianjiParams  TianjiParams   `yaml:"tianji_params"`
	ModelInfo     *ModelInfo     `yaml:"model_info,omitempty"`
	Tags          []string       `yaml:"tags,omitempty"`
	AccessControl *AccessControl `yaml:"access_control,omitempty"`
}

// TianjiParams holds provider-specific parameters for a model.
type TianjiParams struct {
	Model      string  `yaml:"model"`
	APIKey     *string `yaml:"api_key,omitempty"`
	APIBase    *string `yaml:"api_base,omitempty"`
	APIVersion *string `yaml:"api_version,omitempty"`
	TPM        *int64  `yaml:"tpm,omitempty"`
	RPM        *int64  `yaml:"rpm,omitempty"`
	Timeout    *int    `yaml:"timeout,omitempty"`
	Region     string  `yaml:"region,omitempty"`

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
	Callbacks       []string `yaml:"callbacks,omitempty"`
	SuccessCallback []string `yaml:"success_callback,omitempty"`
	FailureCallback []string `yaml:"failure_callback,omitempty"`

	// Core behavior
	DropParams     bool         `yaml:"drop_params"`
	Cache          bool         `yaml:"cache"`
	CacheParams    *CacheParams `yaml:"cache_params,omitempty"`
	SetVerbose     bool         `yaml:"set_verbose"`
	NumRetries     *int         `yaml:"num_retries,omitempty"`
	RequestTimeout *int         `yaml:"request_timeout,omitempty"`
	AllowedFails   *int         `yaml:"allowed_fails,omitempty"`
	JSONLogs       bool         `yaml:"json_logs"`

	// Fallbacks
	Fallbacks              []map[string][]string `yaml:"fallbacks,omitempty"`
	ContextWindowFallbacks []map[string][]string `yaml:"context_window_fallbacks,omitempty"`
	DefaultFallbacks       []string              `yaml:"default_fallbacks,omitempty"`

	// Key generation limits
	UpperboundKeyGenerateParams map[string]any `yaml:"upperbound_key_generate_params,omitempty"`

	// Custom provider map
	CustomProviderMap []map[string]any `yaml:"custom_provider_map,omitempty"`

	// Structured callback configs (alternative to string list)
	CallbackConfigs []CallbackConfig `yaml:"callback_configs,omitempty"`

	// Default team settings
	DefaultTeamSettings []map[string]any `yaml:"default_team_settings,omitempty"`

	// Budget limits
	MaxInternalUserBudget        *float64 `yaml:"max_internal_user_budget,omitempty"`
	DefaultMaxInternalUserBudget *float64 `yaml:"default_max_internal_user_budget,omitempty"`

	// Overflow captures any tianji_settings fields not explicitly modeled.
	Overflow map[string]any `yaml:",inline"`
}

// CacheParams holds cache configuration.
type CacheParams struct {
	Type      string `yaml:"type"`
	Mode      string `yaml:"mode,omitempty"`
	Host      string `yaml:"host,omitempty"`
	Port      int    `yaml:"port,omitempty"`
	Password  string `yaml:"password,omitempty"`
	TTL       *int   `yaml:"ttl,omitempty"`
	Namespace string `yaml:"namespace,omitempty"`

	// Redis-specific
	DefaultInMemoryTTL *float64 `yaml:"default_in_memory_ttl,omitempty"`
	DefaultInRedisTTL  *float64 `yaml:"default_in_redis_ttl,omitempty"`
	RedisFlushSize     *int     `yaml:"redis_flush_size,omitempty"`

	// Redis Cluster
	Addrs []string `yaml:"addrs,omitempty"`

	// Semantic cache
	EmbeddingModel string `yaml:"embedding_model,omitempty"`

	// Overflow captures cache params not explicitly modeled (S3, GCS, etc.).
	Overflow map[string]any `yaml:",inline"`
}

// GeneralSettings holds proxy server settings.
type GeneralSettings struct {
	// Core
	MasterKey   string `yaml:"master_key"`
	DatabaseURL string `yaml:"database_url,omitempty"`
	Port        int    `yaml:"port,omitempty"`

	// Database
	DatabaseConnectionPoolLimit *int     `yaml:"database_connection_pool_limit,omitempty"`
	DatabaseConnectionTimeout   *float64 `yaml:"database_connection_timeout,omitempty"`

	// Auth
	CustomAuth     string `yaml:"custom_auth,omitempty"`
	AuthHeaderName string `yaml:"auth_header_name,omitempty"`

	// Rate limiting
	MaxParallelRequests                        *int `yaml:"max_parallel_requests,omitempty"`
	GlobalMaxParallelRequests                  *int `yaml:"global_max_parallel_requests,omitempty"`
	MaxRequestSizeMB                           *int `yaml:"max_request_size_mb,omitempty"`
	MaxResponseSizeMB                          *int `yaml:"max_response_size_mb,omitempty"`
	DisableRetryOnMaxParallelRequestLimitError bool `yaml:"disable_retry_on_max_parallel_request_limit_error"`

	// Health checks
	BackgroundHealthChecks bool `yaml:"background_health_checks"`
	HealthCheckInterval    *int `yaml:"health_check_interval,omitempty"`
	HealthCheckDetails     bool `yaml:"health_check_details"`

	// Spend tracking
	DisableSpendLogs        bool `yaml:"disable_spend_logs"`
	StorePromptsInSpendLogs bool `yaml:"store_prompts_in_spend_logs"`

	// Audit logging
	StoreAuditLogs bool `yaml:"store_audit_logs"`

	// Budget scheduler
	ProxyBudgetReschedulerMinTime *int `yaml:"proxy_budget_rescheduler_min_time,omitempty"`
	ProxyBudgetReschedulerMaxTime *int `yaml:"proxy_budget_rescheduler_max_time,omitempty"`
	ProxyBatchWriteAt             *int `yaml:"proxy_batch_write_at,omitempty"`

	// Alerting
	Alerting          []string          `yaml:"alerting,omitempty"`
	AlertingThreshold *int              `yaml:"alerting_threshold,omitempty"`
	AlertTypes        []string          `yaml:"alert_types,omitempty"`
	AlertToWebhookURL map[string]string `yaml:"alert_to_webhook_url,omitempty"`
	AlertingArgs      map[string]any    `yaml:"alerting_args,omitempty"`

	// Management webhooks
	ManagementWebhookURL string `yaml:"management_webhook_url,omitempty"`

	// UI
	UIAccessMode string `yaml:"ui_access_mode,omitempty"`

	// JWT
	TianjiJWTAuth map[string]any `yaml:"tianji_jwtauth,omitempty"`

	// RBAC
	RolePermissions []map[string]any `yaml:"role_permissions,omitempty"`

	// Pass-through endpoints (also at top level)
	PassThroughEndpoints []PassThroughEndpoint `yaml:"pass_through_endpoints,omitempty"`

	// Completion model
	CompletionModel string `yaml:"completion_model,omitempty"`

	// Secret manager
	SecretManager *SecretManagerConfig `yaml:"secret_manager,omitempty"`

	// Prompt management
	PromptManagement *PromptManagementConfig `yaml:"prompt_management,omitempty"`

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
	RoutingStrategy     string         `yaml:"routing_strategy,omitempty"`
	RoutingStrategyArgs map[string]any `yaml:"routing_strategy_args,omitempty"`

	// Retries
	NumRetries *int `yaml:"num_retries,omitempty"`
	MaxRetries *int `yaml:"max_retries,omitempty"`
	RetryAfter *int `yaml:"retry_after,omitempty"`

	// Failure handling
	AllowedFails     *int `yaml:"allowed_fails,omitempty"`
	CooldownTime     *int `yaml:"cooldown_time,omitempty"`
	DisableCooldowns bool `yaml:"disable_cooldowns"`

	// Timeouts
	Timeout       *int `yaml:"timeout,omitempty"`
	StreamTimeout *int `yaml:"stream_timeout,omitempty"`

	// Fallbacks
	Fallbacks              []map[string]any `yaml:"fallbacks,omitempty"`
	ContextWindowFallbacks []map[string]any `yaml:"context_window_fallbacks,omitempty"`
	ContentPolicyFallbacks []map[string]any `yaml:"content_policy_fallbacks,omitempty"`
	DefaultFallbacks       []string         `yaml:"default_fallbacks,omitempty"`
	MaxFallbacks           *int             `yaml:"max_fallbacks,omitempty"`

	// Model aliases
	ModelGroupAlias map[string]any `yaml:"model_group_alias,omitempty"`

	// Retry policies
	RetryPolicy           map[string]any `yaml:"retry_policy,omitempty"`
	ModelGroupRetryPolicy map[string]any `yaml:"model_group_retry_policy,omitempty"`

	// Tag filtering
	EnableTagFiltering   bool `yaml:"enable_tag_filtering"`
	TagFilteringMatchAny bool `yaml:"tag_filtering_match_any"`

	// Budget
	ProviderBudgetConfig map[string]any `yaml:"provider_budget_config,omitempty"`
	AlertingConfig       map[string]any `yaml:"alerting_config,omitempty"`

	// Defaults
	DefaultTianjiParams        map[string]any `yaml:"default_tianji_params,omitempty"`
	DefaultMaxParallelRequests *int           `yaml:"default_max_parallel_requests,omitempty"`

	// Other
	EnablePreCallChecks      bool `yaml:"enable_pre_call_checks"`
	SetVerbose               bool `yaml:"set_verbose"`
	IgnoreInvalidDeployments bool `yaml:"ignore_invalid_deployments"`

	// Overflow captures any router_settings fields not explicitly modeled.
	Overflow map[string]any `yaml:",inline"`
}

// GuardrailConfig represents a guardrail entry in the top-level guardrails list.
type GuardrailConfig struct {
	GuardrailName string         `yaml:"guardrail_name"`
	TianjiParams  map[string]any `yaml:"tianji_params,omitempty"`
	DefaultOn     bool           `yaml:"default_on"`
	FailurePolicy string         `yaml:"failure_policy,omitempty"` // "fail_open" or "fail_closed"

	// Overflow captures guardrail-specific fields.
	Overflow map[string]any `yaml:",inline"`
}

// SecretManagerConfig holds configuration for external secret managers.
type SecretManagerConfig struct {
	Type      string `yaml:"type"`                 // aws_secrets_manager, google_secret_manager, azure_key_vault, hashicorp_vault
	Region    string `yaml:"region,omitempty"`     // AWS region
	ProjectID string `yaml:"project_id,omitempty"` // GCP project ID
	VaultURL  string `yaml:"vault_url,omitempty"`  // Vault/Azure Key Vault URL
	CacheTTL  *int   `yaml:"cache_ttl,omitempty"`  // Secret cache TTL in seconds (default 86400)
}

// PromptManagementConfig holds configuration for external prompt sources.
type PromptManagementConfig struct {
	Type      string `yaml:"type"` // langfuse
	PublicKey string `yaml:"public_key,omitempty"`
	SecretKey string `yaml:"secret_key,omitempty"`
	BaseURL   string `yaml:"base_url,omitempty"`
}

// CallbackConfig holds structured configuration for a callback instance.
type CallbackConfig struct {
	Type          string `yaml:"type"`
	Bucket        string `yaml:"bucket,omitempty"`
	Prefix        string `yaml:"prefix,omitempty"`
	FlushInterval *int   `yaml:"flush_interval,omitempty"`
	BatchSize     *int   `yaml:"batch_size,omitempty"`
	APIKey        string `yaml:"api_key,omitempty"`
	BaseURL       string `yaml:"base_url,omitempty"`
	Project       string `yaml:"project,omitempty"`
	Entity        string `yaml:"entity,omitempty"`
	Region        string `yaml:"region,omitempty"`
	QueueURL      string `yaml:"queue_url,omitempty"`
	TableName     string `yaml:"table_name,omitempty"`

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

// SearchToolConfig represents a search tool entry in search_tools config.
type SearchToolConfig struct {
	SearchToolName string                 `yaml:"search_tool_name"`
	TianjiParams   SearchToolTianjiParams `yaml:"tianji_params"`
	SearchToolInfo *SearchToolInfo        `yaml:"search_tool_info,omitempty"`
}

// SearchToolTianjiParams holds provider-specific params for a search tool.
type SearchToolTianjiParams struct {
	SearchProvider string `yaml:"search_provider"` // brave, tavily, searxng, exa_ai, google_pse, dataforseo
	APIKey         string `yaml:"api_key,omitempty"`
	APIBase        string `yaml:"api_base,omitempty"`
}

// SearchToolInfo holds metadata about a search tool.
type SearchToolInfo struct {
	Description string `yaml:"description,omitempty"`
}

// MetricsConfig controls the Prometheus metrics exporter.
type MetricsConfig struct {
	Enabled       bool `yaml:"enabled"`
	RequireAuth   bool `yaml:"require_auth"`
	PerKeyMetrics bool `yaml:"per_key_metrics"`
}
