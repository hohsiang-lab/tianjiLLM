package chatgptcodex

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildResponsesRequest_NormalizesCodexBackendPayload(t *testing.T) {
	transport := Transport{BaseURL: "http://backend.example"}
	payload := map[string]any{
		"model": "openai/gpt-5.5",
		"input": "say OK",
	}

	req, err := buildResponsesTestRequest(transport, payload, true, "access-token", "acct-123")
	require.NoError(t, err)
	defer req.Body.Close()

	var got map[string]any
	require.NoError(t, json.NewDecoder(req.Body).Decode(&got))
	assert.Equal(t, "gpt-5.5", got["model"])
	assert.Equal(t, false, got["store"])
	assert.Equal(t, true, got["stream"])
	assert.NotContains(t, got, "instructions")
	input, ok := got["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 1)
	message, ok := input[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "message", message["type"])
	assert.Equal(t, "user", message["role"])
	content, ok := message["content"].([]any)
	require.True(t, ok)
	require.Len(t, content, 1)
	part, ok := content[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "input_text", part["type"])
	assert.Equal(t, "say OK", part["text"])
	assert.Equal(t, "Bearer access-token", req.Header.Get("Authorization"))
	assert.Equal(t, "acct-123", req.Header.Get("ChatGPT-Account-Id"))
	assert.Empty(t, req.Header.Get("Accept"))
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	assert.Empty(t, req.Header.Get("originator"))
	assert.Empty(t, req.Header.Get("OpenAI-Beta"))
	assert.Empty(t, req.Header.Get("Version"))
}

func TestBuildResponsesRequestDoesNotSynthesizeCodexTurnMetadataHeader(t *testing.T) {
	transport := Transport{BaseURL: "http://backend.example"}
	turnMetadata := `{"analytics_enabled":false,"request_kind":"turn","tool_namespaces_info":[{"namespace":"functions"}]}`
	payload := map[string]any{
		"model":           "openai/gpt-5.5",
		"input":           "say OK",
		"client_metadata": map[string]any{"x-codex-turn-metadata": turnMetadata},
	}

	builders := []struct {
		name  string
		build func() (*http.Request, error)
	}{
		{
			name: "responses",
			build: func() (*http.Request, error) {
				return buildResponsesTestRequest(transport, payload, true, "access-token", "acct-123")
			},
		},
		{
			name: "compact",
			build: func() (*http.Request, error) {
				prepared, err := transport.PrepareCompactResponsesPayload(payload)
				if err != nil {
					return nil, err
				}
				return transport.BuildCompactRequest(context.Background(), prepared, "access-token", "acct-123")
			},
		},
	}

	for _, builder := range builders {
		t.Run(builder.name, func(t *testing.T) {
			req, err := builder.build()
			require.NoError(t, err)
			defer req.Body.Close()

			assert.Empty(t, req.Header.Get("x-codex-turn-metadata"))
			var got map[string]any
			require.NoError(t, json.NewDecoder(req.Body).Decode(&got))
			clientMetadata, ok := got["client_metadata"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, turnMetadata, clientMetadata["x-codex-turn-metadata"])
		})
	}
}

func TestBuildResponsesRequest_OmitsEmptyAccountID(t *testing.T) {
	transport := Transport{BaseURL: "http://backend.example"}

	req, err := buildResponsesTestRequest(transport, map[string]any{
		"model": "openai/gpt-5.5",
		"input": "say OK",
	}, false, "access-token", "")
	require.NoError(t, err)
	assert.Empty(t, req.Header.Get("ChatGPT-Account-Id"))
}

func TestBuildResponsesRequest_StripsResponseItemIDsWhenStoreFalse(t *testing.T) {
	transport := Transport{BaseURL: "http://backend.example"}
	payload := map[string]any{
		"model": "openai/gpt-5.6-terra",
		"store": false,
		"input": []any{
			map[string]any{
				"type": "message",
				"id":   "msg_user",
				"role": "user",
				"content": []any{
					map[string]any{"type": "input_text", "text": "continue"},
				},
			},
			map[string]any{
				"type":              "reasoning",
				"id":                "rs_test",
				"summary":           []any{map[string]any{"type": "summary_text", "text": "condensed reasoning"}},
				"encrypted_content": "encrypted-reasoning",
				"future_field":      map[string]any{"version": 2},
			},
			map[string]any{
				"type":      "function_call",
				"id":        "fc_test",
				"call_id":   "call_test",
				"name":      "shell",
				"arguments": "{}",
			},
			map[string]any{
				"type":    "function_call_output",
				"id":      "fco_test",
				"call_id": "call_test",
				"output":  "done",
			},
		},
	}

	req, err := buildResponsesTestRequest(transport, payload, true, "access-token", "acct-123")
	require.NoError(t, err)
	defer req.Body.Close()

	var got map[string]any
	require.NoError(t, json.NewDecoder(req.Body).Decode(&got))
	assert.Equal(t, false, got["store"])

	input, ok := got["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 4)
	for _, item := range input {
		itemMap, itemOK := item.(map[string]any)
		require.True(t, itemOK)
		assert.NotContains(t, itemMap, "id")
	}

	reasoning, ok := input[1].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "reasoning", reasoning["type"])
	assert.Equal(t, "encrypted-reasoning", reasoning["encrypted_content"])
	summary, ok := reasoning["summary"].([]any)
	require.True(t, ok)
	require.Len(t, summary, 1)
	summaryItem, ok := summary[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "condensed reasoning", summaryItem["text"])
	futureField, ok := reasoning["future_field"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(2), futureField["version"])

	functionCall, ok := input[2].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "call_test", functionCall["call_id"])
	functionCallOutput, ok := input[3].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "call_test", functionCallOutput["call_id"])
}

