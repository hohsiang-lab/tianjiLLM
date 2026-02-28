package handler

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

// mockStore is a test double for db.Store.
// Only methods needed by tests are implemented; the rest panic.
type mockStore struct {
	// Verification tokens (keys)
	createVerificationTokenFn          func(ctx context.Context, arg db.CreateVerificationTokenParams) (db.VerificationToken, error)
	getVerificationTokenFn             func(ctx context.Context, token string) (db.VerificationToken, error)
	listVerificationTokensFilteredFn   func(ctx context.Context, arg db.ListVerificationTokensFilteredParams) ([]db.VerificationToken, error)
	countVerificationTokensFilteredFn  func(ctx context.Context, arg db.CountVerificationTokensFilteredParams) (int64, error)
	deleteVerificationTokenFn          func(ctx context.Context, token string) error
	blockVerificationTokenFn           func(ctx context.Context, token string) error
	unblockVerificationTokenFn         func(ctx context.Context, token string) error
	updateVerificationTokenFn          func(ctx context.Context, arg db.UpdateVerificationTokenParams) (db.VerificationToken, error)

	// Teams
	createTeamFn      func(ctx context.Context, arg db.CreateTeamParams) (db.TeamTable, error)
	listTeamsFn       func(ctx context.Context) ([]db.TeamTable, error)
	deleteTeamFn      func(ctx context.Context, teamID string) error
	updateTeamFn      func(ctx context.Context, arg db.UpdateTeamParams) (db.TeamTable, error)
	addTeamMemberFn   func(ctx context.Context, arg db.AddTeamMemberParams) error
	removeTeamMemberFn func(ctx context.Context, arg db.RemoveTeamMemberParams) error

	// Users
	createUserFn func(ctx context.Context, arg db.CreateUserParams) (db.UserTable, error)
	listUsersFn  func(ctx context.Context) ([]db.UserTable, error)
	deleteUserFn func(ctx context.Context, userID string) error

	// Organizations
	createOrganizationFn func(ctx context.Context, arg db.CreateOrganizationParams) (db.OrganizationTable, error)
	getOrganizationFn    func(ctx context.Context, organizationID string) (db.OrganizationTable, error)
	updateOrganizationFn func(ctx context.Context, arg db.UpdateOrganizationParams) (db.OrganizationTable, error)
	deleteOrganizationFn func(ctx context.Context, organizationID string) error
	addOrgMemberFn       func(ctx context.Context, arg db.AddOrgMemberParams) (db.OrganizationMembership, error)
	updateOrgMemberFn    func(ctx context.Context, arg db.UpdateOrgMemberParams) (db.OrganizationMembership, error)
	deleteOrgMemberFn    func(ctx context.Context, arg db.DeleteOrgMemberParams) error

	// Spend
	getSpendByKeyFn     func(ctx context.Context, arg db.GetSpendByKeyParams) ([]db.GetSpendByKeyRow, error)
	getSpendByUserFn    func(ctx context.Context, arg db.GetSpendByUserParams) ([]db.GetSpendByUserRow, error)
	getSpendByTeamFn    func(ctx context.Context, starttime pgtype.Timestamptz) ([]db.GetSpendByTeamRow, error)
	getSpendByTagFn     func(ctx context.Context, starttime pgtype.Timestamptz) ([]db.GetSpendByTagRow, error)
	getSpendByModelFn   func(ctx context.Context, starttime pgtype.Timestamptz) ([]db.GetSpendByModelRow, error)
	getSpendByEndUserFn func(ctx context.Context, starttime pgtype.Timestamptz) ([]db.GetSpendByEndUserRow, error)

	// Audit (no-op by default)
	insertAuditLogFn func(ctx context.Context, arg db.InsertAuditLogParams) (db.AuditLog, error)
}

func newMockStore() *mockStore {
	return &mockStore{}
}

func (m *mockStore) ni() { panic("not implemented") }

