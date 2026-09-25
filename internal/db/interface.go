package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store defines the database operations used by HTTP handlers.
// It is satisfied by *Queries (compile-time check below).
type Store interface {
	// Extension methods
	Pool() *pgxpool.Pool
	Ping(ctx context.Context) error

	// Agents
	CreateAgent(ctx context.Context, arg CreateAgentParams) (AgentsTable, error)
	DeleteAgent(ctx context.Context, agentID string) error
	GetAgent(ctx context.Context, agentID string) (AgentsTable, error)
	ListAgents(ctx context.Context, arg ListAgentsParams) ([]AgentsTable, error)
	PatchAgent(ctx context.Context, arg PatchAgentParams) (AgentsTable, error)
	UpdateAgent(ctx context.Context, arg UpdateAgentParams) (AgentsTable, error)

	// Audit
	GetAuditLog(ctx context.Context, id string) (AuditLog, error)
	InsertAuditLog(ctx context.Context, arg InsertAuditLogParams) (AuditLog, error)
	ListAuditLogs(ctx context.Context, arg ListAuditLogsParams) ([]AuditLog, error)

	// Budgets
	CreateBudget(ctx context.Context, arg CreateBudgetParams) (BudgetTable, error)
	DeleteBudget(ctx context.Context, budgetID string) error
	GetBudget(ctx context.Context, budgetID string) (BudgetTable, error)
	ListBudgets(ctx context.Context) ([]BudgetTable, error)
	UpdateBudget(ctx context.Context, arg UpdateBudgetParams) (BudgetTable, error)

	// Credentials
	CreateCredential(ctx context.Context, arg CreateCredentialParams) (CredentialTable, error)
	DeleteCredential(ctx context.Context, credentialID string) error
	GetCredential(ctx context.Context, credentialID string) (CredentialTable, error)
	ListCredentials(ctx context.Context) ([]CredentialTable, error)
	ListCredentialsByOrg(ctx context.Context, organizationID *string) ([]CredentialTable, error)
	ListOpenAISubscriptionRefreshCandidates(ctx context.Context) ([]CredentialTable, error)
	UpdateCredential(ctx context.Context, arg UpdateCredentialParams) error
	UpdateCredentialInfo(ctx context.Context, arg UpdateCredentialInfoParams) error
	UpdateCredentialValueAndInfo(ctx context.Context, arg UpdateCredentialValueAndInfoParams) error

	// End users
	BlockEndUser(ctx context.Context, id string) (EndUserTable2, error)
	CreateEndUser(ctx context.Context, arg CreateEndUserParams) (EndUserTable2, error)
	DeleteEndUser(ctx context.Context, id string) error
	GetEndUser(ctx context.Context, id string) (EndUserTable2, error)
	ListEndUsers(ctx context.Context) ([]EndUserTable2, error)
	UnblockEndUser(ctx context.Context, id string) (EndUserTable2, error)
	UpdateEndUser(ctx context.Context, arg UpdateEndUserParams) (EndUserTable2, error)

	// Error logs
	InsertErrorLog(ctx context.Context, arg InsertErrorLogParams) error
	ListErrorLogs(ctx context.Context, arg ListErrorLogsParams) ([]ErrorLog, error)

	// Health checks
	InsertHealthCheck(ctx context.Context, arg InsertHealthCheckParams) error
	ListHealthChecks(ctx context.Context, arg ListHealthChecksParams) ([]HealthCheckTable, error)

	// IP Whitelist
	CreateIPWhitelist(ctx context.Context, arg CreateIPWhitelistParams) (IPWhitelistTable, error)
	DeleteIPWhitelistByAddress(ctx context.Context, ipAddress string) error
	ListIPWhitelist(ctx context.Context) ([]IPWhitelistTable, error)

	// MCP
	CreateMCPServer(ctx context.Context, arg CreateMCPServerParams) (MCPServerTable, error)
	DeleteMCPServer(ctx context.Context, id string) error
	GetMCPServer(ctx context.Context, id string) (MCPServerTable, error)
	ListMCPServers(ctx context.Context) ([]MCPServerTable, error)
	UpdateMCPServer(ctx context.Context, arg UpdateMCPServerParams) (MCPServerTable, error)

	// OAuth Token Rate Limit State
	GetOAuthTokenRateLimitState(ctx context.Context, tokenKey string) (OAuthTokenRateLimitState, error)
	GetActiveOAuthTokenRateLimitStates(ctx context.Context) ([]OAuthTokenRateLimitState, error)
	UpsertOAuthTokenRateLimitState(ctx context.Context, arg UpsertOAuthTokenRateLimitStateParams) error

	// OAuth Token Metadata
	DisableOAuthToken(ctx context.Context, tokenKey string) error
	EnableOAuthToken(ctx context.Context, tokenKey string) error
	GetAllOAuthTokenMetadata(ctx context.Context) ([]GetAllOAuthTokenMetadataRow, error)
	GetOAuthTokenMetadata(ctx context.Context, tokenKey string) (GetOAuthTokenMetadataRow, error)
	UpsertOAuthTokenMetadata(ctx context.Context, arg UpsertOAuthTokenMetadataParams) error

	// Organizations
	GetOrganization(ctx context.Context, organizationID string) (OrganizationTable, error)

	// Plugins
	CreatePlugin(ctx context.Context, arg CreatePluginParams) (ClaudeCodePluginTable, error)
	DeletePlugin(ctx context.Context, name string) error
	DisablePlugin(ctx context.Context, name string) error
	EnablePlugin(ctx context.Context, name string) error
	GetPlugin(ctx context.Context, name string) (ClaudeCodePluginTable, error)
	ListEnabledPlugins(ctx context.Context) ([]ClaudeCodePluginTable, error)
	ListPlugins(ctx context.Context, arg ListPluginsParams) ([]ClaudeCodePluginTable, error)

	// Prompts
	CreatePromptTemplate(ctx context.Context, arg CreatePromptTemplateParams) (PromptTemplateTable, error)
	DeletePromptTemplate(ctx context.Context, id string) error
	GetLatestPromptByName(ctx context.Context, name string) (PromptTemplateTable, error)
	GetNextPromptVersion(ctx context.Context, name string) (int32, error)
	GetPromptTemplate(ctx context.Context, id string) (PromptTemplateTable, error)
	GetPromptTemplateByNameVersion(ctx context.Context, arg GetPromptTemplateByNameVersionParams) (PromptTemplateTable, error)
	GetPromptVersions(ctx context.Context, name string) ([]PromptTemplateTable, error)
	ListPromptTemplates(ctx context.Context) ([]PromptTemplateTable, error)

	// Proxy models
	CreateProxyModel(ctx context.Context, arg CreateProxyModelParams) (ProxyModelTable, error)
	DeleteProxyModel(ctx context.Context, modelID string) error
	GetProxyModel(ctx context.Context, modelID string) (ProxyModelTable, error)
	ListProxyModels(ctx context.Context) ([]ProxyModelTable, error)
	UpdateProxyModel(ctx context.Context, arg UpdateProxyModelParams) (ProxyModelTable, error)

	// Skills
	CreateSkill(ctx context.Context, arg CreateSkillParams) (SkillsTable, error)
	DeleteSkill(ctx context.Context, skillID string) error
	GetSkill(ctx context.Context, skillID string) (SkillsTable, error)
	ListSkills(ctx context.Context, arg ListSkillsParams) ([]SkillsTable, error)

	// Spend
	GetCacheHitStats(ctx context.Context, arg GetCacheHitStatsParams) ([]GetCacheHitStatsRow, error)
	GetDailySpendByKey(ctx context.Context, arg GetDailySpendByKeyParams) ([]GetDailySpendByKeyRow, error)
	GetDailySpendByModel(ctx context.Context, arg GetDailySpendByModelParams) ([]GetDailySpendByModelRow, error)
	GetDailySpendByTag(ctx context.Context, arg GetDailySpendByTagParams) ([]GetDailySpendByTagRow, error)
	GetDailySpendByTeam(ctx context.Context, arg GetDailySpendByTeamParams) ([]GetDailySpendByTeamRow, error)
	GetGlobalActivity(ctx context.Context, arg GetGlobalActivityParams) ([]GetGlobalActivityRow, error)
	GetGlobalActivityByModel(ctx context.Context, arg GetGlobalActivityByModelParams) ([]GetGlobalActivityByModelRow, error)
	GetGlobalSpend(ctx context.Context, arg GetGlobalSpendParams) (GetGlobalSpendRow, error)
	GetGlobalSpendByProvider(ctx context.Context, arg GetGlobalSpendByProviderParams) ([]GetGlobalSpendByProviderRow, error)
	GetGlobalSpendReport(ctx context.Context, arg GetGlobalSpendReportParams) ([]GetGlobalSpendReportRow, error)
	GetGlobalSpendReportByCustomer(ctx context.Context, arg GetGlobalSpendReportByCustomerParams) ([]GetGlobalSpendReportByCustomerRow, error)
	GetGlobalSpendReportByKey(ctx context.Context, arg GetGlobalSpendReportByKeyParams) ([]GetGlobalSpendReportByKeyRow, error)
	GetSpendByEndUser(ctx context.Context, starttime pgtype.Timestamptz) ([]GetSpendByEndUserRow, error)
	GetSpendByKey(ctx context.Context, arg GetSpendByKeyParams) ([]GetSpendByKeyRow, error)
	GetSpendByModel(ctx context.Context, starttime pgtype.Timestamptz) ([]GetSpendByModelRow, error)
	GetSpendByTag(ctx context.Context, starttime pgtype.Timestamptz) ([]GetSpendByTagRow, error)
	GetSpendByTeam(ctx context.Context, starttime pgtype.Timestamptz) ([]GetSpendByTeamRow, error)
	GetSpendByUser(ctx context.Context, arg GetSpendByUserParams) ([]GetSpendByUserRow, error)
	GetSpendLogsByFilter(ctx context.Context, arg GetSpendLogsByFilterParams) ([]GetSpendLogsByFilterRow, error)
	ResetAllKeySpend(ctx context.Context) error
	ResetAllTeamSpend(ctx context.Context) error
	ResetVerificationTokenSpend(ctx context.Context, token string) error

	// Tags
	CreateTag(ctx context.Context, arg CreateTagParams) (TagTable, error)
	DeleteTag(ctx context.Context, id string) error
	GetTag(ctx context.Context, id string) (TagTable, error)
	ListTags(ctx context.Context) ([]TagTable, error)
	UpdateTag(ctx context.Context, arg UpdateTagParams) (TagTable, error)

	// Teams
	GetTeam(ctx context.Context, teamID string) (TeamTable, error)
	ListTeams(ctx context.Context) ([]TeamTable, error)

	// Users
	CreateUser(ctx context.Context, arg CreateUserParams) (UserTable, error)
	DeleteUser(ctx context.Context, userID string) error
	GetUser(ctx context.Context, userID string) (UserTable, error)
	GetUserDailyActivity(ctx context.Context, arg GetUserDailyActivityParams) ([]GetUserDailyActivityRow, error)
	ListDistinctKeyAliases(ctx context.Context) ([]*string, error)
	ListUsers(ctx context.Context) ([]UserTable, error)
	UpdateUser(ctx context.Context, arg UpdateUserParams) (UserTable, error)

	// Verification tokens
	BlockVerificationToken(ctx context.Context, token string) error
	BulkUpdateVerificationTokens(ctx context.Context, arg BulkUpdateVerificationTokensParams) error
	CountVerificationTokensFiltered(ctx context.Context, arg CountVerificationTokensFilteredParams) (int64, error)
	CreateVerificationToken(ctx context.Context, arg CreateVerificationTokenParams) (VerificationToken, error)
	DeleteVerificationToken(ctx context.Context, token string) error
	GetVerificationToken(ctx context.Context, token string) (VerificationToken, error)
	GetVerificationTokenBatch(ctx context.Context, dollar_1 []string) ([]VerificationToken, error)
	ListVerificationTokensFiltered(ctx context.Context, arg ListVerificationTokensFilteredParams) ([]VerificationToken, error)
	RegenerateVerificationToken(ctx context.Context, arg RegenerateVerificationTokenParams) (VerificationToken, error)
	UnblockVerificationToken(ctx context.Context, token string) error
	UpdateVerificationToken(ctx context.Context, arg UpdateVerificationTokenParams) (VerificationToken, error)
}

// Compile-time check: *Queries implements Store.
var _ Store = (*Queries)(nil)
