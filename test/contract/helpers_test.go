package contract

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/praxisllmlab/tianjiLLM/internal/testutil/openaitest"
)

func init() {
	provider.Register("openaicompat", openai.New())
}

const (
	contractMasterKey = "sk-master"
	contractModel     = "contract-model"
)

func newAuthenticatedContractRequest(method, path, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+contractMasterKey)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

func newDirectOpenAIContractServer(t *testing.T, upstreamOptions openaitest.UpstreamServerOptions) (*proxy.Server, *openaitest.UpstreamServer) {
	t.Helper()
	return newOpenAIContractServer(t, upstreamOptions, model.CapabilityRecord{
		SupportsStream:                    true,
		SupportsNonStream:                 true,
		SupportsResponseFormat:            true,
		SupportsJSONObject:                true,
		SupportsJSONSchema:                true,
		SupportsTools:                     true,
		SupportsToolChoice:                true,
		SupportsTemperature:               true,
		SupportsTopP:                      true,
		SupportsMaxTokens:                 true,
		SupportsMaxCompletionTokens:       true,
		SupportsStreamOptionsIncludeUsage: true,
		SupportsEmbeddings:                true,
	})
}

func newStreamOnlyContractServer(t *testing.T, upstreamOptions openaitest.UpstreamServerOptions) (*proxy.Server, *openaitest.UpstreamServer) {
	t.Helper()
	return newOpenAIContractServer(t, upstreamOptions, model.CapabilityRecord{
		SupportsStream:                    true,
		SupportsStreamOptionsIncludeUsage: true,
	})
}

func newOpenAIContractServer(t *testing.T, upstreamOptions openaitest.UpstreamServerOptions, capability model.CapabilityRecord) (*proxy.Server, *openaitest.UpstreamServer) {
	t.Helper()
	upstream := openaitest.NewUpstreamServer(t, upstreamOptions)
	return newOpenAIContractProxy(upstream.BaseURL(), capability), upstream
}

func newOpenAIContractProxy(apiBase string, capability model.CapabilityRecord) *proxy.Server {
	apiKey := "sk-upstream"
	cfg := &config.ProxyConfig{
		ModelList: []config.ModelConfig{{
			ModelName: contractModel,
			TianjiParams: config.TianjiParams{
				Model:   "openaicompat/gpt-4o",
				APIKey:  &apiKey,
				APIBase: &apiBase,
			},
		}},
		GeneralSettings: config.GeneralSettings{MasterKey: contractMasterKey},
	}
	handlers := &handler.Handlers{
		Config: cfg,
		Capabilities: model.CapabilityMatrix{
			{Backend: model.BackendDirectOpenAIHTTP, Model: contractModel}: capability,
		},
	}
	return proxy.NewServer(proxy.ServerConfig{Handlers: handlers, MasterKey: contractMasterKey})
}
