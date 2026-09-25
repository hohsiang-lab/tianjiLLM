package e2e

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

type codexProbeConfig struct {
	URL       string
	Token     string
	AccountID string
	Model     string
	Version   string
}

func TestCodexCapabilityProbe(t *testing.T) {
	cfg := codexProbeConfig{
		URL:       envOr("TIANJI_CODEX_PROBE_URL", "https://chatgpt.com/backend-api/codex/responses"),
		Token:     os.Getenv("TIANJI_CODEX_PROBE_TOKEN"),
		AccountID: os.Getenv("TIANJI_CODEX_PROBE_ACCOUNT_ID"),
		Model:     envOr("TIANJI_CODEX_PROBE_MODEL", "gpt-5.6-terra"),
		Version:   envOr("TIANJI_CODEX_PROBE_VERSION", "0.144.1"),
	}
	if cfg.Token == "" {
		t.Skip("set TIANJI_CODEX_PROBE_TOKEN to run the authenticated Codex capability probe")
	}

	t.Run("stream_only", func(t *testing.T) {
		payload := cfg.basePayload("Reply with exactly OK")
		payload["stream"] = false
		status, _, detail := cfg.request(t, payload)
		if status != http.StatusBadRequest || !strings.Contains(detail, "Stream must be set to true") {
			t.Fatalf("non-stream probe: status=%d detail=%q", status, detail)
		}
	})

	t.Run("json_object", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			payload := cfg.basePayload("Reply exactly FORMAT_PROBE_PLAIN_TEXT, not JSON.")
			payload["text"] = map[string]any{"format": map[string]any{"type": "json_object"}}
			status, events, detail := cfg.request(t, payload)
			if status != http.StatusOK {
				t.Fatalf("run %d: status=%d detail=%q", i+1, status, detail)
			}
			var result map[string]any
			if err := json.Unmarshal([]byte(outputText(events)), &result); err != nil || len(result) == 0 {
				t.Fatalf("run %d: invalid json_object output: %v", i+1, err)
			}
			assertCompletedFormat(t, events, "json_object")
			assertCompletedUsage(t, events)
		}
	})

	t.Run("json_schema", func(t *testing.T) {
		format := map[string]any{
			"type":   "json_schema",
			"name":   "entity",
			"strict": true,
			"schema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name":      map[string]any{"type": "string"},
					"pet_count": map[string]any{"type": "integer"},
				},
				"required":             []string{"name", "pet_count"},
				"additionalProperties": false,
			},
		}
		for i := 0; i < 3; i++ {
			payload := cfg.basePayload("Extract Alice owns 2 cats, but reply exactly as plain text: Alice has two cats. Do not use JSON.")
			payload["text"] = map[string]any{"format": format}
			status, events, detail := cfg.request(t, payload)
			if status != http.StatusOK {
				t.Fatalf("run %d: status=%d detail=%q", i+1, status, detail)
			}
			var result map[string]any
			err := json.Unmarshal([]byte(outputText(events)), &result)
			if err != nil || len(result) != 2 ||
				result["name"] != "Alice" || result["pet_count"] != float64(2) {
				t.Fatalf("run %d: schema output mismatch: %#v err=%v", i+1, result, err)
			}
			assertCompletedFormat(t, events, "json_schema")
			assertCompletedUsage(t, events)
		}
	})

	t.Run("tools_and_tool_choice", func(t *testing.T) {
		tests := []struct {
			choice   string
			prompt   string
			wantCall bool
		}{
			{choice: "auto", prompt: "Reply exactly NO_TOOL. Do not call any tool."},
			{choice: "required", prompt: "Reply exactly NO_TOOL. Do not call any tool.", wantCall: true},
			{choice: "none", prompt: "Call emit_probe now."},
		}
		for _, tt := range tests {
			t.Run(tt.choice, func(t *testing.T) {
				payload := cfg.basePayload(tt.prompt)
				payload["tools"] = []any{map[string]any{
					"type":        "function",
					"name":        "emit_probe",
					"description": "Emit a capability probe marker",
					"parameters": map[string]any{
						"type":                 "object",
						"properties":           map[string]any{},
						"additionalProperties": false,
					},
					"strict": true,
				}}
				payload["tool_choice"] = tt.choice
				status, events, detail := cfg.request(t, payload)
				if status != http.StatusOK {
					t.Fatalf("status=%d detail=%q", status, detail)
				}
				name, arguments := completedFunctionCall(events)
				if !tt.wantCall {
					if name != "" || strings.TrimSpace(outputText(events)) == "" {
						t.Fatalf("unexpected function call: name=%q arguments=%q", name, arguments)
					}
					assertCompletedUsage(t, events)
					return
				}
				if name != "emit_probe" {
					t.Fatalf("function name=%q", name)
				}
				deltaArguments, doneArguments := functionCallArgumentEvents(events)
				if deltaArguments == "" || doneArguments == "" || deltaArguments != doneArguments {
					t.Fatalf("argument events: delta=%q done=%q", deltaArguments, doneArguments)
				}
				var args map[string]any
				if err := json.Unmarshal([]byte(arguments), &args); err != nil || len(args) != 0 {
					t.Fatalf("function arguments=%q err=%v", arguments, err)
				}
				assertCompletedUsage(t, events)
			})
		}
	})

	for _, param := range []string{"max_output_tokens", "temperature", "top_p"} {
		t.Run("unsupported_"+param, func(t *testing.T) {
			payload := cfg.basePayload("Reply with exactly OK")
			switch param {
			case "max_output_tokens":
				payload[param] = 1
			case "temperature":
				payload[param] = 0.2
			case "top_p":
				payload[param] = 0.9
			}
			status, _, detail := cfg.request(t, payload)
			if status != http.StatusBadRequest || !strings.Contains(detail, "Unsupported parameter: "+param) {
				t.Fatalf("%s: status=%d detail=%q", param, status, detail)
			}
		})
	}
}