func TestBuildResponsesRequest_PreservesResponseItemIDsWhenStoreTrue(t *testing.T) {
	transport := Transport{BaseURL: "http://backend.example"}
	payload := map[string]any{
		"model": "openai/gpt-5.6-terra",
		"store": true,
		"input": []any{
			map[string]any{
				"type":              "reasoning",
				"id":                "rs_test",
				"summary":           []any{},
				"encrypted_content": "encrypted-reasoning",
			},
		},
	}

	req, err := buildResponsesTestRequest(transport, payload, true, "access-token", "acct-123")
	require.NoError(t, err)
	defer req.Body.Close()

	var got map[string]any
	require.NoError(t, json.NewDecoder(req.Body).Decode(&got))
	assert.Equal(t, true, got["store"])

	input, ok := got["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 1)
	reasoning, ok := input[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "rs_test", reasoning["id"])
}

func TestPrepareResponsesPayloadOwnsCallerAndReturnedSnapshots(t *testing.T) {
	tags := []string{"original"}
	payload := map[string]any{
		"model": "openai/gpt-5.6-terra",
		"input": []any{
			map[string]any{
				"type":              "reasoning",
				"id":                "rs_caller",
				"summary":           []any{},
				"encrypted_content": "encrypted-reasoning",
				"future_field":      map[string][]string{"tags": tags},
			},
		},
		"metadata": map[string]any{
			"session": map[string]any{"id": "session-original"},
		},
	}

	prepared, err := PrepareResponsesPayload(payload, true)
	require.NoError(t, err)

	callerInput, ok := payload["input"].([]any)
	require.True(t, ok)
	callerReasoning, ok := callerInput[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "rs_caller", callerReasoning["id"])
	callerReasoning["id"] = "rs_mutated"
	callerMetadata, ok := payload["metadata"].(map[string]any)
	require.True(t, ok)
	callerSession, ok := callerMetadata["session"].(map[string]any)
	require.True(t, ok)
	callerSession["id"] = "session-mutated"
	tags[0] = "caller-mutated"

	firstValue, ok := prepared.Snapshot().(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, firstValue["store"])
	firstInput, ok := firstValue["input"].([]any)
	require.True(t, ok)
	firstReasoning, ok := firstInput[0].(map[string]any)
	require.True(t, ok)
	assert.NotContains(t, firstReasoning, "id")
	assert.Equal(t, "encrypted-reasoning", firstReasoning["encrypted_content"])
	firstMetadata, ok := firstValue["metadata"].(map[string]any)
	require.True(t, ok)
	firstSession, ok := firstMetadata["session"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "session-original", firstSession["id"])
	firstSession["id"] = "session-value-mutated"
	firstFuture, ok := firstReasoning["future_field"].(map[string]any)
	require.True(t, ok)
	firstTags, ok := firstFuture["tags"].([]any)
	require.True(t, ok)
	assert.Equal(t, []any{"original"}, firstTags)
	firstTags[0] = "snapshot-mutated"

	secondValue, ok := prepared.Snapshot().(map[string]any)
	require.True(t, ok)
	secondMetadata, ok := secondValue["metadata"].(map[string]any)
	require.True(t, ok)
	secondSession, ok := secondMetadata["session"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "session-original", secondSession["id"])
	secondInput, ok := secondValue["input"].([]any)
	require.True(t, ok)
	secondReasoning, ok := secondInput[0].(map[string]any)
	require.True(t, ok)
	secondFuture, ok := secondReasoning["future_field"].(map[string]any)
	require.True(t, ok)
	secondTags, ok := secondFuture["tags"].([]any)
	require.True(t, ok)
	assert.Equal(t, []any{"original"}, secondTags)
}

func TestBuildPreparedResponsesRequestReusesValidatedBodyAcrossAttempts(t *testing.T) {
	transport := Transport{BaseURL: "http://backend.example"}
	prepared, err := PrepareResponsesPayload(map[string]any{
		"model": "openai/gpt-5.6-terra",
		"input": "say OK",
	}, true)
	require.NoError(t, err)

	first, err := transport.BuildPreparedResponsesRequest(context.Background(), prepared, "access-a", "acct-a")
	require.NoError(t, err)
	defer first.Body.Close()
	second, err := transport.BuildPreparedResponsesRequest(context.Background(), prepared, "access-b", "acct-b")
	require.NoError(t, err)
	defer second.Body.Close()

	firstBody, err := io.ReadAll(first.Body)
	require.NoError(t, err)
	secondBody, err := io.ReadAll(second.Body)
	require.NoError(t, err)
	assert.Equal(t, firstBody, secondBody)
	assert.Equal(t, "Bearer access-a", first.Header.Get("Authorization"))
	assert.Equal(t, "acct-a", first.Header.Get("ChatGPT-Account-Id"))
	assert.Equal(t, "Bearer access-b", second.Header.Get("Authorization"))
	assert.Equal(t, "acct-b", second.Header.Get("ChatGPT-Account-Id"))
}

func TestBuildPreparedResponsesRequestReusesResponsesLiteContractAcrossAttempts(t *testing.T) {
	transport := Transport{
		BaseURL:          "http://backend.example",
		UseResponsesLite: true,
	}
	prepared, err := PrepareResponsesPayload(map[string]any{
		"model": "gpt-5.6-luna",
		"input": "say OK",
	}, true)
	require.NoError(t, err)

	first, err := transport.BuildPreparedResponsesRequest(context.Background(), prepared, "access-a", "acct-a")
	require.NoError(t, err)
	defer first.Body.Close()
	second, err := transport.BuildPreparedResponsesRequest(context.Background(), prepared, "access-b", "acct-b")
	require.NoError(t, err)
	defer second.Body.Close()

	firstBody, err := io.ReadAll(first.Body)
	require.NoError(t, err)
	secondBody, err := io.ReadAll(second.Body)
	require.NoError(t, err)
	assert.Equal(t, firstBody, secondBody)

	var got map[string]any
	require.NoError(t, json.Unmarshal(firstBody, &got))
	assert.Equal(t, false, got["parallel_tool_calls"])
}

func TestResponsesLiteRequestUsesAdditionalToolsContract(t *testing.T) {
	transport := Transport{
		BaseURL:          "http://backend.example",
		UseResponsesLite: true,
	}
	payload := map[string]any{
		"model":        "gpt-5.6-luna",
		"instructions": "Follow the caller instructions.",
		"input": []any{
			map[string]any{
				"type": "message",
				"role": "user",
				"content": []any{
					map[string]any{"type": "input_text", "text": "say OK"},
				},
			},
		},
		"tools": []any{
			map[string]any{
				"type":        "function",
				"name":        "lookup",
				"description": "Look up an order",
				"parameters":  map[string]any{"type": "object"},
			},
			map[string]any{
				"type":        "namespace",
				"name":        "editor",
				"description": "Editing tools",
				"tools": []any{
					map[string]any{"type": "function", "name": "edit"},
				},
			},
			map[string]any{
				"type":        "custom",
				"name":        "exec",
				"description": "Run code",
				"format": map[string]any{
					"type":       "grammar",
					"syntax":     "lark",
					"definition": "start: /.+/",
				},
			},
			map[string]any{
				"type":        "namespace",
				"name":        "functions",
				"description": "Existing default tools",
				"tools": []any{
					map[string]any{"type": "function", "name": "existing"},
					map[string]any{"type": "web_search"},
				},
			},
			map[string]any{
				"type":                "web_search",
				"external_web_access": true,
			},
			map[string]any{
				"type":        "tool_search",
				"execution":   "server",
				"description": "Server-side search",
			},
			map[string]any{
				"type":        "tool_search",
				"execution":   "client",
				"description": "Client-side search",
				"parameters":  map[string]any{"type": "object"},
			},
		},
		"tool_choice":         "required",
		"parallel_tool_calls": true,
		"reasoning": map[string]any{
			"context": "current_turn",
			"effort":  "medium",
		},
		"client_metadata": map[string]any{"session_id": "session-123"},
	}

	prepared, err := transport.PrepareResponsesPayload(payload, true)
	require.NoError(t, err)

	got, ok := prepared.Snapshot().(map[string]any)
	require.True(t, ok)
	assert.NotContains(t, got, "instructions")
	assert.NotContains(t, got, "tools")
	assert.Equal(t, "auto", got["tool_choice"])
	assert.Equal(t, false, got["parallel_tool_calls"])
	assert.Equal(t, map[string]any{
		"context": "all_turns",
		"effort":  "medium",
	}, got["reasoning"])
	assert.Equal(t, payload["client_metadata"], got["client_metadata"])

	input, ok := got["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 3)
	assert.Equal(t, map[string]any{
		"type": "additional_tools",
		"role": "developer",
		"tools": []any{
			map[string]any{
				"type":        "namespace",
				"name":        "functions",
				"description": "Existing default tools",
				"tools": []any{
					map[string]any{"type": "function", "name": "lookup", "description": "Look up an order", "parameters": map[string]any{"type": "object"}},
					map[string]any{"type": "custom", "name": "exec", "description": "Run code", "format": map[string]any{"type": "grammar", "syntax": "lark", "definition": "start: /.+/"}},
					map[string]any{"type": "function", "name": "existing"},
				},
			},
			map[string]any{
				"type":        "namespace",
				"name":        "editor",
				"description": "Editing tools",
				"tools": []any{
					map[string]any{"type": "function", "name": "edit"},
				},
			},
			map[string]any{
				"type":        "tool_search",
				"execution":   "client",
				"description": "Client-side search",
				"parameters":  map[string]any{"type": "object"},
			},
		},
	}, input[0])
	assert.Equal(t, map[string]any{
		"type": "message",
		"role": "developer",
		"content": []any{
			map[string]any{
				"type": "input_text",
				"text": "Follow the caller instructions.",
			},
		},
	}, input[1])
	lastItem, ok := input[2].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "message", lastItem["type"])
}

func TestBuildCompactRequestUsesResponsesLiteContract(t *testing.T) {
	transport := Transport{
		BaseURL:          "http://backend.example",
		UseResponsesLite: true,
	}
	prepared, err := transport.PrepareCompactResponsesPayload(map[string]any{
		"model":        "gpt-5.6-luna",
		"instructions": "Compact with these instructions.",
		"input":        []any{map[string]any{"type": "message", "role": "user", "content": "compact this"}},
		"tools":        []any{map[string]any{"type": "function", "name": "lookup"}},
		"store":        true,
		"stream":       true,
	})
	require.NoError(t, err)

	req, err := transport.BuildCompactRequest(context.Background(), prepared, "access-token", "acct-123")
	require.NoError(t, err)
	defer req.Body.Close()

	var got map[string]any
	require.NoError(t, json.NewDecoder(req.Body).Decode(&got))
	assert.NotContains(t, got, "instructions")
	assert.NotContains(t, got, "tools")
	assert.NotContains(t, got, "store")
	assert.NotContains(t, got, "stream")
	assert.Equal(t, "auto", got["tool_choice"])
	assert.Equal(t, false, got["parallel_tool_calls"])
	input, ok := got["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 3)
	firstItem, ok := input[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "additional_tools", firstItem["type"])
	instructionsItem, ok := input[1].(map[string]any)
	require.True(t, ok)
	instructionsContent, ok := instructionsItem["content"].([]any)
	require.True(t, ok)
	require.Len(t, instructionsContent, 1)
	instructionsText, ok := instructionsContent[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Compact with these instructions.", instructionsText["text"])
	lastItem, ok := input[2].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "message", lastItem["type"])
}

func TestNonResponsesLiteRequestPreservesToolsAndReasoning(t *testing.T) {
	transport := Transport{BaseURL: "http://backend.example"}
	payload := map[string]any{
		"model":        "gpt-5.6-luna",
		"instructions": "Keep this instruction field.",
		"input":        []any{map[string]any{"type": "message", "role": "user", "content": "say OK"}},
		"tools":        []any{map[string]any{"type": "function", "name": "lookup"}},
		"tool_choice":  "required",
		"reasoning":    map[string]any{"context": "current_turn", "effort": "medium"},
	}

	prepared, err := transport.PrepareResponsesPayload(payload, true)
	require.NoError(t, err)
	got, ok := prepared.Snapshot().(map[string]any)
	require.True(t, ok)
	assert.Equal(t, payload["instructions"], got["instructions"])
	assert.Equal(t, payload["tools"], got["tools"])
	assert.Equal(t, payload["tool_choice"], got["tool_choice"])
	assert.Equal(t, payload["reasoning"], got["reasoning"])
}

func TestPrepareResponsesPayload_PreservesCallerInstructions(t *testing.T) {
	for _, tt := range []struct {
		name         string
		instructions any
		present      bool
	}{
		{name: "omitted"},
		{name: "empty", instructions: "", present: true},
		{name: "whitespace", instructions: " 	\n ", present: true},
		{name: "null", present: true},
		{name: "custom", instructions: " Follow the caller instructions.\n", present: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			transport := Transport{BaseURL: "http://backend.example"}
			payload := map[string]any{
				"model": "openai/gpt-5.5",
				"input": []any{map[string]any{"role": "user", "content": "say OK"}},
			}
			if tt.present {
				payload["instructions"] = tt.instructions
			}
			before, err := json.Marshal(payload)
			require.NoError(t, err)
			prepared, err := PrepareResponsesPayload(payload, false)
			require.NoError(t, err)
			snapshot, ok := prepared.Snapshot().(map[string]any)
			require.True(t, ok)
			instructions, present := snapshot["instructions"]
			assert.Equal(t, tt.present, present)
			assert.Equal(t, tt.instructions, instructions)
			snapshot["instructions"] = "snapshot mutation"

			for _, accountID := range []string{"acct-a", "acct-b"} {
				req, err := transport.BuildPreparedResponsesRequest(context.Background(), prepared, "access-token", accountID)
				require.NoError(t, err)
				defer req.Body.Close()
				var got map[string]any
				require.NoError(t, json.NewDecoder(req.Body).Decode(&got))
				instructions, present := got["instructions"]
				assert.Equal(t, tt.present, present)
				assert.Equal(t, tt.instructions, instructions)
				after, err := json.Marshal(payload)
				require.NoError(t, err)
				assert.JSONEq(t, string(before), string(after))
			}
		})
	}
}

func TestPrepareResponsesPayload_PreservesNativeOptions(t *testing.T) {
	for _, tt := range []struct {
		name                                    string
		temperature, maxTokens, maxOutputTokens any
	}{
		{name: "zero", temperature: float64(0), maxTokens: float64(0), maxOutputTokens: float64(0)},
		{name: "null"},
		{name: "normal", temperature: 0.2, maxTokens: float64(2048), maxOutputTokens: float64(1024)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			transport := Transport{BaseURL: "http://backend.example"}
			payload := map[string]any{
				"model":             "openai/gpt-5.5",
				"input":             "summarize this",
				"max_output_tokens": tt.maxOutputTokens,
				"max_tokens":        tt.maxTokens,
				"temperature":       tt.temperature,
				"future_option":     map[string]any{"enabled": true},
			}
			before, err := json.Marshal(payload)
			require.NoError(t, err)
			prepared, err := PrepareResponsesPayload(payload, true)
			require.NoError(t, err)
			snapshot, ok := prepared.Snapshot().(map[string]any)
			require.True(t, ok)
			for _, key := range []string{"temperature", "max_tokens", "max_output_tokens", "future_option"} {
				value, present := snapshot[key]
				assert.True(t, present, key)
				assert.Equal(t, payload[key], value, key)
				snapshot[key] = "snapshot mutation"
			}

			for _, accountID := range []string{"acct-a", "acct-b"} {
				req, err := transport.BuildPreparedResponsesRequest(context.Background(), prepared, "access-token", accountID)
				require.NoError(t, err)
				defer req.Body.Close()
				var got map[string]any
				require.NoError(t, json.NewDecoder(req.Body).Decode(&got))
				for _, key := range []string{"temperature", "max_tokens", "max_output_tokens", "future_option"} {
					value, present := got[key]
					assert.True(t, present, key)
					assert.Equal(t, payload[key], value, key)
				}
				after, err := json.Marshal(payload)
				require.NoError(t, err)
				assert.JSONEq(t, string(before), string(after))
			}
		})
	}
}

