package handler

import (
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestReasoningEffortFromPayload(t *testing.T) {
	tests := []struct {
		name    string
		payload map[string]any
		want    string
	}{
		{
			name: "responses reasoning object",
			payload: map[string]any{
				"reasoning": map[string]any{"effort": "low"},
			},
			want: "low",
		},
		{
			name:    "chat completion reasoning effort",
			payload: map[string]any{"reasoning_effort": "high"},
			want:    "high",
		},
		{
			name: "direct field wins",
			payload: map[string]any{
				"reasoning_effort": "xhigh",
				"reasoning":        map[string]any{"effort": "low"},
			},
			want: "xhigh",
		},
		{
			name: "empty direct field falls back to reasoning object",
			payload: map[string]any{
				"reasoning_effort": " ",
				"reasoning":        map[string]any{"effort": "max"},
			},
			want: "max",
		},
		{
			name:    "missing",
			payload: map[string]any{"reasoning": map[string]any{}},
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, reasoningEffortFromPayload(tt.payload))
		})
	}
}

func TestReasoningEffortFromChatRequest(t *testing.T) {
	req := &model.ChatCompletionRequest{
		ExtraParams: map[string]any{
			"reasoning": map[string]any{"effort": "medium"},
		},
	}
	assert.Equal(t, "medium", reasoningEffortFromChatRequest(req))
	assert.Empty(t, reasoningEffortFromChatRequest(nil))
}

func TestReasoningEffortFromExpandedWebSocketPayload(t *testing.T) {
	state := &responsesWebSocketState{}
	state.storeResponse("resp_previous", map[string]any{
		"model":     "gpt-5.6-sol",
		"reasoning": map[string]any{"effort": "xhigh"},
		"input":     []any{},
	})

	expanded, err := state.expandPayload(map[string]any{
		"model":                "gpt-5.6-sol",
		"previous_response_id": "resp_previous",
		"input":                []any{map[string]any{"role": "user", "content": "continue"}},
	})

	assert.NoError(t, err)
	assert.Equal(t, "xhigh", reasoningEffortFromPayload(expanded))
}
