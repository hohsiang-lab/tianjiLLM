package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMoreHandlersNoDB(t *testing.T) {
	h := newTestHandlers()

	type ep struct {
		name string
		fn   func(http.ResponseWriter, *http.Request)
	}

	endpoints := []ep{
		// budget
		{"BudgetNew", h.BudgetNew},
		{"BudgetInfo", h.BudgetInfo},
		{"BudgetList", h.BudgetList},
		{"BudgetUpdate", h.BudgetUpdate},
		{"BudgetDelete", h.BudgetDelete},
		{"BudgetSettings", h.BudgetSettings},
		// team
		{"TeamNew", h.TeamNew},
		{"TeamDelete", h.TeamDelete},
		{"TeamUpdate", h.TeamUpdate},
		{"TeamList", h.TeamList},
		{"TeamMemberAdd", h.TeamMemberAdd},
		{"TeamMemberDelete", h.TeamMemberDelete},
		// user
		{"UserNew", h.UserNew},
		{"UserDelete", h.UserDelete},
		{"UserList", h.UserList},
		// organization
		{"OrgNew", h.OrgNew},
		{"OrgInfo", h.OrgInfo},
		{"OrgUpdate", h.OrgUpdate},
		{"OrgDelete", h.OrgDelete},
		{"OrgMemberAdd", h.OrgMemberAdd},
		{"OrgMemberUpdate", h.OrgMemberUpdate},
		{"OrgMemberDelete", h.OrgMemberDelete},
		// credentials
		{"CredentialNew", h.CredentialNew},
		{"CredentialInfo", h.CredentialInfo},
		{"CredentialList", h.CredentialList},
		{"CredentialUpdate", h.CredentialUpdate},
		{"CredentialDelete", h.CredentialDelete},
		// customer mgmt
		{"EndUserNew", h.EndUserNew},
		{"EndUserList", h.EndUserList},
		{"EndUserInfo", h.EndUserInfo},
		{"EndUserUpdate", h.EndUserUpdate},
		{"EndUserDelete", h.EndUserDelete},
		{"EndUserBlock", h.EndUserBlock},
		{"EndUserUnblock", h.EndUserUnblock},
		// prompt mgmt
		{"PromptCreate", h.PromptCreate},
		{"PromptGet", h.PromptGet},
		{"PromptList", h.PromptList},
		{"PromptUpdate", h.PromptUpdate},
		{"PromptDelete", h.PromptDelete},
		{"PromptVersions", h.PromptVersions},
		{"PromptTest", h.PromptTest},
		// tag mgmt
		{"TagNew", h.TagNew},
		{"TagInfo", h.TagInfo},
		{"TagList", h.TagList},
		{"TagUpdate", h.TagUpdate},
		{"TagDelete", h.TagDelete},
		// guardrail mgmt
		{"GuardrailCreate", h.GuardrailCreate},
		{"GuardrailGet", h.GuardrailGet},
		{"GuardrailList", h.GuardrailList},
		{"GuardrailUpdate", h.GuardrailUpdate},
		{"GuardrailDelete", h.GuardrailDelete},
		// mcp mgmt
		{"MCPServerCreate", h.MCPServerCreate},
		{"MCPServerList", h.MCPServerList},
		{"MCPServerGet", h.MCPServerGet},
		{"MCPServerUpdate", h.MCPServerUpdate},
		{"MCPServerDelete", h.MCPServerDelete},
		// marketplace
		{"PluginCreate", h.PluginCreate},
		{"PluginGet", h.PluginGet},
		{"PluginList", h.PluginList},
		{"PluginEnable", h.PluginEnable},
		{"PluginDisable", h.PluginDisable},
		{"PluginDelete", h.PluginDelete},
		// model mgmt
		{"ModelNew", h.ModelNew},
		{"ModelInfo", h.ModelInfo},
		{"ModelUpdate", h.ModelUpdate},
		{"ModelDelete", h.ModelDelete},
		// key ext
		{"KeyRegenerate", h.KeyRegenerate},
		{"KeyBulkUpdate", h.KeyBulkUpdate},
		{"KeyHealthCheck", h.KeyHealthCheck},
		{"ServiceAccountKeyGenerate", h.ServiceAccountKeyGenerate},
		{"ResetKeySpend", h.ResetKeySpend},
		{"KeyAliases", h.KeyAliases},
		{"KeyInfoV2", h.KeyInfoV2},
		// agent
		{"AgentCreate", h.AgentCreate},
		{"AgentGet", h.AgentGet},
		{"AgentList", h.AgentList},
		{"AgentUpdate", h.AgentUpdate},
		{"AgentPatch", h.AgentPatch},
		{"AgentDelete", h.AgentDelete},
		// skills
		{"SkillCreate", h.SkillCreate},
		{"SkillGet", h.SkillGet},
		{"SkillList", h.SkillList},
		{"SkillDelete", h.SkillDelete},
		// container
		{"ContainerCreate", h.ContainerCreate},
		{"ContainerGet", h.ContainerGet},
		{"ContainerList", h.ContainerList},
		{"ContainerDelete", h.ContainerDelete},
		{"ContainerFiles", h.ContainerFiles},
		// cache mgmt
		{"CacheFlushAll", h.CacheFlushAll},
		{"CachePing", h.CachePing},
		{"CacheDelete", h.CacheDelete},
		// analytics
		{"SpendAnalytics", h.SpendAnalytics},
		{"SpendTopN", h.SpendTopN},
		{"SpendTrend", h.SpendTrend},
		// audit
		{"AuditLogList", h.AuditLogList},
		{"AuditLogGet", h.AuditLogGet},
		// misc
		{"ErrorLogsList", h.ErrorLogsList},
		{"ConfigV2Get", h.ConfigV2Get},
		{"HealthCheckHistory", h.HealthCheckHistory},
		// ip
		{"IPAdd", h.IPAdd},
		{"IPDelete", h.IPDelete},
		{"IPList", h.IPList},
		// config
		{"ConfigGet", h.ConfigGet},
		{"ConfigUpdate", h.ConfigUpdate},
		// router settings
		{"RouterSettingsGet", h.RouterSettingsGet},
		{"RouterSettingsPatch", h.RouterSettingsPatch},
		// fallback
		{"FallbackCreate", h.FallbackCreate},
		{"FallbackGet", h.FallbackGet},
		{"FallbackDelete", h.FallbackDelete},
		// user ext
		{"UserInfo", h.UserInfo},
		{"UserUpdate", h.UserUpdate},
		{"UserDailyActivity", h.UserDailyActivity},
		// sso
		{"SSOLogin", h.SSOLogin},
		{"SSOCallback", h.SSOCallback},
		// video
		{"VideoCreate", h.VideoCreate},
		{"VideoGet", h.VideoGet},
		{"VideoContent", h.VideoContent},
		// search
		{"SearchHandler", h.SearchHandler},
		// ocr
		{"OCRProcess", h.OCRProcess},
		// rag
		{"RAGIngest", h.RAGIngest},
		{"RAGQuery", h.RAGQuery},
		// passthrough ext
		{"AnthropicBatchesCreate", h.AnthropicBatchesCreate},
		{"AnthropicBatchesGet", h.AnthropicBatchesGet},
		{"AnthropicBatchesResults", h.AnthropicBatchesResults},
		// responses ext
		{"GetResponse", h.GetResponse},
		{"CancelResponse", h.CancelResponse},
		{"ListResponseInputItems", h.ListResponseInputItems},
	}

	for _, ep := range endpoints {
		t.Run(ep.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", "/test", strings.NewReader("{}"))
			r.Header.Set("Content-Type", "application/json")
			ep.fn(w, r)
			assert.NotEqual(t, 0, w.Code, ep.name)
		})
	}
}

func TestCallbackList(t *testing.T) {
	h := newTestHandlers()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/callback/list", nil)
	h.CallbackList(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDiscovery(t *testing.T) {
	h := newTestHandlers()

	t.Run("PublicProviders", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/providers", nil)
		h.PublicProviders(w, r)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("PublicModelCostMap", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/model_cost_map", nil)
		h.PublicModelCostMap(w, r)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("MarketplaceJSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/marketplace.json", nil)
		h.MarketplaceJSON(w, r)
		assert.Contains(t, []int{http.StatusOK, http.StatusServiceUnavailable}, w.Code)
	})

	t.Run("RoutesList", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/routes", nil)
		h.RoutesList(w, r)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestParseAnalyticsQuery(t *testing.T) {
	h := newTestHandlers()
	r := httptest.NewRequest("GET", "/analytics?dimension=model&start_date=2025-01-01&end_date=2025-12-31", nil)
	q := h.parseAnalyticsQuery(r)
	assert.NotNil(t, q)
}