func TestBuildCompactRequest_UsesResponsesCompactURLAndHeaders(t *testing.T) {
	transport := Transport{
		BaseURL: "http://backend.example",
	}
	payload := map[string]any{
		"model":  "openai/gpt-5.5",
		"input":  "summarize this context",
		"stream": true,
	}

	prepared, err := PrepareCompactResponsesPayload(payload)
	require.NoError(t, err)
	clientHeaders := make(http.Header)
	clientHeaders.Set("Version", "0.156.1-client")
	clientHeaders.Set("OpenAI-Beta", "client-beta")
	clientHeaders.Set("originator", "client-originator")
	clientHeaders.Set("Authorization", "Bearer client-token")
	clientHeaders.Set("ChatGPT-Account-Id", "client-account")
	clientHeaders.Set("x-codex-turn-state", "turn-state-client")
	clientHeaders.Set("x-codex-turn-metadata", `{"source":"client-header"}`)
	clientHeaders.Set("x-codex-routing-hint", "model=client-model;tier=client-tier")
	clientHeaders.Set("Accept", "application/json, application/vnd.example+json")
	clientHeaders.Set("Content-Type", "application/json; charset=utf-8")
	req, err := transport.BuildCompactRequest(context.Background(), prepared, "access-token", "acct-123", clientHeaders)
	require.NoError(t, err)
	defer req.Body.Close()

	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "http://backend.example/backend-api/codex/responses/compact", req.URL.String())
	assert.Equal(t, "Bearer access-token", req.Header.Get("Authorization"))
	assert.Equal(t, "acct-123", req.Header.Get("ChatGPT-Account-Id"))
	assert.Equal(t, "client-beta", req.Header.Get("OpenAI-Beta"))
	assert.Equal(t, "client-originator", req.Header.Get("originator"))
	assert.Equal(t, "application/json, application/vnd.example+json", req.Header.Get("Accept"))
	assert.Equal(t, "application/json; charset=utf-8", req.Header.Get("Content-Type"))
	assert.Equal(t, "0.156.1-client", req.Header.Get("Version"))
	assert.Equal(t, "turn-state-client", req.Header.Get("x-codex-turn-state"))
	assert.Equal(t, `{"source":"client-header"}`, req.Header.Get("x-codex-turn-metadata"))
	assert.Equal(t, "model=client-model;tier=client-tier", req.Header.Get("x-codex-routing-hint"))

	var got map[string]any
	require.NoError(t, json.NewDecoder(req.Body).Decode(&got))
	assert.Equal(t, "gpt-5.5", got["model"])
	assert.NotContains(t, got, "store")
	assert.NotContains(t, got, "stream")
	assert.NotContains(t, got, "instructions")
}

