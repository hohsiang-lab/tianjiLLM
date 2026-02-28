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
	createVerificationTokenFn         func(ctx context.Context, arg db.CreateVerificationTokenParams) (db.VerificationToken, error)
	getVerificationTokenFn            func(ctx context.Context, token string) (db.VerificationToken, error)
	listVerificationTokensFilteredFn  func(ctx context.Context, arg db.ListVerificationTokensFilteredParams) ([]db.VerificationToken, error)
	countVerificationTokensFilteredFn func(ctx context.Context, arg db.CountVerificationTokensFilteredParams) (int64, error)
	deleteVerificationTokenFn         func(ctx context.Context, token string) error
	blockVerificationTokenFn          func(ctx context.Context, token string) error
	unblockVerificationTokenFn        func(ctx context.Context, token string) error
	updateVerificationTokenFn         func(ctx context.Context, arg db.UpdateVerificationTokenParams) (db.VerificationToken, error)

	// Teams
	createTeamFn       func(ctx context.Context, arg db.CreateTeamParams) (db.TeamTable, error)
	listTeamsFn        func(ctx context.Context) ([]db.TeamTable, error)
	deleteTeamFn       func(ctx context.Context, teamID string) error
	updateTeamFn       func(ctx context.Context, arg db.UpdateTeamParams) (db.TeamTable, error)
	addTeamMemberFn    func(ctx context.Context, arg db.AddTeamMemberParams) error
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

	// Budget
	createBudgetFn func(ctx context.Context, arg db.CreateBudgetParams) (db.BudgetTable, error)
	getBudgetFn    func(ctx context.Context, budgetID string) (db.BudgetTable, error)
	listBudgetsFn  func(ctx context.Context) ([]db.BudgetTable, error)
	updateBudgetFn func(ctx context.Context, arg db.UpdateBudgetParams) (db.BudgetTable, error)
	deleteBudgetFn func(ctx context.Context, budgetID string) error

	// Tag
	createTagFn func(ctx context.Context, arg db.CreateTagParams) (db.TagTable, error)
	getTagFn    func(ctx context.Context, id string) (db.TagTable, error)
	listTagsFn  func(ctx context.Context) ([]db.TagTable, error)
	updateTagFn func(ctx context.Context, arg db.UpdateTagParams) (db.TagTable, error)
	deleteTagFn func(ctx context.Context, id string) error

	// Guardrail
	createGuardrailConfigFn func(ctx context.Context, arg db.CreateGuardrailConfigParams) (db.GuardrailConfigTable, error)
	getGuardrailConfigFn    func(ctx context.Context, id string) (db.GuardrailConfigTable, error)
	listGuardrailConfigsFn  func(ctx context.Context) ([]db.GuardrailConfigTable, error)
	updateGuardrailConfigFn func(ctx context.Context, arg db.UpdateGuardrailConfigParams) (db.GuardrailConfigTable, error)
	deleteGuardrailConfigFn func(ctx context.Context, id string) error

	// MCP
	createMCPServerFn func(ctx context.Context, arg db.CreateMCPServerParams) (db.MCPServerTable, error)
	getMCPServerFn    func(ctx context.Context, id string) (db.MCPServerTable, error)
	listMCPServersFn  func(ctx context.Context) ([]db.MCPServerTable, error)
	updateMCPServerFn func(ctx context.Context, arg db.UpdateMCPServerParams) (db.MCPServerTable, error)
	deleteMCPServerFn func(ctx context.Context, id string) error

	// Model
	createProxyModelFn func(ctx context.Context, arg db.CreateProxyModelParams) (db.ProxyModelTable, error)
	getProxyModelFn    func(ctx context.Context, modelID string) (db.ProxyModelTable, error)
	listProxyModelsFn  func(ctx context.Context) ([]db.ProxyModelTable, error)
	updateProxyModelFn func(ctx context.Context, arg db.UpdateProxyModelParams) (db.ProxyModelTable, error)
	deleteProxyModelFn func(ctx context.Context, modelID string) error

	// Policy
	createPolicyFn           func(ctx context.Context, arg db.CreatePolicyParams) (db.PolicyTable, error)
	getPolicyFn              func(ctx context.Context, id string) (db.PolicyTable, error)
	listPoliciesFn           func(ctx context.Context) ([]db.PolicyTable, error)
	updatePolicyFn           func(ctx context.Context, arg db.UpdatePolicyParams) (db.PolicyTable, error)
	deletePolicyFn           func(ctx context.Context, id string) error
	createPolicyAttachmentFn func(ctx context.Context, arg db.CreatePolicyAttachmentParams) (db.PolicyAttachmentTable, error)
	getPolicyAttachmentFn    func(ctx context.Context, id string) (db.PolicyAttachmentTable, error)
	listPolicyAttachmentsFn  func(ctx context.Context) ([]db.PolicyAttachmentTable, error)
	deletePolicyAttachmentFn func(ctx context.Context, id string) error

	// Credential
	createCredentialFn     func(ctx context.Context, arg db.CreateCredentialParams) (db.CredentialTable, error)
	getCredentialFn        func(ctx context.Context, credentialID string) (db.CredentialTable, error)
	listCredentialsFn      func(ctx context.Context) ([]db.CredentialTable, error)
	listCredentialsByOrgFn func(ctx context.Context, organizationID *string) ([]db.CredentialTable, error)
	updateCredentialFn     func(ctx context.Context, arg db.UpdateCredentialParams) error
	deleteCredentialFn     func(ctx context.Context, credentialID string) error

	// Spend global
	getGlobalSpendFn                 func(ctx context.Context, arg db.GetGlobalSpendParams) (db.GetGlobalSpendRow, error)
	getGlobalSpendReportFn           func(ctx context.Context, arg db.GetGlobalSpendReportParams) ([]db.GetGlobalSpendReportRow, error)
	getGlobalSpendReportByCustomerFn func(ctx context.Context, arg db.GetGlobalSpendReportByCustomerParams) ([]db.GetGlobalSpendReportByCustomerRow, error)
	getGlobalSpendReportByKeyFn      func(ctx context.Context, arg db.GetGlobalSpendReportByKeyParams) ([]db.GetGlobalSpendReportByKeyRow, error)
	getGlobalActivityFn              func(ctx context.Context, arg db.GetGlobalActivityParams) ([]db.GetGlobalActivityRow, error)
	getGlobalActivityByModelFn       func(ctx context.Context, arg db.GetGlobalActivityByModelParams) ([]db.GetGlobalActivityByModelRow, error)
	getGlobalSpendByProviderFn       func(ctx context.Context, arg db.GetGlobalSpendByProviderParams) ([]db.GetGlobalSpendByProviderRow, error)
	getCacheHitStatsFn               func(ctx context.Context, arg db.GetCacheHitStatsParams) ([]db.GetCacheHitStatsRow, error)
	getSpendLogsByFilterFn           func(ctx context.Context, arg db.GetSpendLogsByFilterParams) ([]db.GetSpendLogsByFilterRow, error)
	getDailySpendByKeyFn             func(ctx context.Context, arg db.GetDailySpendByKeyParams) ([]db.GetDailySpendByKeyRow, error)
	getDailySpendByModelFn           func(ctx context.Context, arg db.GetDailySpendByModelParams) ([]db.GetDailySpendByModelRow, error)
	getDailySpendByTeamFn            func(ctx context.Context, arg db.GetDailySpendByTeamParams) ([]db.GetDailySpendByTeamRow, error)
	getDailySpendByTagFn             func(ctx context.Context, arg db.GetDailySpendByTagParams) ([]db.GetDailySpendByTagRow, error)
	resetAllKeySpendFn               func(ctx context.Context) error
	resetAllTeamSpendFn              func(ctx context.Context) error
}

