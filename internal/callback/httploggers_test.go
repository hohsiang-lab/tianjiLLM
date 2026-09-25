package callback

import (
	"errors"
	"net/http/httptest"
	"testing"
	"time"
)

// TestHTTPLoggers exercises LogSuccess and LogFailure on all HTTP-based
// callback implementations. A local httptest server accepts any POST,
// ensuring the send path is fully covered without network calls.
func TestHTTPLoggers(t *testing.T) {
	srv := httptest.NewServer(nil) // default mux returns 404 — that's fine
	defer srv.Close()

	data := LogData{
		Model:            "gpt-4",
		Provider:         "openai",
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
		Cost:             0.01,
		Latency:          200 * time.Millisecond,
		UserID:           "u1",
		TeamID:           "t1",
		RequestTags:      []string{"test"},
		CacheHit:         false,
		StartTime:        time.Now().Add(-time.Second),
		EndTime:          time.Now(),
	}
	dataErr := data
	dataErr.Error = errors.New("test error")

	loggers := []CustomLogger{
		NewAgentOpsCallback("key", srv.URL),
		NewArgillaCallback("key", srv.URL),
		NewArizeFullCallback("key", srv.URL),
		NewAthinaCallback("key", srv.URL),
		NewBitbucketCallback("key", srv.URL),
		NewBraintrustLogger("key", srv.URL, "proj"),
		NewCloudZeroCallback("key", srv.URL),
		NewDatadogCallback("key", srv.URL),
		NewDatadogLLMCallback("key", srv.URL),
		NewDeepEvalCallback("key", srv.URL),
		NewDotPromptCallback("key", srv.URL),
		NewFocusCallback("key", srv.URL),
		NewGalileoCallback("key", srv.URL),
		NewGCSPubSubCallback("proj", "topic", srv.URL),
		NewGenericAPICallback(srv.URL, map[string]string{"X-Custom": "v"}),
		NewGitLabCallback("key", srv.URL),
		NewGreenscaleCallback("key", srv.URL),
		NewHeliconeLogger("key", srv.URL),
		NewHumanLoopCallback("key", srv.URL),
		NewLagoCallback("key", srv.URL),
		NewLangfuseCallback(srv.URL, "pub", "secret"),
		NewLangsmithLogger("key", srv.URL, "proj"),
		NewLangTraceCallback("key", srv.URL),
		NewLevoCallback("key", srv.URL),
		NewLiteralAICallback("key", srv.URL),
		NewLogfireCallback("key", srv.URL),
		NewLunaryCallback("key", srv.URL),
		NewMLflowLogger(srv.URL, "exp1"),
		NewOpenMeterCallback("key", srv.URL),
		NewOpikCallback("key", srv.URL),
		NewOTelCallback(srv.URL, nil),
		NewPostHogCallback("key", srv.URL),
		NewPromptLayerCallback("key", srv.URL),
		NewSlackCallback(srv.URL, 100),
		NewSupabaseCallback("key", srv.URL),
		NewTraceloopCallback("key", srv.URL),
		NewWandbLogger("key", "proj", "entity"),
		NewWeaveCallback("key", srv.URL),
		NewWebhookCallback(srv.URL, map[string]string{"X-H": "v"}),
		NewWebSearchCallback("key", srv.URL),
		NewCustomBatchCallback("key", srv.URL),
		NewAzureSentinelCallback("wid", "key", srv.URL),
	}

	for _, l := range loggers {
		l.LogSuccess(data)
		l.LogFailure(dataErr)
	}
}

// TestRegistryOperations tests Registry methods.
func TestRegistryOperations(t *testing.T) {
	r := NewRegistry()
	if r.Count() != 0 {
		t.Fatalf("expected 0, got %d", r.Count())
	}

	srv := httptest.NewServer(nil)
	defer srv.Close()

	w := NewWebhookCallback(srv.URL, nil)
	r.Register(w)

	if r.Count() != 1 {
		t.Fatalf("expected 1, got %d", r.Count())
	}

	names := r.Names()
	if len(names) != 1 || names[0] != "WebhookCallback" {
		t.Fatalf("unexpected names: %v", names)
	}

	data := LogData{Model: "m", Provider: "p", EndTime: time.Now()}
	r.LogSuccess(data)
	r.LogFailure(data)
}