func TestBuildPreparedResponsesRequestDoesNotSynthesizeVersionHeader(t *testing.T) {
	transport := Transport{
		BaseURL: "http://backend.example",
	}
	payload := map[string]any{
		"model": "openai/gpt-5.5",
		"input": "say OK",
	}

	req, err := buildResponsesTestRequest(transport, payload, true, "access-token", "acct-123")
	require.NoError(t, err)
	defer req.Body.Close()

	assert.Empty(t, req.Header.Get("Version"))
}

func TestBuildPreparedResponsesRequestDoesNotSynthesizeRoutingHint(t *testing.T) {
	transport := Transport{
		BaseURL:          "http://backend.example",
		UseResponsesLite: true,
	}
	prepared, err := PrepareResponsesPayload(map[string]any{
		"model":        "openai/gpt-5.6-sol",
		"service_tier": "priority",
		"input":        "say OK",
	}, true)
	require.NoError(t, err)

	req, err := transport.BuildPreparedResponsesRequest(context.Background(), prepared, "access-token", "acct-123")
	require.NoError(t, err)
	defer req.Body.Close()

	assert.Equal(t, "true", req.Header.Get("x-openai-internal-codex-responses-lite"))
	assert.Empty(t, req.Header.Get("x-codex-routing-hint"))
}