func newMockStore() *mockStore {
	return &mockStore{}
}

func (m *mockStore) ni() { panic("not implemented") }

// Implemented methods delegate to function fields
func (m *mockStore) CreateVerificationToken(ctx context.Context, arg db.CreateVerificationTokenParams) (db.VerificationToken, error) {
	if m.createVerificationTokenFn != nil {
		return m.createVerificationTokenFn(ctx, arg)
	}
	return db.VerificationToken{}, fmt.Errorf("not mocked")
}
func (m *mockStore) GetVerificationToken(ctx context.Context, token string) (db.VerificationToken, error) {
	if m.getVerificationTokenFn != nil {
		return m.getVerificationTokenFn(ctx, token)
	}
	return db.VerificationToken{}, fmt.Errorf("not mocked")
}
func (m *mockStore) ListVerificationTokensFiltered(ctx context.Context, arg db.ListVerificationTokensFilteredParams) ([]db.VerificationToken, error) {
	if m.listVerificationTokensFilteredFn != nil {
		return m.listVerificationTokensFilteredFn(ctx, arg)
	}
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) CountVerificationTokensFiltered(ctx context.Context, arg db.CountVerificationTokensFilteredParams) (int64, error) {
	if m.countVerificationTokensFilteredFn != nil {
		return m.countVerificationTokensFilteredFn(ctx, arg)
	}
	return 0, fmt.Errorf("not mocked")
}
func (m *mockStore) DeleteVerificationToken(ctx context.Context, token string) error {
	if m.deleteVerificationTokenFn != nil {
		return m.deleteVerificationTokenFn(ctx, token)
	}
	return fmt.Errorf("not mocked")
}
func (m *mockStore) BlockVerificationToken(ctx context.Context, token string) error {
	if m.blockVerificationTokenFn != nil {
		return m.blockVerificationTokenFn(ctx, token)
	}
	return fmt.Errorf("not mocked")
}
func (m *mockStore) UnblockVerificationToken(ctx context.Context, token string) error {
	if m.unblockVerificationTokenFn != nil {
		return m.unblockVerificationTokenFn(ctx, token)
	}
	return fmt.Errorf("not mocked")
}
func (m *mockStore) UpdateVerificationToken(ctx context.Context, arg db.UpdateVerificationTokenParams) (db.VerificationToken, error) {
	if m.updateVerificationTokenFn != nil {
		return m.updateVerificationTokenFn(ctx, arg)
	}
	return db.VerificationToken{}, fmt.Errorf("not mocked")
}
func (m *mockStore) CreateTeam(ctx context.Context, arg db.CreateTeamParams) (db.TeamTable, error) {
	if m.createTeamFn != nil {
		return m.createTeamFn(ctx, arg)
	}
	return db.TeamTable{}, fmt.Errorf("not mocked")
}
func (m *mockStore) ListTeams(ctx context.Context) ([]db.TeamTable, error) {
	if m.listTeamsFn != nil {
		return m.listTeamsFn(ctx)
	}
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) DeleteTeam(ctx context.Context, teamID string) error {
	if m.deleteTeamFn != nil {
		return m.deleteTeamFn(ctx, teamID)
	}
	return fmt.Errorf("not mocked")
}
func (m *mockStore) UpdateTeam(ctx context.Context, arg db.UpdateTeamParams) (db.TeamTable, error) {
	if m.updateTeamFn != nil {
		return m.updateTeamFn(ctx, arg)
	}
	return db.TeamTable{}, fmt.Errorf("not mocked")
}
func (m *mockStore) AddTeamMember(ctx context.Context, arg db.AddTeamMemberParams) error {
	if m.addTeamMemberFn != nil {
		return m.addTeamMemberFn(ctx, arg)
	}
	return fmt.Errorf("not mocked")
}
func (m *mockStore) RemoveTeamMember(ctx context.Context, arg db.RemoveTeamMemberParams) error {
	if m.removeTeamMemberFn != nil {
		return m.removeTeamMemberFn(ctx, arg)
	}
	return fmt.Errorf("not mocked")
}
func (m *mockStore) CreateUser(ctx context.Context, arg db.CreateUserParams) (db.UserTable, error) {
	if m.createUserFn != nil {
		return m.createUserFn(ctx, arg)
	}
	return db.UserTable{}, fmt.Errorf("not mocked")
}
func (m *mockStore) ListUsers(ctx context.Context) ([]db.UserTable, error) {
	if m.listUsersFn != nil {
		return m.listUsersFn(ctx)
	}
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) DeleteUser(ctx context.Context, userID string) error {
	if m.deleteUserFn != nil {
		return m.deleteUserFn(ctx, userID)
	}
	return fmt.Errorf("not mocked")
}
func (m *mockStore) CreateOrganization(ctx context.Context, arg db.CreateOrganizationParams) (db.OrganizationTable, error) {
	if m.createOrganizationFn != nil {
		return m.createOrganizationFn(ctx, arg)
	}
	return db.OrganizationTable{}, fmt.Errorf("not mocked")
}
func (m *mockStore) GetOrganization(ctx context.Context, organizationID string) (db.OrganizationTable, error) {
	if m.getOrganizationFn != nil {
		return m.getOrganizationFn(ctx, organizationID)
	}
	return db.OrganizationTable{}, fmt.Errorf("not mocked")
}
func (m *mockStore) UpdateOrganization(ctx context.Context, arg db.UpdateOrganizationParams) (db.OrganizationTable, error) {
	if m.updateOrganizationFn != nil {
		return m.updateOrganizationFn(ctx, arg)
	}
	return db.OrganizationTable{}, fmt.Errorf("not mocked")
}
func (m *mockStore) DeleteOrganization(ctx context.Context, organizationID string) error {
	if m.deleteOrganizationFn != nil {
		return m.deleteOrganizationFn(ctx, organizationID)
	}
	return fmt.Errorf("not mocked")
}
func (m *mockStore) AddOrgMember(ctx context.Context, arg db.AddOrgMemberParams) (db.OrganizationMembership, error) {
	if m.addOrgMemberFn != nil {
		return m.addOrgMemberFn(ctx, arg)
	}
	return db.OrganizationMembership{}, fmt.Errorf("not mocked")
}
func (m *mockStore) UpdateOrgMember(ctx context.Context, arg db.UpdateOrgMemberParams) (db.OrganizationMembership, error) {
	if m.updateOrgMemberFn != nil {
		return m.updateOrgMemberFn(ctx, arg)
	}
	return db.OrganizationMembership{}, fmt.Errorf("not mocked")
}
func (m *mockStore) DeleteOrgMember(ctx context.Context, arg db.DeleteOrgMemberParams) error {
	if m.deleteOrgMemberFn != nil {
		return m.deleteOrgMemberFn(ctx, arg)
	}
	return fmt.Errorf("not mocked")
}
func (m *mockStore) GetSpendByKey(ctx context.Context, arg db.GetSpendByKeyParams) ([]db.GetSpendByKeyRow, error) {
	if m.getSpendByKeyFn != nil {
		return m.getSpendByKeyFn(ctx, arg)
	}
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) GetSpendByUser(ctx context.Context, arg db.GetSpendByUserParams) ([]db.GetSpendByUserRow, error) {
	if m.getSpendByUserFn != nil {
		return m.getSpendByUserFn(ctx, arg)
	}
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) GetSpendByTeam(ctx context.Context, starttime pgtype.Timestamptz) ([]db.GetSpendByTeamRow, error) {
	if m.getSpendByTeamFn != nil {
		return m.getSpendByTeamFn(ctx, starttime)
	}
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) GetSpendByTag(ctx context.Context, starttime pgtype.Timestamptz) ([]db.GetSpendByTagRow, error) {
	if m.getSpendByTagFn != nil {
		return m.getSpendByTagFn(ctx, starttime)
	}
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) GetSpendByModel(ctx context.Context, starttime pgtype.Timestamptz) ([]db.GetSpendByModelRow, error) {
	if m.getSpendByModelFn != nil {
		return m.getSpendByModelFn(ctx, starttime)
	}
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) GetSpendByEndUser(ctx context.Context, starttime pgtype.Timestamptz) ([]db.GetSpendByEndUserRow, error) {
	if m.getSpendByEndUserFn != nil {
		return m.getSpendByEndUserFn(ctx, starttime)
	}
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) InsertAuditLog(ctx context.Context, arg db.InsertAuditLogParams) (db.AuditLog, error) {
	if m.insertAuditLogFn != nil {
		return m.insertAuditLogFn(ctx, arg)
	}
	return db.AuditLog{}, nil // no-op by default for audit
}