func (c codexProbeConfig) basePayload(prompt string) map[string]any {
	return map[string]any{
		"model":        c.Model,
		"store":        false,
		"stream":       true,
		"instructions": "Return only the requested result.",
		"input": []any{map[string]any{
			"type": "message",
			"role": "user",
			"content": []any{map[string]any{
				"type": "input_text",
				"text": prompt,
			}},
		}},
	}
}

func (c codexProbeConfig) request(t *testing.T, payload map[string]any) (int, []map[string]any, string) {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	stream, _ := payload["stream"].(bool)
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	} else {
		req.Header.Set("Accept", "application/json")
	}
	req.Header.Set("OpenAI-Beta", "responses=experimental")
	req.Header.Set("originator", "codex_cli_rs")
	req.Header.Set("Version", c.Version)
	if c.AccountID != "" {
		req.Header.Set("ChatGPT-Account-Id", c.AccountID)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			t.Fatal(err)
		}
		var failure struct {
			Detail string `json:"detail"`
			Error  struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(data, &failure)
		if failure.Detail == "" {
			failure.Detail = failure.Error.Message
		}
		return resp.StatusCode, nil, failure.Detail
	}
	if !stream {
		var response map[string]any
		if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&response); err != nil {
			t.Fatalf("decode non-stream response: %v", err)
		}
		return resp.StatusCode, []map[string]any{{
			"type":     "response.completed",
			"response": response,
		}}, ""
	}

	var events []map[string]any
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 4<<20)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
		if data == "" || data == "[DONE]" {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			t.Fatalf("decode SSE event: %v", err)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if !hasEvent(events, "response.completed") {
		t.Fatalf("stream did not complete; event types=%v", eventTypes(events))
	}
	return resp.StatusCode, events, ""
}

func outputText(events []map[string]any) string {
	var text strings.Builder
	for _, event := range events {
		if event["type"] == "response.output_text.delta" {
			delta, _ := event["delta"].(string)
			text.WriteString(delta)
		}
	}
	return text.String()
}

func completedFunctionCall(events []map[string]any) (string, string) {
	for _, event := range events {
		if event["type"] != "response.output_item.done" {
			continue
		}
		item, _ := event["item"].(map[string]any)
		if item["type"] == "function_call" {
			name, _ := item["name"].(string)
			arguments, _ := item["arguments"].(string)
			return name, arguments
		}
	}
	return "", ""
}

func functionCallArgumentEvents(events []map[string]any) (string, string) {
	var delta strings.Builder
	var done string
	for _, event := range events {
		switch event["type"] {
		case "response.function_call_arguments.delta":
			part, _ := event["delta"].(string)
			delta.WriteString(part)
		case "response.function_call_arguments.done":
			done, _ = event["arguments"].(string)
		}
	}
	return delta.String(), done
}

func assertCompletedFormat(t *testing.T, events []map[string]any, want string) {
	t.Helper()
	response := completedResponse(t, events)
	text, _ := response["text"].(map[string]any)
	format, _ := text["format"].(map[string]any)
	if got, _ := format["type"].(string); got != want {
		t.Fatalf("completed response format=%q, want %q", got, want)
	}
}

func assertCompletedUsage(t *testing.T, events []map[string]any) {
	t.Helper()
	response := completedResponse(t, events)
	usage, _ := response["usage"].(map[string]any)
	input, _ := usage["input_tokens"].(float64)
	output, _ := usage["output_tokens"].(float64)
	total, _ := usage["total_tokens"].(float64)
	if input <= 0 || output <= 0 || total <= 0 || total != input+output {
		t.Fatalf("completed response usage=%#v", usage)
	}
}

func completedResponse(t *testing.T, events []map[string]any) map[string]any {
	t.Helper()
	for _, event := range events {
		if event["type"] == "response.completed" {
			response, _ := event["response"].(map[string]any)
			if response != nil {
				return response
			}
		}
	}
	t.Fatalf("missing response.completed payload; event types=%v", eventTypes(events))
	return nil
}

func hasEvent(events []map[string]any, want string) bool {
	for _, event := range events {
		if event["type"] == want {
			return true
		}
	}
	return false
}

func eventTypes(events []map[string]any) []string {
	types := make([]string, 0, len(events))
	for _, event := range events {
		types = append(types, fmt.Sprint(event["type"]))
	}
	return types
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
