package contract

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/praxisllmlab/tianjiLLM/internal/router"
	"github.com/praxisllmlab/tianjiLLM/internal/router/strategy"
	"github.com/praxisllmlab/tianjiLLM/internal/testutil/openaitest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type modelItem struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}

func TestListModelsAndRetrieveModel_UseSameVisibleCatalog(t *testing.T) {
	srv, _ := newDirectOpenAIContractServer(t, openaitest.UpstreamServerOptions{})

	listRecorder := httptest.NewRecorder()
	srv.ServeHTTP(listRecorder, newAuthenticatedContractRequest(http.MethodGet, "/v1/models", ""))
	require.Equal(t, http.StatusOK, listRecorder.Code)

	var list struct {
		Data []modelItem `json:"data"`
	}
	require.NoError(t, json.Unmarshal(listRecorder.Body.Bytes(), &list))
	require.Len(t, list.Data, 1)

	for _, path := range []string{"/v1/models/" + contractModel, "/models/" + contractModel} {
		t.Run(path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			srv.ServeHTTP(recorder, newAuthenticatedContractRequest(http.MethodGet, path, ""))

			require.Equal(t, http.StatusOK, recorder.Code)
			var retrieved modelItem
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &retrieved))
			assert.Equal(t, list.Data[0], retrieved)
		})
	}
}

func TestRetrieveModel_UnknownModelReturnsStandardNotFound(t *testing.T) {
	srv, _ := newDirectOpenAIContractServer(t, openaitest.UpstreamServerOptions{})

	for _, path := range []string{"/v1/models/missing-model", "/models/missing-model"} {
		t.Run(path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			srv.ServeHTTP(recorder, newAuthenticatedContractRequest(http.MethodGet, path, ""))

			require.Equal(t, http.StatusNotFound, recorder.Code)
			err := requireStandardError(t, recorder.Body.Bytes(), "", "model_not_found")
			assert.Equal(t, `model "missing-model" not found`, err.Message)
		})
	}
}

func TestRetrieveModel_UsesExactVisibleCatalogOnly(t *testing.T) {
	models := []config.ModelConfig{
		{ModelName: "visible-model"},
		{ModelName: "hidden-model"},
		{ModelName: "gpt-*"},
	}
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers: &handler.Handlers{
			Config: &config.ProxyConfig{ModelList: models},
			Router: router.New(models, strategy.NewShuffle(), router.RouterSettings{
				ModelGroupAlias: map[string]router.ModelGroupAliasItem{
					"hidden-model": {Model: "visible-model", Hidden: true},
				},
			}),
		},
		MasterKey: contractMasterKey,
	})

	tests := []struct {
		model string
		code  int
	}{
		{"visible-model", http.StatusOK},
		{"hidden-model", http.StatusNotFound},
		{"gpt-4o", http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			srv.ServeHTTP(recorder, newAuthenticatedContractRequest(http.MethodGet, "/v1/models/"+tt.model, ""))
			assert.Equal(t, tt.code, recorder.Code)
		})
	}
}
