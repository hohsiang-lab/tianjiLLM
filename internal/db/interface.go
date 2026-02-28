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

	// Access groups
	CreateAccessGroup(ctx context.Context, arg CreateAccessGroupParams) (ModelAccessGroup, error)
	DeleteAccessGroup(ctx context.Context, groupID string) error
	GetAccessGroup(ctx context.Context, groupID string) (ModelAccessGroup, error)
	UpdateAccessGroup(ctx context.Context, arg UpdateAccessGroupParams) error

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
	UpdateCredential(ctx context.Context, arg UpdateCredentialParams) error

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

	// Guardrails
	CreateGuardrailConfig(ctx context.Context, arg CreateGuardrailConfigParams) (GuardrailConfigTable, error)
	DeleteGuardrailConfig(ctx context.Context, id string) error
	GetGuardrailConfig(ctx context.Context, id string) (GuardrailConfigTable, error)
	ListGuardrailConfigs(ctx context.Context) ([]GuardrailConfigTable, error)
	UpdateGuardrailConfig(ctx context.Context, arg UpdateGuardrailConfigParams) (GuardrailConfigTable, error)

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

	// Organizations
	AddOrgMember(ctx context.Context, arg AddOrgMemberParams) (OrganizationMembership, error)
	CreateOrganization(ctx context.Context, arg CreateOrganizationParams) (OrganizationTable, error)
	DeleteOrgMember(ctx context.Context, arg DeleteOrgMemberParams) error
	DeleteOrganization(ctx context.Context, organizationID string) error
	GetOrganization(ctx context.Context, organizationID string) (OrganizationTable, error)
	UpdateOrgMember(ctx context.Context, arg UpdateOrgMemberParams) (OrganizationMembership, error)
	UpdateOrganization(ctx context.Context, arg UpdateOrganizationParams) (OrganizationTable, error)

	// Plugins
	CreatePlugin(ctx context.Context, arg CreatePluginParams) (ClaudeCodePluginTable, error)
	DeletePlugin(ctx context.Context, name string) error
	DisablePlugin(ctx context.Context, name string) error
	EnablePlugin(ctx context.Context, name string) error
	GetPlugin(ctx context.Context, name string) (ClaudeCodePluginTable, error)
	ListEnabledPlugins(ctx context.Context) ([]ClaudeCodePluginTable, error)
	ListPlugins(ctx context.Context, arg ListPluginsParams) ([]ClaudeCodePluginTable, error)

	// Policies
	CreatePolicy(ctx context.Context, arg CreatePolicyParams) (PolicyTable, error)
	CreatePolicyAttachment(ctx context.Context, arg CreatePolicyAttachmentParams) (PolicyAttachmentTable, error)
	DeletePolicy(ctx context.Context, id string) error
	DeletePolicyAttachment(ctx context.Context, id string) error
	GetPolicy(ctx context.Context, id string) (PolicyTable, error)
	GetPolicyAttachment(ctx context.Context, id string) (PolicyAttachmentTable, error)
	GetPolicyByName(ctx context.Context, name string) (PolicyTable, error)
	ListPolicies(ctx context.Context) ([]PolicyTable, error)
	ListPolicyAttachments(ctx context.Context) ([]PolicyAttachmentTable, error)
	ListPolicyAttachmentsByPolicy(ctx context.Context, policyName string) ([]PolicyAttachmentTable, error)
	UpdatePolicy(ctx context.Context, arg UpdatePolicyParams) (PolicyTable, error)

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
	ResetTeamSpend(ctx context.Context, teamID string) error
	ResetVerificationTokenSpend(ctx context.Context, token string) error

	// Tags
	CreateTag(ctx context.Context, arg CreateTagParams) (TagTable, error)
	DeleteTag(ctx context.Context, id string) error
	GetTag(ctx context.Context, id string) (TagTable, error)
	ListTags(ctx context.Context) ([]TagTable, error)
	UpdateTag(ctx context.Context, arg UpdateTagParams) (TagTable, error)

	// Teams
	AddTeamMember(ctx context.Context, arg AddTeamMemberParams) error
	AddTeamModel(ctx context.Context, arg AddTeamModelParams) error
	BlockTeam(ctx context.Context, teamID string) error
	CreateTeam(ctx context.Context, arg CreateTeamParams) (TeamTable, error)
	DeleteTeam(ctx context.Context, teamID string) error
	GetTeam(ctx context.Context, teamID string) (TeamTable, error)
	GetTeamCallback(ctx context.Context, teamID string) (interface{}, error)
	GetTeamDailyActivity(ctx context.Context, arg GetTeamDailyActivityParams) ([]GetTeamDailyActivityRow, error)
	GetTeamPermissions(ctx context.Context, teamID string) ([]byte, error)
	ListAvailableTeams(ctx context.Context) ([]TeamTable, error)
	ListTeams(ctx context.Context) ([]TeamTable, error)
	RemoveTeamMember(ctx context.Context, arg RemoveTeamMemberParams) error
	RemoveTeamModel(ctx context.Context, arg RemoveTeamModelParams) error
	SetTeamCallback(ctx context.Context, arg SetTeamCallbackParams) error
	SetTeamPermissions(ctx context.Context, arg SetTeamPermissionsParams) error
	UnblockTeam(ctx context.Context, teamID string) error
	UpdateTeam(ctx context.Context, arg UpdateTeamParams) (TeamTable, error)
	UpdateTeamMemberRole(ctx context.Context, arg UpdateTeamMemberRoleParams) error

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
