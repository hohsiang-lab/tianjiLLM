//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
)

func TestModelOpenAISubscriptionFormHidesPerModelControls(t *testing.T) {
	f := setup(t)
	f.NavigateToModels()

	f.ClickButton("Add Model")
	f.WaitDialogOpen("create-model-dialog")

	html, err := f.Page.Locator("#create-model-dialog").InnerHTML()
	require.NoError(t, err)
	assert.NotContains(t, html, "openai_subscription_credential_ids")
	assert.NotContains(t, html, "openai_subscription_transport")
	assert.NotContains(t, html, "direct_openai_http")
}

func TestModelOpenAISubscriptionCreateNormalizesLegacyFields(t *testing.T) {
	f := setup(t)
	modelName := "openai-global-pool-" + generateTestKey()[20:28]
	ctx := context.Background()

	f.NavigateToModels()
	f.ClickButton("Add Model")
	f.WaitDialogOpen("create-model-dialog")

	const dialog = "#create-model-dialog"
	f.InputByIDIn(dialog, "model_name", modelName)
	f.InputByIDIn(dialog, "model", "openai/gpt-4o")
	f.InputByIDIn(dialog, "api_base", "https://legacy.example/v1")
	f.InputByIDIn(dialog, "api_key", "[REDACTED]")

	f.SubmitDialog("create-model-dialog", "Create")
	f.WaitDialogClose("create-model-dialog")

	dbModel := waitForProxyModelByName(t, ctx, modelName)
	var params map[string]any
	require.NoError(t, json.Unmarshal(dbModel.TianjiParams, &params))

	assert.Equal(t, "openai/gpt-4o", params["model"])
	assert.NotContains(t, params, "api_key")
	assert.NotContains(t, params, "api_base")
	assert.NotContains(t, params, "openai_subscription_credential_ids")
	assert.Equal(t, "chatgpt_codex_backend", params["openai_subscription_transport"])
}

func TestModelOpenAISubscriptionCreateRoutesThroughGlobalCodexPool(t *testing.T) {
	f := setup(t)
	modelName := "openai-global-route-" + generateTestKey()[20:28]
	const selectedToken = "[REDACTED]"
	const selectedAccountID = "[REDACTED]"
	captured := make(chan struct {
		auth    string
		account string
		body    map[string]any
	}, 1)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/backend-api/wham/usage" {
			writeCodexUsageSnapshot(t, w, "model-route@example.com")
			return
		}
		require.Equal(t, "/backend-api/codex/responses", r.URL.Path)
		requestBody, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read generation request: %v", err)
			return
		}
		var payload map[string]any
		if err := json.Unmarshal(requestBody, &payload); err != nil {
			t.Errorf("decode generation request: %v", err)
			return
		}
		captured <- struct {
			auth    string
			account string
			body    map[string]any
		}{
			auth:    r.Header.Get("Authorization"),
			account: r.Header.Get("ChatGPT-Account-Id"),
			body:    payload,
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(strings.Join([]string{
			`data: {"type":"response.output_text.delta","response_id":"resp_model_route","delta":"OK"}`,
			``,
			`data: {"type":"response.completed","response":{"id":"resp_model_route","model":"gpt-4o","usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n")))
	}))
	defer backend.Close()

	oldBaseURL := cfg.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL
	cfg.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = backend.URL
	t.Cleanup(func() {
		cfg.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = oldBaseURL
	})
	f.SeedEncryptedOpenAISubscriptionCredential(SeedCredentialOpts{
		ID:   "cred-model-global-route",
		Name: "Global model route credential",
		Info: map[string]any{"status": "active"},
	}, handler.OpenAISubscriptionTokenBundle{
		AccessToken:  selectedToken,
		RefreshToken: selectedToken,
		ExpiresAt:    time.Now().UTC().Add(time.Hour),
		AccountID:    selectedAccountID,
	})

	f.NavigateToModels()
	f.ClickButton("Add Model")
	f.WaitDialogOpen("create-model-dialog")
	f.InputByIDIn("#create-model-dialog", "model_name", modelName)
	f.InputByIDIn("#create-model-dialog", "model", "openai/gpt-4o")
	f.InputByIDIn("#create-model-dialog", "api_base", "https://legacy.example/v1")
	f.InputByIDIn("#create-model-dialog", "api_key", selectedToken)
	f.SubmitDialog("create-model-dialog", "Create")
	f.WaitDialogClose("create-model-dialog")

	ctx := context.Background()
	dbModel := waitForProxyModelByName(t, ctx, modelName)
	var params map[string]any
	require.NoError(t, json.Unmarshal(dbModel.TianjiParams, &params))
	require.NotContains(t, params, "api_key")
	require.NotContains(t, params, "api_base")
	require.Equal(t, "chatgpt_codex_backend", params["openai_subscription_transport"])

	requestBody, err := json.Marshal(map[string]any{
		"model":  modelName,
		"stream": true,
		"input":  "say OK",
	})
	require.NoError(t, err)
	resp := postProxyResponses(t, string(requestBody))
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, resp.StatusCode, string(body))
	got := <-captured
	assert.Equal(t, "Bearer "+selectedToken, got.auth)
	assert.Equal(t, selectedAccountID, got.account)
	assert.Equal(t, "gpt-4o", got.body["model"])
	assert.Equal(t, true, got.body["stream"])
	assert.NotNil(t, got.body["input"])
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/event-stream")
	assert.Contains(t, string(body), `"type":"response.output_text.delta"`)
	assert.Contains(t, string(body), `"delta":"OK"`)
	assert.Contains(t, string(body), `data: [DONE]`)
	assert.NotContains(t, string(body), selectedToken)
}

func TestModelOpenAISubscriptionEditNormalizesLegacyFields(t *testing.T) {
	f := setup(t)
	modelID := f.SeedModel(SeedModelOpts{
		ModelName: "openai-legacy-model",
		Model:     "openai/gpt-4o",
		APIKey:    "[REDACTED]",
		APIBase:   "https://legacy.example/v1",
		Extra: map[string]any{
			"openai_subscription_credential_ids": []string{"legacy-credential"},
			"openai_subscription_transport":      "direct_openai_http",
		},
	})

	f.NavigateToModels()
	f.Page.Locator("#models-table table tbody tr").First().GetByRole("button", playwright.LocatorGetByRoleOptions{
		Name: "Edit",
	}).Click()
	f.WaitDialogOpen("edit-model-dialog")

	html, err := f.Page.Locator("#edit-model-dialog").InnerHTML()
	require.NoError(t, err)
	assert.NotContains(t, html, "openai_subscription_credential_ids")
	assert.NotContains(t, html, "openai_subscription_transport")
	assert.NotContains(t, html, "direct_openai_http")

	f.SubmitDialog("edit-model-dialog", "Save Changes")
	f.WaitStable()

	dbModel, err := testDB.GetProxyModel(context.Background(), modelID)
	require.NoError(t, err)
	var params map[string]any
	require.NoError(t, json.Unmarshal(dbModel.TianjiParams, &params))

	assert.Equal(t, "openai/gpt-4o", params["model"])
	assert.NotContains(t, params, "api_key")
	assert.NotContains(t, params, "api_base")
	assert.NotContains(t, params, "openai_subscription_credential_ids")
	assert.Equal(t, "chatgpt_codex_backend", params["openai_subscription_transport"])
}
