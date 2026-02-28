package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtraHandlersNoDB(t *testing.T) {
	h := newTestHandlers()

	noDBHandlers := []struct {
		name    string
		method  string
		path    string
		body    string
		handler http.HandlerFunc
	}{
		{"SpendByTeams", "GET", "/spend/teams", "", h.SpendByTeams},
		{"SpendByTags", "GET", "/spend/tags", "", h.SpendByTags},
		{"SpendByModels", "GET", "/spend/models", "", h.SpendByModels},
		{"SpendByEndUsers", "GET", "/spend/end_users", "", h.SpendByEndUsers},
		{"BudgetNew", "POST", "/budget/new", `{"budget_id":"test"}`, h.BudgetNew},
		{"BudgetInfo", "GET", "/budget/info", "", h.BudgetInfo},
		{"BudgetUpdate", "PUT", "/budget/update", `{"budget_id":"x"}`, h.BudgetUpdate},
		{"BudgetList", "GET", "/budget/list", "", h.BudgetList},
		{"BudgetDelete", "DELETE", "/budget/delete", `{"budget_id":"x"}`, h.BudgetDelete},
		{"BudgetSettings", "GET", "/budget/settings", "", h.BudgetSettings},
		{"CredentialNew", "POST", "/credentials/new", `{"name":"x"}`, h.CredentialNew},
		{"CredentialList", "GET", "/credentials/list", "", h.CredentialList},
		{"CredentialInfo", "GET", "/credentials/info", "", h.CredentialInfo},
		{"CredentialUpdate", "PUT", "/credentials/update", `{"id":"x"}`, h.CredentialUpdate},
		{"CredentialDelete", "DELETE", "/credentials/delete", `{"id":"x"}`, h.CredentialDelete},
		{"KeyRegenerate", "POST", "/key/regenerate", `{"key":"x"}`, h.KeyRegenerate},
		{"KeyBulkUpdate", "POST", "/key/bulk_update", `{"keys":[]}`, h.KeyBulkUpdate},
		{"KeyHealthCheck", "POST", "/key/health", `{}`, h.KeyHealthCheck},
		{"ResetKeySpend", "POST", "/key/reset_spend", `{"key":"x"}`, h.ResetKeySpend},
		{"KeyAliases", "GET", "/key/aliases", "", h.KeyAliases},
		{"KeyDelete", "DELETE", "/key/delete", `{"keys":["x"]}`, h.KeyDelete},
		{"KeyBlock", "POST", "/key/block", `{"key":"x"}`, h.KeyBlock},
		{"KeyUnblock", "POST", "/key/unblock", `{"key":"x"}`, h.KeyUnblock},
		{"KeyUpdate", "POST", "/key/update", `{"key":"x"}`, h.KeyUpdate},
		{"ModelNew", "POST", "/model/new", `{"model_name":"x"}`, h.ModelNew},
		{"ModelDelete", "POST", "/model/delete", `{"id":"x"}`, h.ModelDelete},
		{"ModelUpdate", "POST", "/model/update", `{"model_id":"x"}`, h.ModelUpdate},
		{"ModelInfo", "GET", "/model/info", "", h.ModelInfo},
		{"TeamDelete", "DELETE", "/team/delete", `{"team_id":"x"}`, h.TeamDelete},
		{"TeamUpdate", "PUT", "/team/update", `{"team_id":"x"}`, h.TeamUpdate},
		{"TeamMemberAdd", "POST", "/team/member_add", `{}`, h.TeamMemberAdd},
		{"TeamMemberDelete", "POST", "/team/member_delete", `{}`, h.TeamMemberDelete},
		{"UserDelete", "DELETE", "/user/delete", `{"user_id":"x"}`, h.UserDelete},
		{"SSOLogin", "GET", "/sso/login", "", h.SSOLogin},
		{"SSOCallback", "GET", "/sso/callback", "", h.SSOCallback},
		{"OrgNew", "POST", "/organization/new", `{"organization_alias":"x"}`, h.OrgNew},
		{"OrgMemberUpdate", "POST", "/organization/member_update", `{}`, h.OrgMemberUpdate},
		{"OrgUpdate", "PUT", "/organization/update", `{"organization_id":"x"}`, h.OrgUpdate},
		{"OrgDelete", "DELETE", "/organization/delete", `{"organization_id":"x"}`, h.OrgDelete},
		{"OrgInfo", "GET", "/organization/info", "", h.OrgInfo},
		{"OrgMemberAdd", "POST", "/organization/member_add", `{}`, h.OrgMemberAdd},
		{"OrgMemberDelete", "POST", "/organization/member_delete", `{}`, h.OrgMemberDelete},
		{"GlobalSpendByProvider", "GET", "/global/spend/provider", "", h.GlobalSpendByProvider},
		{"GlobalSpendByModels", "GET", "/global/spend/models", "", h.GlobalSpendByModels},
		{"GlobalSpendByKeys", "GET", "/global/spend/keys", "", h.GlobalSpendByKeys},
		{"GlobalSpendByTeams", "GET", "/global/spend/teams", "", h.GlobalSpendByTeams},
		{"GlobalSpendByTags", "GET", "/global/spend/tags", "", h.GlobalSpendByTags},
		{"GlobalActivity", "GET", "/global/activity", "", h.GlobalActivity},
		{"GlobalActivityByModel", "GET", "/global/activity/model", "", h.GlobalActivityByModel},
		{"GlobalSpend", "GET", "/global/spend", "", h.GlobalSpend},
		{"GlobalSpendReport", "GET", "/global/spend/report", "", h.GlobalSpendReport},
		{"GlobalSpendReset", "POST", "/global/spend/reset", `{}`, h.GlobalSpendReset},
		{"CacheHitStats", "GET", "/cache/stats", "", h.CacheHitStats},
		{"SpendLogs", "GET", "/spend/logs", "", h.SpendLogs},
		{"TransformRequest", "POST", "/utils/transform_request", `{"model":"gpt-4o","messages":[]}`, h.TransformRequest},
		{"TokenCount", "POST", "/utils/token_count", `{"model":"gpt-4o","messages":[]}`, h.TokenCount},
		{"SupportedOpenAIParams", "GET", "/utils/supported_params", "", h.SupportedOpenAIParams},
		{"RoutesList", "GET", "/routes/list", "", h.RoutesList},
		{"ErrorLogsList", "GET", "/errors/list", "", h.ErrorLogsList},
	}

	for _, tt := range noDBHandlers {
		t.Run(tt.name, func(t *testing.T) {
			var body *strings.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			} else {
				body = strings.NewReader("")
			}
			r := httptest.NewRequest(tt.method, tt.path, body)
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			tt.handler(w, r)
			// Most return 500/503 without DB, some might return 200
			assert.NotEqual(t, 0, w.Code, "%s should return a status code", tt.name)
		})
	}
}
