package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssistantsProxy_NotConfigured(t *testing.T) {
	h := newTestHandlers()

	endpoints := []struct {
		name string
		fn   func(http.ResponseWriter, *http.Request)
	}{
		{"AssistantCreate", h.AssistantCreate},
		{"AssistantGet", h.AssistantGet},
		{"AssistantList", h.AssistantList},
		{"AssistantModify", h.AssistantModify},
		{"AssistantDelete", h.AssistantDelete},
		{"ThreadCreate", h.ThreadCreate},
		{"ThreadGet", h.ThreadGet},
		{"ThreadModify", h.ThreadModify},
		{"ThreadDelete", h.ThreadDelete},
		{"MessageCreate", h.MessageCreate},
		{"MessageList", h.MessageList},
		{"MessageGet", h.MessageGet},
		{"RunCreate", h.RunCreate},
		{"RunGet", h.RunGet},
		{"RunList", h.RunList},
		{"RunCancel", h.RunCancel},
		{"RunStepsList", h.RunStepsList},
		{"RunStepGet", h.RunStepGet},
	}

	for _, ep := range endpoints {
		t.Run(ep.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", "/v1/assistants", nil)
			ep.fn(w, r)
			assert.Equal(t, http.StatusNotImplemented, w.Code)
		})
	}
}

func TestOpenAIProxy_ScrubsInboundAuthAliases(t *testing.T) {
	const selectedCredential = "[REDACTED]"
	aliases := []string{"api-key", "x-api-key", "x-goog-api-key", "Proxy-Authorization"}
	headersCh := make(chan http.Header, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headersCh <- r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/assistants", strings.NewReader(`{}`))
	for _, name := range append([]string{"Authorization"}, aliases...) {
		req.Header.Set(name, "[REDACTED]")
	}
	w := httptest.NewRecorder()

	(&Handlers{}).proxyOpenAIUpstream(w, req, upstream.URL+"/api/v1", selectedCredential, true)

	require.Equal(t, http.StatusOK, w.Code)
	got := <-headersCh
	require.Equal(t, []string{"Bearer " + selectedCredential}, got.Values("Authorization"))
	for _, name := range aliases {
		assert.Empty(t, got.Values(name), name+" must not cross the OpenAI proxy boundary")
	}
}

func TestOpenAISubscriptionProxy_ScrubsInboundAuthAliases(t *testing.T) {
	const selectedCredential = "[REDACTED]"
	aliases := []string{"api-key", "x-api-key", "x-goog-api-key", "Proxy-Authorization"}
	headersCh := make(chan http.Header, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headersCh <- r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/assistants", strings.NewReader(`{}`))
	for _, name := range append([]string{"Authorization"}, aliases...) {
		req.Header.Set(name, "[REDACTED]")
	}
	w := httptest.NewRecorder()

	(&Handlers{}).proxyOpenAIUpstreamWithSubscriptionCandidates(w, req, upstream.URL+"/api/v1", []resolvedOpenAISubscriptionCredential{{BearerToken: selectedCredential}})

	require.Equal(t, http.StatusOK, w.Code)
	got := <-headersCh
	require.Equal(t, []string{"Bearer " + selectedCredential}, got.Values("Authorization"))
	for _, name := range aliases {
		assert.Empty(t, got.Values(name), name+" must not cross the OpenAI subscription proxy boundary")
	}
}

func TestOpenAISubscriptionFailoverTransport_ScrubsInboundAuthAliasesOnRetry(t *testing.T) {
	const selectedCredential = "[REDACTED]"
	aliases := []string{"api-key", "x-api-key", "x-goog-api-key", "Proxy-Authorization"}
	var calls atomic.Int32
	headersCh := make(chan http.Header, 2)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headersCh <- r.Header.Clone()
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	req := httptest.NewRequest(http.MethodPost, upstream.URL+"/v1/assistants", strings.NewReader(`{}`))
	for _, name := range append([]string{"Authorization"}, aliases...) {
		req.Header.Set(name, "[REDACTED]")
	}
	transport := openAISubscriptionFailoverTransport{
		base:       upstream.Client().Transport,
		candidates: []resolvedOpenAISubscriptionCredential{{CredentialID: "credential", BearerToken: selectedCredential}},
		refresh: func(context.Context, string) (resolvedOpenAISubscriptionCredential, error) {
			return resolvedOpenAISubscriptionCredential{BearerToken: selectedCredential}, nil
		},
	}

	resp, err := transport.RoundTrip(req)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
	first, second := <-headersCh, <-headersCh
	for _, headers := range []http.Header{first, second} {
		require.Equal(t, []string{"Bearer " + selectedCredential}, headers.Values("Authorization"))
		for _, name := range aliases {
			assert.Empty(t, headers.Values(name), name+" must not cross the retry boundary")
		}
	}
}