func TestBuildPreparedResponsesRequestAddsMissingResponsesLiteReasoningContext(t *testing.T) {
	transport := Transport{
		BaseURL:          "http://backend.example",
		UseResponsesLite: true,
	}
	payload := map[string]any{
		"model": "gpt-5.6-luna",
		"input": "say OK",
	}

	prepared, err := transport.PrepareResponsesPayload(payload, true)
	require.NoError(t, err)
	req, err := transport.BuildPreparedResponsesRequest(context.Background(), prepared, "access-token", "acct-123")
	require.NoError(t, err)
	defer req.Body.Close()

	var got map[string]any
	require.NoError(t, json.NewDecoder(req.Body).Decode(&got))
	reasoning, ok := got["reasoning"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "all_turns", reasoning["context"])
}

func TestBuildPreparedResponsesRequestAddsMissingResponsesLiteReasoningContextForPlainPreparedPayload(t *testing.T) {
	transport := Transport{
		BaseURL:          "http://backend.example",
		UseResponsesLite: true,
	}
	prepared, err := PrepareResponsesPayload(map[string]any{
		"model": "gpt-5.6-luna",
		"input": "say OK",
	}, true)
	require.NoError(t, err)

	req, err := transport.BuildPreparedResponsesRequest(context.Background(), prepared, "access-token", "acct-123")
	require.NoError(t, err)
	defer req.Body.Close()

	var got map[string]any
	require.NoError(t, json.NewDecoder(req.Body).Decode(&got))
	reasoning, ok := got["reasoning"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "all_turns", reasoning["context"])
}

func TestBuildRequestAddsMissingResponsesLiteReasoningContext(t *testing.T) {
	transport := Transport{
		BaseURL:          "http://backend.example",
		UseResponsesLite: true,
	}
	req, err := transport.BuildRequest(context.Background(), &model.ChatCompletionRequest{
		Model: "gpt-5.6-luna",
		Messages: []model.Message{{
			Role:    "user",
			Content: "say OK",
		}},
		Stream: ptrBool(true),
	}, "access-token", "acct-123")
	require.NoError(t, err)
	defer req.Body.Close()

	var got map[string]any
	require.NoError(t, json.NewDecoder(req.Body).Decode(&got))
	reasoning, ok := got["reasoning"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "all_turns", reasoning["context"])
	assert.Equal(t, false, got["parallel_tool_calls"])
}

func TestBuildCompactRequestAddsMissingResponsesLiteReasoningContext(t *testing.T) {
	transport := Transport{
		BaseURL:          "http://backend.example",
		UseResponsesLite: true,
	}
	prepared, err := transport.PrepareCompactResponsesPayload(map[string]any{
		"model": "gpt-5.6-luna",
		"input": "compact this",
	})
	require.NoError(t, err)

	req, err := transport.BuildCompactRequest(context.Background(), prepared, "access-token", "acct-123")
	require.NoError(t, err)
	defer req.Body.Close()

	var got map[string]any
	require.NoError(t, json.NewDecoder(req.Body).Decode(&got))
	reasoning, ok := got["reasoning"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "all_turns", reasoning["context"])
	assert.Equal(t, false, got["parallel_tool_calls"])
}

func TestBuildCompactRequestAddsResponsesLiteContractForPlainPreparedPayload(t *testing.T) {
	transport := Transport{
		BaseURL:          "http://backend.example",
		UseResponsesLite: true,
	}
	prepared, err := PrepareCompactResponsesPayload(map[string]any{
		"model": "gpt-5.6-luna",
		"input": "compact this",
	})
	require.NoError(t, err)

	req, err := transport.BuildCompactRequest(context.Background(), prepared, "access-token", "acct-123")
	require.NoError(t, err)
	defer req.Body.Close()

	var got map[string]any
	require.NoError(t, json.NewDecoder(req.Body).Decode(&got))
	reasoning, ok := got["reasoning"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "all_turns", reasoning["context"])
	assert.Equal(t, false, got["parallel_tool_calls"])
}

func TestResponsesLitePreservesCallerReasoningAndClientMetadata(t *testing.T) {
	transport := Transport{
		BaseURL:          "http://backend.example",
		UseResponsesLite: true,
	}
	payload := map[string]any{
		"model":               "gpt-5.6-luna",
		"input":               "say OK",
		"parallel_tool_calls": true,
		"reasoning": map[string]any{
			"context": "current_turn",
			"effort":  "medium",
		},
		"client_metadata": map[string]any{
			"ws_request_header_x_openai_internal_codex_responses_lite": "true",
			"session_id": "session-123",
		},
	}

	prepared, err := transport.PrepareResponsesPayload(payload, true)
	require.NoError(t, err)
	req, err := transport.BuildPreparedResponsesRequest(context.Background(), prepared, "access-token", "acct-123")
	require.NoError(t, err)
	defer req.Body.Close()

	var got map[string]any
	require.NoError(t, json.NewDecoder(req.Body).Decode(&got))
	assert.Equal(t, map[string]any{
		"context": "all_turns",
		"effort":  "medium",
	}, got["reasoning"])
	assert.Equal(t, payload["client_metadata"], got["client_metadata"])
	assert.Equal(t, false, got["parallel_tool_calls"])
}

