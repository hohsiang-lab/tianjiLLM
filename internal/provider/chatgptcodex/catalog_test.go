package chatgptcodex

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodexCatalogClientParsesReasoningModalitiesAndTiers(t *testing.T) {
	body, err := os.ReadFile("testdata/codex_catalog_sol_terra_luna.json")
	require.NoError(t, err)
	catalog, err := ParseCatalog(body)
	require.NoError(t, err)
	require.Len(t, catalog.Models, 3)
	sol, ok := catalog.Model("gpt-5.6-sol")
	require.True(t, ok)
	require.Equal(t, int64(372000), *sol.ContextWindow)
	require.Equal(t, int64(872000), *sol.MaxContextWindow)
	require.True(t, sol.UseResponsesLite)
	require.True(t, sol.AutoCompactTokenLimit.Present)
	require.Nil(t, sol.AutoCompactTokenLimit.Value)
	require.Equal(t, []string{"text", "image"}, sol.InputModalities)
	require.Equal(t, "max", sol.DefaultReasoningLevel)
	require.Equal(t, "priority", sol.ServiceTiers[0].ID)
}

func TestCodexCatalogClientFetchesCatalogWithSubscriptionHeaders(t *testing.T) {
	body, err := os.ReadFile("testdata/codex_catalog_sol_terra_luna.json")
	require.NoError(t, err)

	var gotPath string
	var gotQuery string
	var gotHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		gotPath = request.URL.Path
		gotQuery = request.URL.Query().Get("client_version")
		gotHeaders = request.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer server.Close()

	client := CatalogClient{
		BaseURL:       server.URL,
		Originator:    "codex_cli_rs",
		ClientVersion: "0.144.1",
		HTTPClient:    server.Client(),
	}
	catalog, err := client.FetchCatalog(context.Background(), CatalogRequest{
		AccessToken: "access-secret",
		AccountID:   "acct_123",
	})

	require.NoError(t, err)
	assert.Equal(t, "/backend-api/codex/models", gotPath)
	assert.Equal(t, "0.144.1", gotQuery)
	assert.Equal(t, "Bearer access-secret", gotHeaders.Get("Authorization"))
	assert.Equal(t, "acct_123", gotHeaders.Get("ChatGPT-Account-Id"))
	assert.Equal(t, "codex_cli_rs", gotHeaders.Get("Originator"))
	assert.Equal(t, "application/json", gotHeaders.Get("Accept"))
	assert.Equal(t, "0.144.1", gotHeaders.Get("Version"))
	_, ok := catalog.Model("gpt-5.6-sol")
	assert.True(t, ok)
}

func TestCodexCatalogClientBuildsStockCodexModelsPath(t *testing.T) {
	tests := []struct {
		name    string
		baseURL func(string) string
	}{
		{name: "host", baseURL: func(serverURL string) string { return serverURL }},
		{name: "backend-api", baseURL: func(serverURL string) string { return serverURL + "/backend-api" }},
		{name: "codex", baseURL: func(serverURL string) string { return serverURL + "/backend-api/codex" }},
		{name: "codex-models", baseURL: func(serverURL string) string { return serverURL + "/backend-api/codex/models" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath string
			var gotQuery string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				gotPath = request.URL.Path
				gotQuery = request.URL.Query().Get("client_version")
				_, _ = w.Write([]byte(`{"models":[]}`))
			}))
			defer server.Close()

			_, err := (CatalogClient{
				BaseURL:       tt.baseURL(server.URL),
				ClientVersion: "0.144.1",
				HTTPClient:    server.Client(),
			}).FetchCatalog(context.Background(), CatalogRequest{AccessToken: "access-secret"})

			require.NoError(t, err)
			assert.Equal(t, "/backend-api/codex/models", gotPath)
			assert.Equal(t, "0.144.1", gotQuery)
		})
	}
}

func TestCodexCatalogClientDefaultHTTPClientIsBounded(t *testing.T) {
	client := CatalogClient{}.httpClient()

	require.NotNil(t, client)
	assert.Equal(t, 30*time.Second, client.Timeout)
	assert.NotSame(t, http.DefaultClient, client)
}

func TestCodexCatalogRoundTripsExplicitNullAutoCompactTokenLimit(t *testing.T) {
	body, err := os.ReadFile("testdata/codex_catalog_sol_terra_luna.json")
	require.NoError(t, err)

	catalog, err := ParseCatalog(body)
	require.NoError(t, err)
	persisted, err := json.Marshal(catalog)
	require.NoError(t, err)
	require.Contains(t, string(persisted), `"auto_compact_token_limit":null`)
	require.NotContains(t, string(persisted), `"Present"`)

	var restored Catalog
	require.NoError(t, json.Unmarshal(persisted, &restored))
	sol, ok := restored.Model("gpt-5.6-sol")
	require.True(t, ok)
	require.True(t, sol.UseResponsesLite)
	require.True(t, sol.AutoCompactTokenLimit.Present)
	require.Nil(t, sol.AutoCompactTokenLimit.Value)
}
