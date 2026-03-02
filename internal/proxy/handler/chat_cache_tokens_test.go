package handler

import (
	"context"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/stretchr/testify/assert"
)

// TestLogSuccess_CacheTokensPassedToCallback verifies that when a non-streaming
// response contains cache tokens, the LogData callback receives non-zero
// CacheReadInputTokens and CacheCreationInputTokens.
//
func TestLogSuccess_CacheTokensPassedToCallback(t *testing.T) {
	t.Parallel()

	cap := newLogCapture()
	reg := callback.NewRegistry()
	reg.Register(cap)

	h := &Handlers{
		Config:    &config.ProxyConfig{},
		Callbacks: reg,
	}

	req := &model.ChatCompletionRequest{Model: "anthropic/claude-sonnet-4-5-20250929"}
	result := &model.ModelResponse{
		Usage: model.Usage{
			PromptTokens:     1000,
			CompletionTokens: 200,
			TotalTokens:      1200,
			// These fields don't exist yet on model.Usage — test won't compile until fixed
			CacheReadInputTokens:     800,
			CacheCreationInputTokens: 150,
		},
	}

	start := time.Now()
	end := start.Add(100 * time.Millisecond)
	h.logSuccess(context.Background(), req, result, nil, start, end, 50*time.Millisecond)

	data := cap.wait(t, 2*time.Second)
	assert.Equal(t, 800, data.CacheReadInputTokens, "CacheReadInputTokens should be passed to callback")
	assert.Equal(t, 150, data.CacheCreationInputTokens, "CacheCreationInputTokens should be passed to callback")
}

// TestLogStreamSuccess_CacheTokensPassedToCallback verifies the streaming path.
func TestLogStreamSuccess_CacheTokensPassedToCallback(t *testing.T) {
	t.Parallel()

	cap := newLogCapture()
	reg := callback.NewRegistry()
	reg.Register(cap)

	h := &Handlers{
		Config:    &config.ProxyConfig{},
		Callbacks: reg,
	}

	req := &model.ChatCompletionRequest{Model: "anthropic/claude-sonnet-4-5-20250929"}
	lastChunk := &model.StreamChunk{Model: "claude-sonnet-4-5-20250929"}
	accUsage := model.Usage{
		PromptTokens:            1000,
		CompletionTokens:        200,
		CacheReadInputTokens:    800,
		CacheCreationInputTokens: 150,
	}

	start := time.Now()
	end := start.Add(100 * time.Millisecond)
	h.logStreamSuccess(context.Background(), req, lastChunk, accUsage, nil, start, end, 50*time.Millisecond)

	data := cap.wait(t, 2*time.Second)
	assert.Equal(t, 800, data.CacheReadInputTokens, "CacheReadInputTokens should be passed to callback")
	assert.Equal(t, 150, data.CacheCreationInputTokens, "CacheCreationInputTokens should be passed to callback")
}
