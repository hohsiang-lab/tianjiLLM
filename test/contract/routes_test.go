package contract

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/proxy"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/praxisllmlab/tianjiLLM/internal/testutil/openaitest"
	"github.com/stretchr/testify/assert"
)

func TestLegacySearchRouteIsAbsent(t *testing.T) {
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  &handler.Handlers{},
		MasterKey: contractMasterKey,
		PassthroughHandler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusTeapot)
		}),
	})
	recorder := httptest.NewRecorder()
	srv.ServeHTTP(recorder, newAuthenticatedContractRequest(http.MethodPost, "/v1/search/legacy", `{}`))

	assert.Equal(t, http.StatusTeapot, recorder.Code)
}

func TestGraphitiAndCogneeSpecificRoutesAreAbsent(t *testing.T) {
	srv, upstream := newDirectOpenAIContractServer(t, openaitest.UpstreamServerOptions{})

	for _, path := range []string{
		"/v1/graphiti/chat/completions",
		"/v1/graphiti/embeddings",
		"/v1/cognee/chat/completions",
		"/v1/cognee/embeddings",
	} {
		t.Run(path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			srv.ServeHTTP(recorder, newAuthenticatedContractRequest(http.MethodPost, path, `{}`))
			assert.False(t, recorder.Code >= 200 && recorder.Code < 300, "unexpected project-specific handler for %s", path)
		})
	}

	assert.Empty(t, upstream.Requests())
}