// All remaining Store interface methods panic with "not implemented".
func (m *mockStore) Pool() *pgxpool.Pool            { return nil }
func (m *mockStore) Ping(ctx context.Context) error { return nil }
func (m *mockStore) AddTeamModel(ctx context.Context, arg db.AddTeamModelParams) error {
	m.ni()
	return nil
}
func (m *mockStore) BlockEndUser(ctx context.Context, id string) (db.EndUserTable2, error) {
	m.ni()
	return db.EndUserTable2{}, nil
}
func (m *mockStore) BlockTeam(ctx context.Context, teamID string) error { m.ni(); return nil }
func (m *mockStore) BulkUpdateVerificationTokens(ctx context.Context, arg db.BulkUpdateVerificationTokensParams) error {
	m.ni()
	return nil
}
func (m *mockStore) CreateAccessGroup(ctx context.Context, arg db.CreateAccessGroupParams) (db.ModelAccessGroup, error) {
	m.ni()
	return db.ModelAccessGroup{}, nil
}
func (m *mockStore) CreateAgent(ctx context.Context, arg db.CreateAgentParams) (db.AgentsTable, error) {
	m.ni()
	return db.AgentsTable{}, nil
}
func (m *mockStore) CreateBudget(ctx context.Context, arg db.CreateBudgetParams) (db.BudgetTable, error) {
	if m.createBudgetFn != nil {
		return m.createBudgetFn(ctx, arg)
	}
	return db.BudgetTable{}, fmt.Errorf("not mocked")
}
func (m *mockStore) CreateCredential(ctx context.Context, arg db.CreateCredentialParams) (db.CredentialTable, error) {
	if m.createCredentialFn != nil {
		return m.createCredentialFn(ctx, arg)
	}
	return db.CredentialTable{}, fmt.Errorf("not mocked")
}
func (m *mockStore) CreateEndUser(ctx context.Context, arg db.CreateEndUserParams) (db.EndUserTable2, error) {
	m.ni()
	return db.EndUserTable2{}, nil
}
func (m *mockStore) CreateGuardrailConfig(ctx context.Context, arg db.CreateGuardrailConfigParams) (db.GuardrailConfigTable, error) {
	if m.createGuardrailConfigFn != nil {
		return m.createGuardrailConfigFn(ctx, arg)
	}
	return db.GuardrailConfigTable{}, fmt.Errorf("not mocked")
}
func (m *mockStore) CreateIPWhitelist(ctx context.Context, arg db.CreateIPWhitelistParams) (db.IPWhitelistTable, error) {
	m.ni()
	return db.IPWhitelistTable{}, nil
}
func (m *mockStore) CreateMCPServer(ctx context.Context, arg db.CreateMCPServerParams) (db.MCPServerTable, error) {
	if m.createMCPServerFn != nil {
		return m.createMCPServerFn(ctx, arg)
	}
	return db.MCPServerTable{}, fmt.Errorf("not mocked")
}
func (m *mockStore) CreatePlugin(ctx context.Context, arg db.CreatePluginParams) (db.ClaudeCodePluginTable, error) {
	m.ni()
	return db.ClaudeCodePluginTable{}, nil
}
func (m *mockStore) CreatePolicy(ctx context.Context, arg db.CreatePolicyParams) (db.PolicyTable, error) {
	if m.createPolicyFn != nil {
		return m.createPolicyFn(ctx, arg)
	}
	return db.PolicyTable{}, fmt.Errorf("not mocked")
}
func (m *mockStore) CreatePolicyAttachment(ctx context.Context, arg db.CreatePolicyAttachmentParams) (db.PolicyAttachmentTable, error) {
	if m.createPolicyAttachmentFn != nil {
		return m.createPolicyAttachmentFn(ctx, arg)
	}
	return db.PolicyAttachmentTable{}, fmt.Errorf("not mocked")
}
func (m *mockStore) CreatePromptTemplate(ctx context.Context, arg db.CreatePromptTemplateParams) (db.PromptTemplateTable, error) {
	m.ni()
	return db.PromptTemplateTable{}, nil
}
func (m *mockStore) CreateProxyModel(ctx context.Context, arg db.CreateProxyModelParams) (db.ProxyModelTable, error) {
	if m.createProxyModelFn != nil {
		return m.createProxyModelFn(ctx, arg)
	}
	return db.ProxyModelTable{}, fmt.Errorf("not mocked")
}
func (m *mockStore) CreateSkill(ctx context.Context, arg db.CreateSkillParams) (db.SkillsTable, error) {
	m.ni()
	return db.SkillsTable{}, nil
}
func (m *mockStore) CreateTag(ctx context.Context, arg db.CreateTagParams) (db.TagTable, error) {
	if m.createTagFn != nil {
		return m.createTagFn(ctx, arg)
	}
	return db.TagTable{}, fmt.Errorf("not mocked")
}
func (m *mockStore) DeleteAccessGroup(ctx context.Context, groupID string) error { m.ni(); return nil }
func (m *mockStore) DeleteAgent(ctx context.Context, agentID string) error       { m.ni(); return nil }
func (m *mockStore) DeleteBudget(ctx context.Context, budgetID string) error {
	if m.deleteBudgetFn != nil {
		return m.deleteBudgetFn(ctx, budgetID)
	}
	return fmt.Errorf("not mocked")
}
func (m *mockStore) DeleteCredential(ctx context.Context, credentialID string) error {
	if m.deleteCredentialFn != nil {
		return m.deleteCredentialFn(ctx, credentialID)
	}
	return fmt.Errorf("not mocked")
}
func (m *mockStore) DeleteEndUser(ctx context.Context, id string) error { m.ni(); return nil }
func (m *mockStore) DeleteGuardrailConfig(ctx context.Context, id string) error {
	if m.deleteGuardrailConfigFn != nil {
		return m.deleteGuardrailConfigFn(ctx, id)
	}
	return fmt.Errorf("not mocked")
}
func (m *mockStore) DeleteIPWhitelistByAddress(ctx context.Context, ipAddress string) error {
	m.ni()
	return nil
}
func (m *mockStore) DeleteMCPServer(ctx context.Context, id string) error {
	if m.deleteMCPServerFn != nil {
		return m.deleteMCPServerFn(ctx, id)
	}
	return fmt.Errorf("not mocked")
}
func (m *mockStore) DeletePlugin(ctx context.Context, name string) error { m.ni(); return nil }
func (m *mockStore) DeletePolicy(ctx context.Context, id string) error {
	if m.deletePolicyFn != nil {
		return m.deletePolicyFn(ctx, id)
	}
	return fmt.Errorf("not mocked")
}
func (m *mockStore) DeletePolicyAttachment(ctx context.Context, id string) error {
	if m.deletePolicyAttachmentFn != nil {
		return m.deletePolicyAttachmentFn(ctx, id)
	}
	return fmt.Errorf("not mocked")
}
func (m *mockStore) DeletePromptTemplate(ctx context.Context, id string) error { m.ni(); return nil }
func (m *mockStore) DeleteProxyModel(ctx context.Context, modelID string) error {
	if m.deleteProxyModelFn != nil {
		return m.deleteProxyModelFn(ctx, modelID)
	}
	return fmt.Errorf("not mocked")
}
func (m *mockStore) DeleteSkill(ctx context.Context, skillID string) error { m.ni(); return nil }
func (m *mockStore) DeleteTag(ctx context.Context, id string) error {
	if m.deleteTagFn != nil {
		return m.deleteTagFn(ctx, id)
	}
	return fmt.Errorf("not mocked")
}
func (m *mockStore) DisablePlugin(ctx context.Context, name string) error { m.ni(); return nil }
func (m *mockStore) EnablePlugin(ctx context.Context, name string) error  { m.ni(); return nil }
func (m *mockStore) GetAccessGroup(ctx context.Context, groupID string) (db.ModelAccessGroup, error) {
	m.ni()
	return db.ModelAccessGroup{}, nil
}
func (m *mockStore) GetAgent(ctx context.Context, agentID string) (db.AgentsTable, error) {
	m.ni()
	return db.AgentsTable{}, nil
}
func (m *mockStore) GetAuditLog(ctx context.Context, id string) (db.AuditLog, error) {
	m.ni()
	return db.AuditLog{}, nil
}
func (m *mockStore) GetBudget(ctx context.Context, budgetID string) (db.BudgetTable, error) {
	if m.getBudgetFn != nil {
		return m.getBudgetFn(ctx, budgetID)
	}
	return db.BudgetTable{}, fmt.Errorf("not mocked")
}
func (m *mockStore) GetCacheHitStats(ctx context.Context, arg db.GetCacheHitStatsParams) ([]db.GetCacheHitStatsRow, error) {
	if m.getCacheHitStatsFn != nil {
		return m.getCacheHitStatsFn(ctx, arg)
	}
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) GetCredential(ctx context.Context, credentialID string) (db.CredentialTable, error) {
	if m.getCredentialFn != nil {
		return m.getCredentialFn(ctx, credentialID)
	}
	return db.CredentialTable{}, fmt.Errorf("not mocked")
}
func (m *mockStore) GetDailySpendByKey(ctx context.Context, arg db.GetDailySpendByKeyParams) ([]db.GetDailySpendByKeyRow, error) {
	if m.getDailySpendByKeyFn != nil {
		return m.getDailySpendByKeyFn(ctx, arg)
	}
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) GetDailySpendByModel(ctx context.Context, arg db.GetDailySpendByModelParams) ([]db.GetDailySpendByModelRow, error) {
	if m.getDailySpendByModelFn != nil {
		return m.getDailySpendByModelFn(ctx, arg)
	}
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) GetDailySpendByTag(ctx context.Context, arg db.GetDailySpendByTagParams) ([]db.GetDailySpendByTagRow, error) {
	if m.getDailySpendByTagFn != nil {
		return m.getDailySpendByTagFn(ctx, arg)
	}
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) GetDailySpendByTeam(ctx context.Context, arg db.GetDailySpendByTeamParams) ([]db.GetDailySpendByTeamRow, error) {
	if m.getDailySpendByTeamFn != nil {
		return m.getDailySpendByTeamFn(ctx, arg)
	}
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) GetEndUser(ctx context.Context, id string) (db.EndUserTable2, error) {
	m.ni()
	return db.EndUserTable2{}, nil
}
func (m *mockStore) GetGlobalActivity(ctx context.Context, arg db.GetGlobalActivityParams) ([]db.GetGlobalActivityRow, error) {
	if m.getGlobalActivityFn != nil {
		return m.getGlobalActivityFn(ctx, arg)
	}
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) GetGlobalActivityByModel(ctx context.Context, arg db.GetGlobalActivityByModelParams) ([]db.GetGlobalActivityByModelRow, error) {
	if m.getGlobalActivityByModelFn != nil {
		return m.getGlobalActivityByModelFn(ctx, arg)
	}
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) GetGlobalSpend(ctx context.Context, arg db.GetGlobalSpendParams) (db.GetGlobalSpendRow, error) {
	if m.getGlobalSpendFn != nil {
		return m.getGlobalSpendFn(ctx, arg)
	}
	return db.GetGlobalSpendRow{}, fmt.Errorf("not mocked")
}
func (m *mockStore) GetGlobalSpendByProvider(ctx context.Context, arg db.GetGlobalSpendByProviderParams) ([]db.GetGlobalSpendByProviderRow, error) {
	if m.getGlobalSpendByProviderFn != nil {
		return m.getGlobalSpendByProviderFn(ctx, arg)
	}
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) GetGlobalSpendReport(ctx context.Context, arg db.GetGlobalSpendReportParams) ([]db.GetGlobalSpendReportRow, error) {
	if m.getGlobalSpendReportFn != nil {
		return m.getGlobalSpendReportFn(ctx, arg)
	}
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) GetGlobalSpendReportByCustomer(ctx context.Context, arg db.GetGlobalSpendReportByCustomerParams) ([]db.GetGlobalSpendReportByCustomerRow, error) {
	if m.getGlobalSpendReportByCustomerFn != nil {
		return m.getGlobalSpendReportByCustomerFn(ctx, arg)
	}
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) GetGlobalSpendReportByKey(ctx context.Context, arg db.GetGlobalSpendReportByKeyParams) ([]db.GetGlobalSpendReportByKeyRow, error) {
	if m.getGlobalSpendReportByKeyFn != nil {
		return m.getGlobalSpendReportByKeyFn(ctx, arg)
	}
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) GetGuardrailConfig(ctx context.Context, id string) (db.GuardrailConfigTable, error) {
	if m.getGuardrailConfigFn != nil {
		return m.getGuardrailConfigFn(ctx, id)
	}
	return db.GuardrailConfigTable{}, fmt.Errorf("not mocked")
}
func (m *mockStore) GetMCPServer(ctx context.Context, id string) (db.MCPServerTable, error) {
	if m.getMCPServerFn != nil {
		return m.getMCPServerFn(ctx, id)
	}
	return db.MCPServerTable{}, fmt.Errorf("not mocked")
}
func (m *mockStore) GetNextPromptVersion(ctx context.Context, name string) (int32, error) {
	m.ni()
	return 0, nil
}
func (m *mockStore) GetPlugin(ctx context.Context, name string) (db.ClaudeCodePluginTable, error) {
	m.ni()
	return db.ClaudeCodePluginTable{}, nil
}
func (m *mockStore) GetPolicy(ctx context.Context, id string) (db.PolicyTable, error) {
	if m.getPolicyFn != nil {
		return m.getPolicyFn(ctx, id)
	}
	return db.PolicyTable{}, fmt.Errorf("not mocked")
}
func (m *mockStore) GetPolicyAttachment(ctx context.Context, id string) (db.PolicyAttachmentTable, error) {
	if m.getPolicyAttachmentFn != nil {
		return m.getPolicyAttachmentFn(ctx, id)
	}
	return db.PolicyAttachmentTable{}, fmt.Errorf("not mocked")
}
func (m *mockStore) GetPolicyByName(ctx context.Context, name string) (db.PolicyTable, error) {
	m.ni()
	return db.PolicyTable{}, nil
}
func (m *mockStore) GetPromptTemplate(ctx context.Context, id string) (db.PromptTemplateTable, error) {
	m.ni()
	return db.PromptTemplateTable{}, nil
}
func (m *mockStore) GetPromptTemplateByNameVersion(ctx context.Context, arg db.GetPromptTemplateByNameVersionParams) (db.PromptTemplateTable, error) {
	m.ni()
	return db.PromptTemplateTable{}, nil
}
func (m *mockStore) GetPromptVersions(ctx context.Context, name string) ([]db.PromptTemplateTable, error) {
	m.ni()
	return nil, nil
}
func (m *mockStore) GetProxyModel(ctx context.Context, modelID string) (db.ProxyModelTable, error) {
	if m.getProxyModelFn != nil {
		return m.getProxyModelFn(ctx, modelID)
	}
	return db.ProxyModelTable{}, fmt.Errorf("not mocked")
}
func (m *mockStore) GetSkill(ctx context.Context, skillID string) (db.SkillsTable, error) {
	m.ni()
	return db.SkillsTable{}, nil
}
func (m *mockStore) GetSpendLogsByFilter(ctx context.Context, arg db.GetSpendLogsByFilterParams) ([]db.GetSpendLogsByFilterRow, error) {
	if m.getSpendLogsByFilterFn != nil {
		return m.getSpendLogsByFilterFn(ctx, arg)
	}
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) GetTag(ctx context.Context, id string) (db.TagTable, error) {
	if m.getTagFn != nil {
		return m.getTagFn(ctx, id)
	}
	return db.TagTable{}, fmt.Errorf("not mocked")
}
func (m *mockStore) GetTeam(ctx context.Context, teamID string) (db.TeamTable, error) {
	m.ni()
	return db.TeamTable{}, nil
}
func (m *mockStore) GetTeamCallback(ctx context.Context, teamID string) (interface{}, error) {
	m.ni()
	return nil, nil
}
func (m *mockStore) GetTeamDailyActivity(ctx context.Context, arg db.GetTeamDailyActivityParams) ([]db.GetTeamDailyActivityRow, error) {
	m.ni()
	return nil, nil
}
func (m *mockStore) GetTeamPermissions(ctx context.Context, teamID string) ([]byte, error) {
	m.ni()
	return nil, nil
}
func (m *mockStore) GetUser(ctx context.Context, userID string) (db.UserTable, error) {
	m.ni()
	return db.UserTable{}, nil
}
func (m *mockStore) GetUserDailyActivity(ctx context.Context, arg db.GetUserDailyActivityParams) ([]db.GetUserDailyActivityRow, error) {
	m.ni()
	return nil, nil
}
func (m *mockStore) GetVerificationTokenBatch(ctx context.Context, dollar_1 []string) ([]db.VerificationToken, error) {
	m.ni()
	return nil, nil
}
func (m *mockStore) GetLatestPromptByName(ctx context.Context, name string) (db.PromptTemplateTable, error) {
	m.ni()
	return db.PromptTemplateTable{}, nil
}
func (m *mockStore) InsertErrorLog(ctx context.Context, arg db.InsertErrorLogParams) error {
	m.ni()
	return nil
}
func (m *mockStore) InsertHealthCheck(ctx context.Context, arg db.InsertHealthCheckParams) error {
	m.ni()
	return nil
}
func (m *mockStore) ListAgents(ctx context.Context, arg db.ListAgentsParams) ([]db.AgentsTable, error) {
	m.ni()
	return nil, nil
}
func (m *mockStore) ListAuditLogs(ctx context.Context, arg db.ListAuditLogsParams) ([]db.AuditLog, error) {
	m.ni()
	return nil, nil
}
func (m *mockStore) ListAvailableTeams(ctx context.Context) ([]db.TeamTable, error) {
	m.ni()
	return nil, nil
}
func (m *mockStore) ListBudgets(ctx context.Context) ([]db.BudgetTable, error) {
	if m.listBudgetsFn != nil {
		return m.listBudgetsFn(ctx)
	}
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) ListCredentials(ctx context.Context) ([]db.CredentialTable, error) {
	if m.listCredentialsFn != nil {
		return m.listCredentialsFn(ctx)
	}
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) ListCredentialsByOrg(ctx context.Context, organizationID *string) ([]db.CredentialTable, error) {
	if m.listCredentialsByOrgFn != nil {
		return m.listCredentialsByOrgFn(ctx, organizationID)
	}
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) ListDistinctKeyAliases(ctx context.Context) ([]*string, error) {
	m.ni()
	return nil, nil
}
func (m *mockStore) ListEnabledPlugins(ctx context.Context) ([]db.ClaudeCodePluginTable, error) {
	m.ni()
	return nil, nil
}
func (m *mockStore) ListEndUsers(ctx context.Context) ([]db.EndUserTable2, error) {
	m.ni()
	return nil, nil
}
func (m *mockStore) ListErrorLogs(ctx context.Context, arg db.ListErrorLogsParams) ([]db.ErrorLog, error) {
	m.ni()
	return nil, nil
}
func (m *mockStore) ListGuardrailConfigs(ctx context.Context) ([]db.GuardrailConfigTable, error) {
	if m.listGuardrailConfigsFn != nil {
		return m.listGuardrailConfigsFn(ctx)
	}
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) ListHealthChecks(ctx context.Context, arg db.ListHealthChecksParams) ([]db.HealthCheckTable, error) {
	m.ni()
	return nil, nil
}
func (m *mockStore) ListIPWhitelist(ctx context.Context) ([]db.IPWhitelistTable, error) {
	m.ni()
	return nil, nil
}
func (m *mockStore) ListMCPServers(ctx context.Context) ([]db.MCPServerTable, error) {
	if m.listMCPServersFn != nil {
		return m.listMCPServersFn(ctx)
	}
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) ListPlugins(ctx context.Context, arg db.ListPluginsParams) ([]db.ClaudeCodePluginTable, error) {
	m.ni()
	return nil, nil
}
func (m *mockStore) ListPolicies(ctx context.Context) ([]db.PolicyTable, error) {
	if m.listPoliciesFn != nil {
		return m.listPoliciesFn(ctx)
	}
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) ListPolicyAttachments(ctx context.Context) ([]db.PolicyAttachmentTable, error) {
	if m.listPolicyAttachmentsFn != nil {
		return m.listPolicyAttachmentsFn(ctx)
	}
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) ListPolicyAttachmentsByPolicy(ctx context.Context, policyName string) ([]db.PolicyAttachmentTable, error) {
	m.ni()
	return nil, nil
}
func (m *mockStore) ListPromptTemplates(ctx context.Context) ([]db.PromptTemplateTable, error) {
	m.ni()
	return nil, nil
}
func (m *mockStore) ListProxyModels(ctx context.Context) ([]db.ProxyModelTable, error) {
	if m.listProxyModelsFn != nil {
		return m.listProxyModelsFn(ctx)
	}
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) ListSkills(ctx context.Context, arg db.ListSkillsParams) ([]db.SkillsTable, error) {
	m.ni()
	return nil, nil
}
func (m *mockStore) ListTags(ctx context.Context) ([]db.TagTable, error) {
	if m.listTagsFn != nil {
		return m.listTagsFn(ctx)
	}
	return nil, fmt.Errorf("not mocked")
}
func (m *mockStore) PatchAgent(ctx context.Context, arg db.PatchAgentParams) (db.AgentsTable, error) {
	m.ni()
	return db.AgentsTable{}, nil
}
func (m *mockStore) RegenerateVerificationToken(ctx context.Context, arg db.RegenerateVerificationTokenParams) (db.VerificationToken, error) {
	m.ni()
	return db.VerificationToken{}, nil
}
func (m *mockStore) RemoveTeamModel(ctx context.Context, arg db.RemoveTeamModelParams) error {
	m.ni()
	return nil
}
func (m *mockStore) ResetAllKeySpend(ctx context.Context) error {
	if m.resetAllKeySpendFn != nil {
		return m.resetAllKeySpendFn(ctx)
	}
	return fmt.Errorf("not mocked")
}
func (m *mockStore) ResetAllTeamSpend(ctx context.Context) error {
	if m.resetAllTeamSpendFn != nil {
		return m.resetAllTeamSpendFn(ctx)
	}
	return fmt.Errorf("not mocked")
}
func (m *mockStore) ResetTeamSpend(ctx context.Context, teamID string) error { m.ni(); return nil }
func (m *mockStore) ResetVerificationTokenSpend(ctx context.Context, token string) error {
	m.ni()
	return nil
}
func (m *mockStore) SetTeamCallback(ctx context.Context, arg db.SetTeamCallbackParams) error {
	m.ni()
	return nil
}
func (m *mockStore) SetTeamPermissions(ctx context.Context, arg db.SetTeamPermissionsParams) error {
	m.ni()
	return nil
}
func (m *mockStore) UnblockEndUser(ctx context.Context, id string) (db.EndUserTable2, error) {
	m.ni()
	return db.EndUserTable2{}, nil
}
func (m *mockStore) UnblockTeam(ctx context.Context, teamID string) error { m.ni(); return nil }
func (m *mockStore) UpdateAccessGroup(ctx context.Context, arg db.UpdateAccessGroupParams) error {
	m.ni()
	return nil
}
func (m *mockStore) UpdateAgent(ctx context.Context, arg db.UpdateAgentParams) (db.AgentsTable, error) {
	m.ni()
	return db.AgentsTable{}, nil
}
func (m *mockStore) UpdateBudget(ctx context.Context, arg db.UpdateBudgetParams) (db.BudgetTable, error) {
	if m.updateBudgetFn != nil {
		return m.updateBudgetFn(ctx, arg)
	}
	return db.BudgetTable{}, fmt.Errorf("not mocked")
}
func (m *mockStore) UpdateCredential(ctx context.Context, arg db.UpdateCredentialParams) error {
	if m.updateCredentialFn != nil {
		return m.updateCredentialFn(ctx, arg)
	}
	return fmt.Errorf("not mocked")
}
func (m *mockStore) UpdateEndUser(ctx context.Context, arg db.UpdateEndUserParams) (db.EndUserTable2, error) {
	m.ni()
	return db.EndUserTable2{}, nil
}
func (m *mockStore) UpdateGuardrailConfig(ctx context.Context, arg db.UpdateGuardrailConfigParams) (db.GuardrailConfigTable, error) {
	if m.updateGuardrailConfigFn != nil {
		return m.updateGuardrailConfigFn(ctx, arg)
	}
	return db.GuardrailConfigTable{}, fmt.Errorf("not mocked")
}
func (m *mockStore) UpdateMCPServer(ctx context.Context, arg db.UpdateMCPServerParams) (db.MCPServerTable, error) {
	if m.updateMCPServerFn != nil {
		return m.updateMCPServerFn(ctx, arg)
	}
	return db.MCPServerTable{}, fmt.Errorf("not mocked")
}
func (m *mockStore) UpdatePolicy(ctx context.Context, arg db.UpdatePolicyParams) (db.PolicyTable, error) {
	if m.updatePolicyFn != nil {
		return m.updatePolicyFn(ctx, arg)
	}
	return db.PolicyTable{}, fmt.Errorf("not mocked")
}
func (m *mockStore) UpdateProxyModel(ctx context.Context, arg db.UpdateProxyModelParams) (db.ProxyModelTable, error) {
	if m.updateProxyModelFn != nil {
		return m.updateProxyModelFn(ctx, arg)
	}
	return db.ProxyModelTable{}, fmt.Errorf("not mocked")
}
func (m *mockStore) UpdateTag(ctx context.Context, arg db.UpdateTagParams) (db.TagTable, error) {
	if m.updateTagFn != nil {
		return m.updateTagFn(ctx, arg)
	}
	return db.TagTable{}, fmt.Errorf("not mocked")
}
func (m *mockStore) UpdateTeamMemberRole(ctx context.Context, arg db.UpdateTeamMemberRoleParams) error {
	m.ni()
	return nil
}
func (m *mockStore) UpdateUser(ctx context.Context, arg db.UpdateUserParams) (db.UserTable, error) {
	m.ni()
	return db.UserTable{}, nil
}

// Compile-time check
var _ db.Store = (*mockStore)(nil)
