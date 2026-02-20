package callback

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPrometheusCallback_LogSuccess(t *testing.T) {
	cb := NewPrometheusCallback()
	assert.NotNil(t, cb)

	// Should not panic
	cb.LogSuccess(LogData{
		Model:            "gpt-4o",
		Provider:         "openai",
		Latency:          200 * time.Millisecond,
		LLMAPILatency:    150 * time.Millisecond,
		PromptTokens:     100,
		CompletionTokens: 50,
		Cost:             0.001,
	})
}

func TestPrometheusCallback_LogFailure(t *testing.T) {
	cb := NewPrometheusCallback()

	cb.LogFailure(LogData{
		Model:    "gpt-4o",
		Provider: "openai",
		Latency:  500 * time.Millisecond,
		Error:    assert.AnError,
	})
}

func TestPrometheusHandler(t *testing.T) {
	h := Handler()
	assert.NotNil(t, h)
}
