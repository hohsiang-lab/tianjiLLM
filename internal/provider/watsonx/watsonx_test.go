package watsonx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

func TestNew(t *testing.T) {
	p := New("", "proj1", "key1")
	if p.baseURL != defaultBaseURL {
		t.Fatalf("got %q, want %q", p.baseURL, defaultBaseURL)
	}
	if p.projectID != "proj1" {
		t.Fatalf("got %q", p.projectID)
	}
	if p.apiVersion != defaultVersion {
		t.Fatalf("got %q", p.apiVersion)
	}

	p2 := New("https://custom.url", "p", "k")
	if p2.baseURL != "https://custom.url" {
		t.Fatalf("got %q", p2.baseURL)
	}
}

func TestGetSupportedParams(t *testing.T) {
	p := New("", "", "")
	params := p.GetSupportedParams()
	if len(params) == 0 {
		t.Fatal("expected non-empty params")
	}
}

func TestMapParams(t *testing.T) {
	p := New("", "", "")
	result := p.MapParams(map[string]any{
		"stop":        []string{"END"},
		"temperature": 0.5,
	})
	if _, ok := result["stop_sequences"]; !ok {
		t.Fatal("expected stop_sequences key")
	}
	if _, ok := result["stop"]; ok {
		t.Fatal("stop key should be renamed")
	}
	if result["temperature"] != 0.5 {
		t.Fatalf("temperature: got %v", result["temperature"])
	}
}

func TestGetRequestURL(t *testing.T) {
	p := New("https://example.com", "", "")
	u := p.GetRequestURL("gpt-4")
	expected := "https://example.com/ml/v1/text/chat?version=" + defaultVersion
	if u != expected {
		t.Fatalf("got %q, want %q", u, expected)
	}
}

func TestSetupHeaders(t *testing.T) {
	p := New("", "", "")
	req, _ := http.NewRequest("POST", "http://example.com", nil)
	p.SetupHeaders(req, "key")
	if req.Header.Get("Content-Type") != "application/json" {
		t.Fatal("missing content-type")
	}
}

func TestTransformResponse(t *testing.T) {
	wxResp := watsonxResponse{
		ModelID: "granite-13b",
		Choices: []watsonxChoice{
			{Index: 0, Message: watsonxMsg{Role: "assistant", Content: "hello"}, FinishReason: "stop"},
		},
		Usage: &watsonxUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
	}
	body, _ := json.Marshal(wxResp)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	p := New("", "", "")
	result, err := p.TransformResponse(context.Background(), resp)
	if err != nil {
		t.Fatal(err)
	}
	if result.Model != "granite-13b" {
		t.Fatalf("model: got %q", result.Model)
	}
	if len(result.Choices) != 1 {
		t.Fatalf("choices: got %d", len(result.Choices))
	}
	if result.Choices[0].Message.Content != "hello" {
		t.Fatalf("content: got %v", result.Choices[0].Message.Content)
	}
	if result.Usage.PromptTokens != 10 {
		t.Fatalf("prompt_tokens: got %d", result.Usage.PromptTokens)
	}
}

func TestTransformResponseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	p := New("", "", "")
	_, err = p.TransformResponse(context.Background(), resp)
	if err == nil {
		t.Fatal("expected error for 401")
	}
}

func TestTransformStreamChunk(t *testing.T) {
	p := New("", "", "")

	data := `{"model_id":"granite","choices":[{"index":0,"delta":{"content":"hi"},"finish_reason":""}]}`
	chunk, isDone, err := p.TransformStreamChunk(context.Background(), []byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if isDone {
		t.Fatal("should not be done")
	}
	if len(chunk.Choices) != 1 {
		t.Fatalf("choices: got %d", len(chunk.Choices))
	}
	if *chunk.Choices[0].Delta.Content != "hi" {
		t.Fatalf("content: got %v", chunk.Choices[0].Delta.Content)
	}

	data2 := `{"model_id":"granite","choices":[{"index":0,"delta":{"content":""},"finish_reason":"stop"}]}`
	_, isDone2, err := p.TransformStreamChunk(context.Background(), []byte(data2))
	if err != nil {
		t.Fatal(err)
	}
	if !isDone2 {
		t.Fatal("should be done")
	}
}

func TestTransformStreamChunkEmpty(t *testing.T) {
	p := New("", "", "")
	data := `{"model_id":"granite","choices":[]}`
	chunk, isDone, err := p.TransformStreamChunk(context.Background(), []byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if isDone {
		t.Fatal("should not be done with empty choices")
	}
	if len(chunk.Choices) != 0 {
		t.Fatalf("expected 0 choices, got %d", len(chunk.Choices))
	}
}

func TestTransformStreamChunkInvalid(t *testing.T) {
	p := New("", "", "")
	_, _, err := p.TransformStreamChunk(context.Background(), []byte("invalid"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestTransformToOpenAINilUsage(t *testing.T) {
	resp := &watsonxResponse{
		ModelID: "m",
		Choices: []watsonxChoice{},
		Usage:   nil,
	}
	result := transformToOpenAI(resp)
	if result.Usage.TotalTokens != 0 {
		t.Fatalf("expected zero usage")
	}
}

func TestTransformRequestWithCachedToken(t *testing.T) {
	p := New("https://example.com", "proj1", "key1")
	p.accessToken = "cached-token"
	p.tokenExpiry = time.Now().Add(time.Hour)

	temp := float64(0.7)
	maxTok := 100
	topP := float64(0.9)
	streaming := true
	req := &model.ChatCompletionRequest{
		Model: "granite-13b",
		Messages: []model.Message{
			{Role: "user", Content: "hello"},
		},
		Temperature: &temp,
		MaxTokens:   &maxTok,
		TopP:        &topP,
		Stop:        "END",
		Stream:      &streaming,
	}

	httpReq, err := p.TransformRequest(context.Background(), req, "")
	if err != nil {
		t.Fatal(err)
	}
	if httpReq.Header.Get("Authorization") != "Bearer cached-token" {
		t.Fatalf("auth header: got %q", httpReq.Header.Get("Authorization"))
	}
	if httpReq.Method != "POST" {
		t.Fatalf("method: got %q", httpReq.Method)
	}
	// Streaming should use chat_stream endpoint
	if httpReq.URL.Path != "/ml/v1/text/chat_stream" {
		t.Fatalf("path: got %q", httpReq.URL.Path)
	}
}

func TestTransformRequestNonStreaming(t *testing.T) {
	p := New("https://example.com", "proj1", "key1")
	p.accessToken = "token"
	p.tokenExpiry = time.Now().Add(time.Hour)

	req := &model.ChatCompletionRequest{
		Model: "granite-13b",
		Messages: []model.Message{
			{Role: "user", Content: "hi"},
		},
	}

	httpReq, err := p.TransformRequest(context.Background(), req, "override-key")
	if err != nil {
		t.Fatal(err)
	}
	if httpReq.URL.Path != "/ml/v1/text/chat" {
		t.Fatalf("path: got %q", httpReq.URL.Path)
	}
}