func TestNormalizeResponsesLitePayloadPreservesRawFields(t *testing.T) {
	body := []byte(`{"model":"gpt-5.6-luna","reasoning":{"effort":"medium"},"client_metadata":{"large":9007199254740993}}`)

	got, err := normalizeResponsesLitePayload(body)
	require.NoError(t, err)

	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(got, &payload))
	assert.JSONEq(t, `{"effort":"medium","context":"all_turns"}`, string(payload["reasoning"]))
	assert.Equal(t, json.RawMessage(`{"large":9007199254740993}`), payload["client_metadata"])
}

func TestNormalizeResponsesLitePayloadForcesContextAndParallelSetting(t *testing.T) {
	body := []byte(`{ "reasoning": {"context":"current_turn","effort":"medium"}, "parallel_tool_calls": false, "client_metadata": {"large":9007199254740993} }`)

	got, err := normalizeResponsesLitePayload(body)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(got, &payload))
	reasoning, ok := payload["reasoning"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "all_turns", reasoning["context"])
	assert.Equal(t, false, payload["parallel_tool_calls"])
	clientMetadata, ok := payload["client_metadata"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(9007199254740993), clientMetadata["large"])
}

func TestNormalizeResponsesLitePayloadReplacesNullReasoningContext(t *testing.T) {
	body := []byte(`{"reasoning":{"context":null,"effort":"medium"}}`)

	got, err := normalizeResponsesLitePayload(body)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(got, &payload))
	reasoning, ok := payload["reasoning"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "all_turns", reasoning["context"])
	assert.Equal(t, false, payload["parallel_tool_calls"])
}

func TestResponsesLiteAddsMissingParallelToolCalls(t *testing.T) {
	transport := Transport{
		BaseURL:          "http://backend.example",
		UseResponsesLite: true,
	}

	prepared, err := transport.PrepareResponsesPayload(map[string]any{
		"model": "gpt-5.6-luna",
		"input": "say OK",
	}, true)
	require.NoError(t, err)

	got, ok := prepared.Snapshot().(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, got["parallel_tool_calls"])
}

func TestBuildImageGenerationRequestAddsMissingResponsesLiteReasoningContext(t *testing.T) {
	transport := Transport{
		BaseURL:          "http://backend.example",
		UseResponsesLite: true,
	}
	payload, err := BuildImageGenerationPayload(&model.ImageGenerationRequest{
		Model:  "gpt-5.6-luna",
		Prompt: "draw a circle",
	})
	require.NoError(t, err)
	prepared, err := transport.PrepareResponsesPayload(payload, true)
	require.NoError(t, err)

	req, err := transport.BuildImageGenerationRequest(context.Background(), prepared, "access-token", "acct-123")
	require.NoError(t, err)
	defer req.Body.Close()

	var got map[string]any
	require.NoError(t, json.NewDecoder(req.Body).Decode(&got))
	reasoning, ok := got["reasoning"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "all_turns", reasoning["context"])
	assert.Equal(t, false, got["parallel_tool_calls"])
}

func TestBuildImageGenerationRequestAddsResponsesLiteContractForPlainPreparedPayload(t *testing.T) {
	transport := Transport{
		BaseURL:          "http://backend.example",
		UseResponsesLite: true,
	}
	payload, err := BuildImageGenerationPayload(&model.ImageGenerationRequest{
		Model:  "gpt-5.6-luna",
		Prompt: "draw a circle",
	})
	require.NoError(t, err)
	prepared, err := PrepareResponsesPayload(payload, true)
	require.NoError(t, err)

	req, err := transport.BuildImageGenerationRequest(context.Background(), prepared, "access-token", "acct-123")
	require.NoError(t, err)
	defer req.Body.Close()

	var got map[string]any
	require.NoError(t, json.NewDecoder(req.Body).Decode(&got))
	reasoning, ok := got["reasoning"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "all_turns", reasoning["context"])
	assert.Equal(t, false, got["parallel_tool_calls"])
}

func TestNonResponsesLiteRequestDoesNotAddReasoningContext(t *testing.T) {
	transport := Transport{BaseURL: "http://backend.example"}
	req, err := buildResponsesTestRequest(transport, map[string]any{
		"model":               "gpt-5.6-luna",
		"input":               "say OK",
		"parallel_tool_calls": true,
	}, true, "access-token", "acct-123")
	require.NoError(t, err)
	defer req.Body.Close()

	var got map[string]any
	require.NoError(t, json.NewDecoder(req.Body).Decode(&got))
	assert.NotContains(t, got, "reasoning")
	assert.Equal(t, true, got["parallel_tool_calls"])
}

func TestBuildPreparedResponsesRequestOmitsRoutingTierWhenAbsent(t *testing.T) {
	transport := Transport{BaseURL: "http://backend.example"}
	prepared, err := PrepareResponsesPayload(map[string]any{
		"model": "gpt-5.6-sol",
		"input": "say OK",
	}, true)
	require.NoError(t, err)

	req, err := transport.BuildPreparedResponsesRequest(context.Background(), prepared, "access-token", "acct-123")
	require.NoError(t, err)
	defer req.Body.Close()

	assert.Empty(t, req.Header.Get("x-codex-routing-hint"))
}

func TestBuildCompactRequestReusesPreparedBodyAcrossAttempts(t *testing.T) {
	transport := Transport{BaseURL: "http://backend.example"}
	prepared, err := PrepareCompactResponsesPayload(map[string]any{
		"model":  "openai/gpt-5.6-terra",
		"input":  "compact this context",
		"stream": true,
	})
	require.NoError(t, err)

	first, err := transport.BuildCompactRequest(context.Background(), prepared, "access-a", "acct-a")
	require.NoError(t, err)
	defer first.Body.Close()
	second, err := transport.BuildCompactRequest(context.Background(), prepared, "access-b", "acct-b")
	require.NoError(t, err)
	defer second.Body.Close()

	firstBody, err := io.ReadAll(first.Body)
	require.NoError(t, err)
	secondBody, err := io.ReadAll(second.Body)
	require.NoError(t, err)
	assert.Equal(t, firstBody, secondBody)
	assert.Equal(t, "Bearer access-a", first.Header.Get("Authorization"))
	assert.Equal(t, "acct-a", first.Header.Get("ChatGPT-Account-Id"))
	assert.Equal(t, "Bearer access-b", second.Header.Get("Authorization"))
	assert.Equal(t, "acct-b", second.Header.Get("ChatGPT-Account-Id"))
}

