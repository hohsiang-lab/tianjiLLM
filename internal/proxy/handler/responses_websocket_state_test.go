package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponsesWebSocketStateStoreResponseOwnsDeepSnapshot(t *testing.T) {
	state := &responsesWebSocketState{}
	payload := map[string]any{
		"model": "openai/gpt-5.6-terra",
		"input": []any{
			map[string]any{
				"type": "message",
				"role": "user",
				"content": []any{
					map[string]any{"type": "input_text", "text": "original"},
				},
			},
		},
		"metadata": map[string]any{
			"session": map[string]any{"id": "session-original"},
		},
	}

	state.storeResponse("resp_snapshot", payload)

	input := requireResponsesTestSlice(t, payload["input"])
	message := requireResponsesTestMap(t, input[0])
	content := requireResponsesTestSlice(t, message["content"])
	requireResponsesTestMap(t, content[0])["text"] = "mutated"
	metadata := requireResponsesTestMap(t, payload["metadata"])
	session := requireResponsesTestMap(t, metadata["session"])
	session["id"] = "session-mutated"

	expanded, err := state.expandPayload(map[string]any{
		"previous_response_id": "resp_snapshot",
		"input":                []any{},
	})
	require.NoError(t, err)

	expandedInput := requireResponsesTestSlice(t, expanded["input"])
	expandedMessage := requireResponsesTestMap(t, expandedInput[0])
	expandedContent := requireResponsesTestSlice(t, expandedMessage["content"])
	assert.Equal(t, "original", requireResponsesTestMap(t, expandedContent[0])["text"])
	expandedMetadata := requireResponsesTestMap(t, expanded["metadata"])
	expandedSession := requireResponsesTestMap(t, expandedMetadata["session"])
	assert.Equal(t, "session-original", expandedSession["id"])
}

func TestResponsesWebSocketStateExpandPayloadReturnsOwnedCopy(t *testing.T) {
	state := &responsesWebSocketState{}
	state.storeResponse("resp_snapshot", map[string]any{
		"input": []any{
			map[string]any{
				"type": "reasoning",
				"id":   "rs_original",
				"summary": []any{
					map[string]any{"type": "summary_text", "text": "original"},
				},
			},
		},
	})

	first, err := state.expandPayload(map[string]any{
		"previous_response_id": "resp_snapshot",
		"input":                []any{},
	})
	require.NoError(t, err)
	firstInput := requireResponsesTestSlice(t, first["input"])
	firstReasoning := requireResponsesTestMap(t, firstInput[0])
	firstReasoning["id"] = "rs_mutated"
	firstSummary := requireResponsesTestSlice(t, firstReasoning["summary"])
	requireResponsesTestMap(t, firstSummary[0])["text"] = "mutated"

	second, err := state.expandPayload(map[string]any{
		"previous_response_id": "resp_snapshot",
		"input":                []any{},
	})
	require.NoError(t, err)
	secondInput := requireResponsesTestSlice(t, second["input"])
	secondReasoning := requireResponsesTestMap(t, secondInput[0])
	assert.Equal(t, "rs_original", secondReasoning["id"])
	secondSummary := requireResponsesTestSlice(t, secondReasoning["summary"])
	assert.Equal(t, "original", requireResponsesTestMap(t, secondSummary[0])["text"])
}

func requireResponsesTestMap(t *testing.T, value any) map[string]any {
	t.Helper()
	mapped, ok := value.(map[string]any)
	require.True(t, ok)
	return mapped
}

func requireResponsesTestSlice(t *testing.T, value any) []any {
	t.Helper()
	items, ok := value.([]any)
	require.True(t, ok)
	return items
}