// Implemented methods delegate to function fields
func (m *mockStore) CreateVerificationToken(ctx context.Context, arg db.CreateVerificationTokenParams) (db.VerificationToken, error) {
	if m.createVerificationTokenFn != nil { return m.createVerificationTokenFn(ctx, arg) }
	return db.VerificationToken{}, fmt.Errorf("not mocked")
}
func (m *mockStore) GetVerificationToken(ctx context.Context, token string) (db.VerificationToken, error) {
	if m.getVerificationTokenFn != nil { return m.getVerificationTokenFn(ctx, token) }
	return db.VerificationToken{}, fmt.Errorf("not mocked")
}
func (m *mockStore) ListVerificationTokensFiltered(ctx context.Context, arg db.ListVerificationTokensFilteredParams) ([]db.VerificationToken, error) {
	if m.listVerificationTokensFilteredFn != nil { return m.listVerificationTokensFilteredFn(ctx, arg) }
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) CountVerificationTokensFiltered(ctx context.Context, arg db.CountVerificationTokensFilteredParams) (int64, error) {
	if m.countVerificationTokensFilteredFn != nil { return m.countVerificationTokensFilteredFn(ctx, arg) }
	return 0, fmt.Errorf("not mocked")
}
func (m *mockStore) DeleteVerificationToken(ctx context.Context, token string) error {
	if m.deleteVerificationTokenFn != nil { return m.deleteVerificationTokenFn(ctx, token) }
	return fmt.Errorf("not mocked")
}
func (m *mockStore) BlockVerificationToken(ctx context.Context, token string) error {
	if m.blockVerificationTokenFn != nil { return m.blockVerificationTokenFn(ctx, token) }
	return fmt.Errorf("not mocked")
}
func (m *mockStore) UnblockVerificationToken(ctx context.Context, token string) error {
	if m.unblockVerificationTokenFn != nil { return m.unblockVerificationTokenFn(ctx, token) }
	return fmt.Errorf("not mocked")
}
func (m *mockStore) UpdateVerificationToken(ctx context.Context, arg db.UpdateVerificationTokenParams) (db.VerificationToken, error) {
	if m.updateVerificationTokenFn != nil { return m.updateVerificationTokenFn(ctx, arg) }
	return db.VerificationToken{}, fmt.Errorf("not mocked")
}
func (m *mockStore) CreateTeam(ctx context.Context, arg db.CreateTeamParams) (db.TeamTable, error) {
	if m.createTeamFn != nil { return m.createTeamFn(ctx, arg) }
	return db.TeamTable{}, fmt.Errorf("not mocked")
}
func (m *mockStore) ListTeams(ctx context.Context) ([]db.TeamTable, error) {
	if m.listTeamsFn != nil { return m.listTeamsFn(ctx) }
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) DeleteTeam(ctx context.Context, teamID string) error {
	if m.deleteTeamFn != nil { return m.deleteTeamFn(ctx, teamID) }
	return fmt.Errorf("not mocked")
}
func (m *mockStore) UpdateTeam(ctx context.Context, arg db.UpdateTeamParams) (db.TeamTable, error) {
	if m.updateTeamFn != nil { return m.updateTeamFn(ctx, arg) }
	return db.TeamTable{}, fmt.Errorf("not mocked")
}
func (m *mockStore) AddTeamMember(ctx context.Context, arg db.AddTeamMemberParams) error {
	if m.addTeamMemberFn != nil { return m.addTeamMemberFn(ctx, arg) }
	return fmt.Errorf("not mocked")
}
func (m *mockStore) RemoveTeamMember(ctx context.Context, arg db.RemoveTeamMemberParams) error {
	if m.removeTeamMemberFn != nil { return m.removeTeamMemberFn(ctx, arg) }
	return fmt.Errorf("not mocked")
}
func (m *mockStore) CreateUser(ctx context.Context, arg db.CreateUserParams) (db.UserTable, error) {
	if m.createUserFn != nil { return m.createUserFn(ctx, arg) }
	return db.UserTable{}, fmt.Errorf("not mocked")
}
func (m *mockStore) ListUsers(ctx context.Context) ([]db.UserTable, error) {
	if m.listUsersFn != nil { return m.listUsersFn(ctx) }
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) DeleteUser(ctx context.Context, userID string) error {
	if m.deleteUserFn != nil { return m.deleteUserFn(ctx, userID) }
	return fmt.Errorf("not mocked")
}
func (m *mockStore) CreateOrganization(ctx context.Context, arg db.CreateOrganizationParams) (db.OrganizationTable, error) {
	if m.createOrganizationFn != nil { return m.createOrganizationFn(ctx, arg) }
	return db.OrganizationTable{}, fmt.Errorf("not mocked")
}
func (m *mockStore) GetOrganization(ctx context.Context, organizationID string) (db.OrganizationTable, error) {
	if m.getOrganizationFn != nil { return m.getOrganizationFn(ctx, organizationID) }
	return db.OrganizationTable{}, fmt.Errorf("not mocked")
}
func (m *mockStore) UpdateOrganization(ctx context.Context, arg db.UpdateOrganizationParams) (db.OrganizationTable, error) {
	if m.updateOrganizationFn != nil { return m.updateOrganizationFn(ctx, arg) }
	return db.OrganizationTable{}, fmt.Errorf("not mocked")
}
func (m *mockStore) DeleteOrganization(ctx context.Context, organizationID string) error {
	if m.deleteOrganizationFn != nil { return m.deleteOrganizationFn(ctx, organizationID) }
	return fmt.Errorf("not mocked")
}
func (m *mockStore) AddOrgMember(ctx context.Context, arg db.AddOrgMemberParams) (db.OrganizationMembership, error) {
	if m.addOrgMemberFn != nil { return m.addOrgMemberFn(ctx, arg) }
	return db.OrganizationMembership{}, fmt.Errorf("not mocked")
}
func (m *mockStore) UpdateOrgMember(ctx context.Context, arg db.UpdateOrgMemberParams) (db.OrganizationMembership, error) {
	if m.updateOrgMemberFn != nil { return m.updateOrgMemberFn(ctx, arg) }
	return db.OrganizationMembership{}, fmt.Errorf("not mocked")
}
func (m *mockStore) DeleteOrgMember(ctx context.Context, arg db.DeleteOrgMemberParams) error {
	if m.deleteOrgMemberFn != nil { return m.deleteOrgMemberFn(ctx, arg) }
	return fmt.Errorf("not mocked")
}
func (m *mockStore) GetSpendByKey(ctx context.Context, arg db.GetSpendByKeyParams) ([]db.GetSpendByKeyRow, error) {
	if m.getSpendByKeyFn != nil { return m.getSpendByKeyFn(ctx, arg) }
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) GetSpendByUser(ctx context.Context, arg db.GetSpendByUserParams) ([]db.GetSpendByUserRow, error) {
	if m.getSpendByUserFn != nil { return m.getSpendByUserFn(ctx, arg) }
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) GetSpendByTeam(ctx context.Context, starttime pgtype.Timestamptz) ([]db.GetSpendByTeamRow, error) {
	if m.getSpendByTeamFn != nil { return m.getSpendByTeamFn(ctx, starttime) }
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) GetSpendByTag(ctx context.Context, starttime pgtype.Timestamptz) ([]db.GetSpendByTagRow, error) {
	if m.getSpendByTagFn != nil { return m.getSpendByTagFn(ctx, starttime) }
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) GetSpendByModel(ctx context.Context, starttime pgtype.Timestamptz) ([]db.GetSpendByModelRow, error) {
	if m.getSpendByModelFn != nil { return m.getSpendByModelFn(ctx, starttime) }
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) GetSpendByEndUser(ctx context.Context, starttime pgtype.Timestamptz) ([]db.GetSpendByEndUserRow, error) {
	if m.getSpendByEndUserFn != nil { return m.getSpendByEndUserFn(ctx, starttime) }
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) InsertAuditLog(ctx context.Context, arg db.InsertAuditLogParams) (db.AuditLog, error) {
	if m.insertAuditLogFn != nil { return m.insertAuditLogFn(ctx, arg) }
	return db.AuditLog{}, nil // no-op by default for audit
}