func TestBuildCompactRequestRejectsUnpreparedPayload(t *testing.T) {
	transport := Transport{BaseURL: "http://backend.example"}

	_, err := transport.BuildCompactRequest(context.Background(), PreparedCompactResponsesPayload{}, "access-token", "acct-123")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "payload is not prepared")
}

func TestBuildImageGenerationRequest_MapsOpenAIImageOptions(t *testing.T) {
	transport := Transport{BaseURL: "http://backend.example"}
	size := "1024x1024"
	quality := "low"
	outputFormat := "jpeg"
	background := "opaque"
	user := "end-user-42"
	outputCompression := 60

	prepared, err := PrepareImageGenerationPayload(&model.ImageGenerationRequest{
		Model:             "openai/gpt-image-2",
		Prompt:            "draw a cat",
		Size:              &size,
		Quality:           &quality,
		OutputFormat:      &outputFormat,
		Background:        &background,
		OutputCompression: &outputCompression,
		User:              &user,
	})
	require.NoError(t, err)
	req, err := transport.BuildImageGenerationRequest(context.Background(), prepared, "access-token", "acct-123")
	require.NoError(t, err)
	defer req.Body.Close()

	var got map[string]any
	require.NoError(t, json.NewDecoder(req.Body).Decode(&got))
	input, ok := got["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 1)
	message, ok := input[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "message", message["type"])

	tools, ok := got["tools"].([]any)
	require.True(t, ok)
	require.Len(t, tools, 1)
	tool, ok := tools[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "image_generation", tool["type"])
	assert.Equal(t, "gpt-image-2", tool["model"])
	assert.Equal(t, "1024x1024", tool["size"])
	assert.Equal(t, "low", tool["quality"])
	assert.Equal(t, "jpeg", tool["output_format"])
	assert.Equal(t, "opaque", tool["background"])
	assert.Equal(t, float64(60), tool["output_compression"])
	assert.NotContains(t, tool, "moderation")
	assert.NotContains(t, tool, "user")
}

func TestBuildImageGenerationRequest_RejectsUnsupportedModeration(t *testing.T) {
	moderation := "low"

	_, err := PrepareImageGenerationPayload(&model.ImageGenerationRequest{
		Model:      "openai/gpt-image-2",
		Prompt:     "draw a cat",
		Moderation: &moderation,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "moderation")
}

func TestBuildImageEditPayload_MapsMultipartImageToCodexPayload(t *testing.T) {
	transport := Transport{BaseURL: "http://backend.example"}
	size := "1024x1024"
	quality := "low"
	outputFormat := "png"
	responseFormat := "b64_json"

	payload, err := BuildImageEditPayload(&model.ImageEditRequest{
		Model:          "openai/gpt-image-2",
		Prompt:         "make the sky orange",
		Size:           &size,
		Quality:        &quality,
		OutputFormat:   &outputFormat,
		ResponseFormat: &responseFormat,
		Images: []model.ImageEditFile{{
			Filename:    "input.png",
			ContentType: "image/png",
			Data:        []byte("png-bytes"),
		}},
	})
	require.NoError(t, err)
	req, err := buildResponsesTestRequest(transport, payload, true, "access-token", "acct-123")
	require.NoError(t, err)
	defer req.Body.Close()

	var got map[string]any
	require.NoError(t, json.NewDecoder(req.Body).Decode(&got))
	input, ok := got["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 1)
	message, ok := input[0].(map[string]any)
	require.True(t, ok)
	content, ok := message["content"].([]any)
	require.True(t, ok)
	require.Len(t, content, 2)
	text, ok := content[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "input_text", text["type"])
	assert.Equal(t, "make the sky orange", text["text"])
	image, ok := content[1].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "input_image", image["type"])
	assert.Equal(t, "data:image/png;base64,cG5nLWJ5dGVz", image["image_url"])
	assert.NotContains(t, image, "url")

	tools, ok := got["tools"].([]any)
	require.True(t, ok)
	require.Len(t, tools, 1)
	tool, ok := tools[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "image_generation", tool["type"])
	assert.Equal(t, "gpt-image-2", tool["model"])
	assert.Equal(t, "1024x1024", tool["size"])
	assert.Equal(t, "low", tool["quality"])
	assert.Equal(t, "png", tool["output_format"])
	assert.Equal(t, "Bearer access-token", req.Header.Get("Authorization"))
	assert.Equal(t, "acct-123", req.Header.Get("ChatGPT-Account-Id"))
}

func TestBuildResponsesRequest_NormalizesOfficialImageURLString(t *testing.T) {
	transport := Transport{BaseURL: "http://backend.example"}
	payload := map[string]any{
		"model": "openai/gpt-image-2",
		"input": []any{map[string]any{
			"type": "message",
			"role": "user",
			"content": []any{
				map[string]any{"type": "input_text", "text": "edit this"},
				map[string]any{"type": "input_image", "image_url": "https://example.test/reference.png", "detail": "high"},
			},
		}},
	}

	req, err := buildResponsesTestRequest(transport, payload, true, "access-token", "acct-123")
	require.NoError(t, err)
	defer req.Body.Close()

	image := decodedResponsesImagePart(t, req.Body)
	assert.Equal(t, "input_image", image["type"])
	assert.Equal(t, "https://example.test/reference.png", image["image_url"])
	assert.Equal(t, "high", image["detail"])
	assert.NotContains(t, image, "url")
}

func TestBuildResponsesRequest_NormalizesLegacyImageURLObject(t *testing.T) {
	transport := Transport{BaseURL: "http://backend.example"}
	payload := map[string]any{
		"model": "openai/gpt-image-2",
		"input": []any{map[string]any{
			"type": "message",
			"role": "user",
			"content": []any{
				map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64,abc123", "detail": "low"}},
			},
		}},
	}

	req, err := buildResponsesTestRequest(transport, payload, true, "access-token", "acct-123")
	require.NoError(t, err)
	defer req.Body.Close()

	image := decodedResponsesImagePart(t, req.Body)
	assert.Equal(t, "input_image", image["type"])
	assert.Equal(t, "data:image/png;base64,abc123", image["image_url"])
	assert.Equal(t, "low", image["detail"])
}

func TestBuildResponsesRequest_NormalizesLegacyInputImageURLObject(t *testing.T) {
	transport := Transport{BaseURL: "http://backend.example"}
	payload := map[string]any{
		"model": "openai/gpt-image-2",
		"input": []any{map[string]any{
			"type": "message",
			"role": "user",
			"content": []any{
				map[string]any{"type": "input_image", "image_url": map[string]any{"url": "https://example.test/reference.png", "detail": "high"}},
			},
		}},
	}

	req, err := buildResponsesTestRequest(transport, payload, true, "access-token", "acct-123")
	require.NoError(t, err)
	defer req.Body.Close()

	image := decodedResponsesImagePart(t, req.Body)
	assert.Equal(t, "input_image", image["type"])
	assert.Equal(t, "https://example.test/reference.png", image["image_url"])
	assert.Equal(t, "high", image["detail"])
	assert.NotContains(t, image, "url")
}

func TestBuildResponsesRequest_PreservesInputImageFileID(t *testing.T) {
	transport := Transport{BaseURL: "http://backend.example"}
	payload := map[string]any{
		"model": "openai/gpt-image-2",
		"input": []any{map[string]any{
			"type": "message",
			"role": "user",
			"content": []any{
				map[string]any{"type": "input_image", "file_id": "file-reference-123", "detail": "auto"},
			},
		}},
	}

	req, err := buildResponsesTestRequest(transport, payload, true, "access-token", "acct-123")
	require.NoError(t, err)
	defer req.Body.Close()

	image := decodedResponsesImagePart(t, req.Body)
	assert.Equal(t, "input_image", image["type"])
	assert.Equal(t, "file-reference-123", image["file_id"])
	assert.Equal(t, "auto", image["detail"])
	assert.NotContains(t, image, "image_url")
}

func TestBuildResponsesRequest_PreservesOutputImageDataURL(t *testing.T) {
	transport := Transport{BaseURL: "http://backend.example"}
	payload := map[string]any{
		"model": "openai/gpt-5.5",
		"input": []any{map[string]any{
			"type": "message",
			"role": "assistant",
			"output": []any{
				map[string]any{"type": "output_text", "text": "created"},
				map[string]any{"type": "input_image", "image_url": "data:image/png;base64,aW1hZ2U=", "detail": "auto"},
			},
		}},
	}

	req, err := buildResponsesTestRequest(transport, payload, true, "access-token", "acct-123")
	require.NoError(t, err)
	defer req.Body.Close()

	output := decodedResponsesOutputItems(t, req.Body)
	text, ok := output[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "output_text", text["type"])
	assert.Equal(t, "created", text["text"])
	image, ok := output[1].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "input_image", image["type"])
	assert.Equal(t, "data:image/png;base64,aW1hZ2U=", image["image_url"])
	assert.Equal(t, "auto", image["detail"])
}

func TestBuildResponsesRequest_RejectsMalformedOutputImageURLAtObservedPath(t *testing.T) {
	transport := Transport{BaseURL: "http://backend.example"}
	input := make([]any, 64)
	for i := range input {
		input[i] = map[string]any{
			"type":    "message",
			"role":    "user",
			"content": []any{map[string]any{"type": "input_text", "text": "context"}},
		}
	}
	input[63] = map[string]any{
		"type": "message",
		"role": "assistant",
		"output": []any{
			map[string]any{"type": "output_text", "text": "created"},
			map[string]any{"type": "input_image", "image_url": "data:image/png,not-base64"},
		},
	}
	payload := map[string]any{
		"model": "openai/gpt-5.5",
		"input": input,
	}

	_, err := buildResponsesTestRequest(transport, payload, true, "access-token", "acct-123")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "input[63].output[1].image_url")
	assert.Contains(t, err.Error(), ";base64")
	assert.NotContains(t, err.Error(), "not-base64")
}

func TestBuildResponsesRequest_RejectsMalformedImageReferenceBeforeUpstream(t *testing.T) {
	transport := Transport{BaseURL: "http://backend.example"}
	payload := map[string]any{
		"model": "openai/gpt-image-2",
		"input": []any{map[string]any{
			"type": "message",
			"role": "user",
			"content": []any{
				map[string]any{"type": "input_image", "image_url": map[string]any{"detail": "high"}},
			},
		}},
	}

	_, err := buildResponsesTestRequest(transport, payload, true, "access-token", "acct-123")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "image content part url")
}

func TestBuildImageEditPayload_RejectsMask(t *testing.T) {
	mask := model.ImageEditFile{Filename: "mask.png", ContentType: "image/png", Data: []byte("mask")}

	_, err := BuildImageEditPayload(&model.ImageEditRequest{
		Model:  "openai/gpt-image-2",
		Prompt: "edit it",
		Images: []model.ImageEditFile{{
			Filename:    "input.png",
			ContentType: "image/png",
			Data:        []byte("png"),
		}},
		Mask: &mask,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "mask")
}

func decodedResponsesImagePart(t *testing.T, body io.Reader) map[string]any {
	t.Helper()
	var got map[string]any
	require.NoError(t, json.NewDecoder(body).Decode(&got))
	input, ok := got["input"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, input)
	message, ok := input[0].(map[string]any)
	require.True(t, ok)
	content, ok := message["content"].([]any)
	require.True(t, ok)
	for _, part := range content {
		item, ok := part.(map[string]any)
		require.True(t, ok)
		if item["type"] == "input_image" {
			return item
		}
	}
	t.Fatalf("input_image content part missing from %#v", content)
	return nil
}

func decodedResponsesOutputItems(t *testing.T, body io.Reader) []any {
	t.Helper()
	var got map[string]any
	require.NoError(t, json.NewDecoder(body).Decode(&got))
	input, ok := got["input"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, input)
	message, ok := input[0].(map[string]any)
	require.True(t, ok)
	output, ok := message["output"].([]any)
	require.True(t, ok)
	require.Len(t, output, 2)
	return output
}

func buildResponsesTestRequest(transport Transport, payload any, stream bool, bearerToken, accountID string) (*http.Request, error) {
	prepared, err := PrepareResponsesPayload(payload, stream)
	if err != nil {
		return nil, err
	}
	return transport.BuildPreparedResponsesRequest(context.Background(), prepared, bearerToken, accountID)
}