// All remaining Store interface methods panic with "not implemented".
func (m *mockStore) Pool() *pgxpool.Pool { return nil }
func (m *mockStore) Ping(ctx context.Context) error { return nil }
func (m *mockStore) AddTeamModel(ctx context.Context, arg db.AddTeamModelParams) error { m.ni(); return nil }
func (m *mockStore) BlockEndUser(ctx context.Context, id string) (db.EndUserTable2, error) { m.ni(); return db.EndUserTable2{}, nil }
func (m *mockStore) BlockTeam(ctx context.Context, teamID string) error { m.ni(); return nil }
func (m *mockStore) BulkUpdateVerificationTokens(ctx context.Context, arg db.BulkUpdateVerificationTokensParams) error { m.ni(); return nil }
func (m *mockStore) CreateAccessGroup(ctx context.Context, arg db.CreateAccessGroupParams) (db.ModelAccessGroup, error) { m.ni(); return db.ModelAccessGroup{}, nil }
func (m *mockStore) CreateAgent(ctx context.Context, arg db.CreateAgentParams) (db.AgentsTable, error) { m.ni(); return db.AgentsTable{}, nil }
func (m *mockStore) CreateBudget(ctx context.Context, arg db.CreateBudgetParams) (db.BudgetTable, error) { m.ni(); return db.BudgetTable{}, nil }
func (m *mockStore) CreateCredential(ctx context.Context, arg db.CreateCredentialParams) (db.CredentialTable, error) { m.ni(); return db.CredentialTable{}, nil }
func (m *mockStore) CreateEndUser(ctx context.Context, arg db.CreateEndUserParams) (db.EndUserTable2, error) { m.ni(); return db.EndUserTable2{}, nil }
func (m *mockStore) CreateGuardrailConfig(ctx context.Context, arg db.CreateGuardrailConfigParams) (db.GuardrailConfigTable, error) { m.ni(); return db.GuardrailConfigTable{}, nil }
func (m *mockStore) CreateIPWhitelist(ctx context.Context, arg db.CreateIPWhitelistParams) (db.IPWhitelistTable, error) { m.ni(); return db.IPWhitelistTable{}, nil }
func (m *mockStore) CreateMCPServer(ctx context.Context, arg db.CreateMCPServerParams) (db.MCPServerTable, error) { m.ni(); return db.MCPServerTable{}, nil }
func (m *mockStore) CreatePlugin(ctx context.Context, arg db.CreatePluginParams) (db.ClaudeCodePluginTable, error) { m.ni(); return db.ClaudeCodePluginTable{}, nil }
func (m *mockStore) CreatePolicy(ctx context.Context, arg db.CreatePolicyParams) (db.PolicyTable, error) { m.ni(); return db.PolicyTable{}, nil }
func (m *mockStore) CreatePolicyAttachment(ctx context.Context, arg db.CreatePolicyAttachmentParams) (db.PolicyAttachmentTable, error) { m.ni(); return db.PolicyAttachmentTable{}, nil }
func (m *mockStore) CreatePromptTemplate(ctx context.Context, arg db.CreatePromptTemplateParams) (db.PromptTemplateTable, error) { m.ni(); return db.PromptTemplateTable{}, nil }
func (m *mockStore) CreateProxyModel(ctx context.Context, arg db.CreateProxyModelParams) (db.ProxyModelTable, error) { m.ni(); return db.ProxyModelTable{}, nil }
func (m *mockStore) CreateSkill(ctx context.Context, arg db.CreateSkillParams) (db.SkillsTable, error) { m.ni(); return db.SkillsTable{}, nil }
func (m *mockStore) CreateTag(ctx context.Context, arg db.CreateTagParams) (db.TagTable, error) { m.ni(); return db.TagTable{}, nil }
func (m *mockStore) DeleteAccessGroup(ctx context.Context, groupID string) error { m.ni(); return nil }
func (m *mockStore) DeleteAgent(ctx context.Context, agentID string) error { m.ni(); return nil }
func (m *mockStore) DeleteBudget(ctx context.Context, budgetID string) error { m.ni(); return nil }
func (m *mockStore) DeleteCredential(ctx context.Context, credentialID string) error { m.ni(); return nil }
func (m *mockStore) DeleteEndUser(ctx context.Context, id string) error { m.ni(); return nil }
func (m *mockStore) DeleteGuardrailConfig(ctx context.Context, id string) error { m.ni(); return nil }
func (m *mockStore) DeleteIPWhitelistByAddress(ctx context.Context, ipAddress string) error { m.ni(); return nil }
func (m *mockStore) DeleteMCPServer(ctx context.Context, id string) error { m.ni(); return nil }
func (m *mockStore) DeletePlugin(ctx context.Context, name string) error { m.ni(); return nil }
func (m *mockStore) DeletePolicy(ctx context.Context, id string) error { m.ni(); return nil }
func (m *mockStore) DeletePolicyAttachment(ctx context.Context, id string) error { m.ni(); return nil }
func (m *mockStore) DeletePromptTemplate(ctx context.Context, id string) error { m.ni(); return nil }
func (m *mockStore) DeleteProxyModel(ctx context.Context, modelID string) error { m.ni(); return nil }
func (m *mockStore) DeleteSkill(ctx context.Context, skillID string) error { m.ni(); return nil }
func (m *mockStore) DeleteTag(ctx context.Context, id string) error { m.ni(); return nil }
func (m *mockStore) DisablePlugin(ctx context.Context, name string) error { m.ni(); return nil }
func (m *mockStore) EnablePlugin(ctx context.Context, name string) error { m.ni(); return nil }
func (m *mockStore) GetAccessGroup(ctx context.Context, groupID string) (db.ModelAccessGroup, error) { m.ni(); return db.ModelAccessGroup{}, nil }
func (m *mockStore) GetAgent(ctx context.Context, agentID string) (db.AgentsTable, error) { m.ni(); return db.AgentsTable{}, nil }
func (m *mockStore) GetAuditLog(ctx context.Context, id string) (db.AuditLog, error) { m.ni(); return db.AuditLog{}, nil }
func (m *mockStore) GetBudget(ctx context.Context, budgetID string) (db.BudgetTable, error) { m.ni(); return db.BudgetTable{}, nil }
func (m *mockStore) GetCacheHitStats(ctx context.Context, arg db.GetCacheHitStatsParams) ([]db.GetCacheHitStatsRow, error) { m.ni(); return nil, nil }
func (m *mockStore) GetCredential(ctx context.Context, credentialID string) (db.CredentialTable, error) { m.ni(); return db.CredentialTable{}, nil }
func (m *mockStore) GetDailySpendByKey(ctx context.Context, arg db.GetDailySpendByKeyParams) ([]db.GetDailySpendByKeyRow, error) { m.ni(); return nil, nil }
func (m *mockStore) GetDailySpendByModel(ctx context.Context, arg db.GetDailySpendByModelParams) ([]db.GetDailySpendByModelRow, error) { m.ni(); return nil, nil }
func (m *mockStore) GetDailySpendByTag(ctx context.Context, arg db.GetDailySpendByTagParams) ([]db.GetDailySpendByTagRow, error) { m.ni(); return nil, nil }
func (m *mockStore) GetDailySpendByTeam(ctx context.Context, arg db.GetDailySpendByTeamParams) ([]db.GetDailySpendByTeamRow, error) { m.ni(); return nil, nil }
func (m *mockStore) GetEndUser(ctx context.Context, id string) (db.EndUserTable2, error) { m.ni(); return db.EndUserTable2{}, nil }
func (m *mockStore) GetGlobalActivity(ctx context.Context, arg db.GetGlobalActivityParams) ([]db.GetGlobalActivityRow, error) { m.ni(); return nil, nil }
func (m *mockStore) GetGlobalActivityByModel(ctx context.Context, arg db.GetGlobalActivityByModelParams) ([]db.GetGlobalActivityByModelRow, error) { m.ni(); return nil, nil }
func (m *mockStore) GetGlobalSpend(ctx context.Context, arg db.GetGlobalSpendParams) (db.GetGlobalSpendRow, error) { m.ni(); return db.GetGlobalSpendRow{}, nil }
func (m *mockStore) GetGlobalSpendByProvider(ctx context.Context, arg db.GetGlobalSpendByProviderParams) ([]db.GetGlobalSpendByProviderRow, error) { m.ni(); return nil, nil }
func (m *mockStore) GetGlobalSpendReport(ctx context.Context, arg db.GetGlobalSpendReportParams) ([]db.GetGlobalSpendReportRow, error) { m.ni(); return nil, nil }
func (m *mockStore) GetGlobalSpendReportByCustomer(ctx context.Context, arg db.GetGlobalSpendReportByCustomerParams) ([]db.GetGlobalSpendReportByCustomerRow, error) { m.ni(); return nil, nil }
func (m *mockStore) GetGlobalSpendReportByKey(ctx context.Context, arg db.GetGlobalSpendReportByKeyParams) ([]db.GetGlobalSpendReportByKeyRow, error) { m.ni(); return nil, nil }
func (m *mockStore) GetGuardrailConfig(ctx context.Context, id string) (db.GuardrailConfigTable, error) { m.ni(); return db.GuardrailConfigTable{}, nil }
func (m *mockStore) GetMCPServer(ctx context.Context, id string) (db.MCPServerTable, error) { m.ni(); return db.MCPServerTable{}, nil }
func (m *mockStore) GetNextPromptVersion(ctx context.Context, name string) (int32, error) { m.ni(); return 0, nil }
func (m *mockStore) GetPlugin(ctx context.Context, name string) (db.ClaudeCodePluginTable, error) { m.ni(); return db.ClaudeCodePluginTable{}, nil }
func (m *mockStore) GetPolicy(ctx context.Context, id string) (db.PolicyTable, error) { m.ni(); return db.PolicyTable{}, nil }
func (m *mockStore) GetPolicyAttachment(ctx context.Context, id string) (db.PolicyAttachmentTable, error) { m.ni(); return db.PolicyAttachmentTable{}, nil }
func (m *mockStore) GetPolicyByName(ctx context.Context, name string) (db.PolicyTable, error) { m.ni(); return db.PolicyTable{}, nil }
func (m *mockStore) GetPromptTemplate(ctx context.Context, id string) (db.PromptTemplateTable, error) { m.ni(); return db.PromptTemplateTable{}, nil }
func (m *mockStore) GetPromptTemplateByNameVersion(ctx context.Context, arg db.GetPromptTemplateByNameVersionParams) (db.PromptTemplateTable, error) { m.ni(); return db.PromptTemplateTable{}, nil }
func (m *mockStore) GetPromptVersions(ctx context.Context, name string) ([]db.PromptTemplateTable, error) { m.ni(); return nil, nil }
func (m *mockStore) GetProxyModel(ctx context.Context, modelID string) (db.ProxyModelTable, error) { m.ni(); return db.ProxyModelTable{}, nil }
func (m *mockStore) GetSkill(ctx context.Context, skillID string) (db.SkillsTable, error) { m.ni(); return db.SkillsTable{}, nil }
func (m *mockStore) GetSpendLogsByFilter(ctx context.Context, arg db.GetSpendLogsByFilterParams) ([]db.GetSpendLogsByFilterRow, error) { m.ni(); return nil, nil }
func (m *mockStore) GetTag(ctx context.Context, id string) (db.TagTable, error) { m.ni(); return db.TagTable{}, nil }
func (m *mockStore) GetTeam(ctx context.Context, teamID string) (db.TeamTable, error) { m.ni(); return db.TeamTable{}, nil }
func (m *mockStore) GetTeamCallback(ctx context.Context, teamID string) (interface{}, error) { m.ni(); return nil, nil }
func (m *mockStore) GetTeamDailyActivity(ctx context.Context, arg db.GetTeamDailyActivityParams) ([]db.GetTeamDailyActivityRow, error) { m.ni(); return nil, nil }
func (m *mockStore) GetTeamPermissions(ctx context.Context, teamID string) ([]byte, error) { m.ni(); return nil, nil }
func (m *mockStore) GetUser(ctx context.Context, userID string) (db.UserTable, error) { m.ni(); return db.UserTable{}, nil }
func (m *mockStore) GetUserDailyActivity(ctx context.Context, arg db.GetUserDailyActivityParams) ([]db.GetUserDailyActivityRow, error) { m.ni(); return nil, nil }
func (m *mockStore) GetVerificationTokenBatch(ctx context.Context, dollar_1 []string) ([]db.VerificationToken, error) { m.ni(); return nil, nil }
func (m *mockStore) GetLatestPromptByName(ctx context.Context, name string) (db.PromptTemplateTable, error) { m.ni(); return db.PromptTemplateTable{}, nil }
func (m *mockStore) InsertErrorLog(ctx context.Context, arg db.InsertErrorLogParams) error { m.ni(); return nil }
func (m *mockStore) InsertHealthCheck(ctx context.Context, arg db.InsertHealthCheckParams) error { m.ni(); return nil }
func (m *mockStore) ListAgents(ctx context.Context, arg db.ListAgentsParams) ([]db.AgentsTable, error) { m.ni(); return nil, nil }
func (m *mockStore) ListAuditLogs(ctx context.Context, arg db.ListAuditLogsParams) ([]db.AuditLog, error) { m.ni(); return nil, nil }
func (m *mockStore) ListAvailableTeams(ctx context.Context) ([]db.TeamTable, error) { m.ni(); return nil, nil }
func (m *mockStore) ListBudgets(ctx context.Context) ([]db.BudgetTable, error) { m.ni(); return nil, nil }
func (m *mockStore) ListCredentials(ctx context.Context) ([]db.CredentialTable, error) { m.ni(); return nil, nil }
func (m *mockStore) ListCredentialsByOrg(ctx context.Context, organizationID *string) ([]db.CredentialTable, error) { m.ni(); return nil, nil }
func (m *mockStore) ListDistinctKeyAliases(ctx context.Context) ([]*string, error) { m.ni(); return nil, nil }
func (m *mockStore) ListEnabledPlugins(ctx context.Context) ([]db.ClaudeCodePluginTable, error) { m.ni(); return nil, nil }
func (m *mockStore) ListEndUsers(ctx context.Context) ([]db.EndUserTable2, error) { m.ni(); return nil, nil }
func (m *mockStore) ListErrorLogs(ctx context.Context, arg db.ListErrorLogsParams) ([]db.ErrorLog, error) { m.ni(); return nil, nil }
func (m *mockStore) ListGuardrailConfigs(ctx context.Context) ([]db.GuardrailConfigTable, error) { m.ni(); return nil, nil }
func (m *mockStore) ListHealthChecks(ctx context.Context, arg db.ListHealthChecksParams) ([]db.HealthCheckTable, error) { m.ni(); return nil, nil }
func (m *mockStore) ListIPWhitelist(ctx context.Context) ([]db.IPWhitelistTable, error) { m.ni(); return nil, nil }
func (m *mockStore) ListMCPServers(ctx context.Context) ([]db.MCPServerTable, error) { m.ni(); return nil, nil }
func (m *mockStore) ListPlugins(ctx context.Context, arg db.ListPluginsParams) ([]db.ClaudeCodePluginTable, error) { m.ni(); return nil, nil }
func (m *mockStore) ListPolicies(ctx context.Context) ([]db.PolicyTable, error) { m.ni(); return nil, nil }
func (m *mockStore) ListPolicyAttachments(ctx context.Context) ([]db.PolicyAttachmentTable, error) { m.ni(); return nil, nil }
func (m *mockStore) ListPolicyAttachmentsByPolicy(ctx context.Context, policyName string) ([]db.PolicyAttachmentTable, error) { m.ni(); return nil, nil }
func (m *mockStore) ListPromptTemplates(ctx context.Context) ([]db.PromptTemplateTable, error) { m.ni(); return nil, nil }
func (m *mockStore) ListProxyModels(ctx context.Context) ([]db.ProxyModelTable, error) { m.ni(); return nil, nil }
func (m *mockStore) ListSkills(ctx context.Context, arg db.ListSkillsParams) ([]db.SkillsTable, error) { m.ni(); return nil, nil }
func (m *mockStore) ListTags(ctx context.Context) ([]db.TagTable, error) { m.ni(); return nil, nil }
func (m *mockStore) PatchAgent(ctx context.Context, arg db.PatchAgentParams) (db.AgentsTable, error) { m.ni(); return db.AgentsTable{}, nil }
func (m *mockStore) RegenerateVerificationToken(ctx context.Context, arg db.RegenerateVerificationTokenParams) (db.VerificationToken, error) { m.ni(); return db.VerificationToken{}, nil }
func (m *mockStore) RemoveTeamModel(ctx context.Context, arg db.RemoveTeamModelParams) error { m.ni(); return nil }
func (m *mockStore) ResetAllKeySpend(ctx context.Context) error { m.ni(); return nil }
func (m *mockStore) ResetAllTeamSpend(ctx context.Context) error { m.ni(); return nil }
func (m *mockStore) ResetTeamSpend(ctx context.Context, teamID string) error { m.ni(); return nil }
func (m *mockStore) ResetVerificationTokenSpend(ctx context.Context, token string) error { m.ni(); return nil }
func (m *mockStore) SetTeamCallback(ctx context.Context, arg db.SetTeamCallbackParams) error { m.ni(); return nil }
func (m *mockStore) SetTeamPermissions(ctx context.Context, arg db.SetTeamPermissionsParams) error { m.ni(); return nil }
func (m *mockStore) UnblockEndUser(ctx context.Context, id string) (db.EndUserTable2, error) { m.ni(); return db.EndUserTable2{}, nil }
func (m *mockStore) UnblockTeam(ctx context.Context, teamID string) error { m.ni(); return nil }
func (m *mockStore) UpdateAccessGroup(ctx context.Context, arg db.UpdateAccessGroupParams) error { m.ni(); return nil }
func (m *mockStore) UpdateAgent(ctx context.Context, arg db.UpdateAgentParams) (db.AgentsTable, error) { m.ni(); return db.AgentsTable{}, nil }
func (m *mockStore) UpdateBudget(ctx context.Context, arg db.UpdateBudgetParams) (db.BudgetTable, error) { m.ni(); return db.BudgetTable{}, nil }
func (m *mockStore) UpdateCredential(ctx context.Context, arg db.UpdateCredentialParams) error { m.ni(); return nil }
func (m *mockStore) UpdateEndUser(ctx context.Context, arg db.UpdateEndUserParams) (db.EndUserTable2, error) { m.ni(); return db.EndUserTable2{}, nil }
func (m *mockStore) UpdateGuardrailConfig(ctx context.Context, arg db.UpdateGuardrailConfigParams) (db.GuardrailConfigTable, error) { m.ni(); return db.GuardrailConfigTable{}, nil }
func (m *mockStore) UpdateMCPServer(ctx context.Context, arg db.UpdateMCPServerParams) (db.MCPServerTable, error) { m.ni(); return db.MCPServerTable{}, nil }
func (m *mockStore) UpdatePolicy(ctx context.Context, arg db.UpdatePolicyParams) (db.PolicyTable, error) { m.ni(); return db.PolicyTable{}, nil }
func (m *mockStore) UpdateProxyModel(ctx context.Context, arg db.UpdateProxyModelParams) (db.ProxyModelTable, error) { m.ni(); return db.ProxyModelTable{}, nil }
func (m *mockStore) UpdateTag(ctx context.Context, arg db.UpdateTagParams) (db.TagTable, error) { m.ni(); return db.TagTable{}, nil }
func (m *mockStore) UpdateTeamMemberRole(ctx context.Context, arg db.UpdateTeamMemberRoleParams) error { m.ni(); return nil }
func (m *mockStore) UpdateUser(ctx context.Context, arg db.UpdateUserParams) (db.UserTable, error) { m.ni(); return db.UserTable{}, nil }

// Compile-time check
var _ db.Store = (*mockStore)(nil)
