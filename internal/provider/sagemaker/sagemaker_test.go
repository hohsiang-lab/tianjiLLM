package sagemaker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

func TestNew(t *testing.T) {
	p := New("")
	if p.region != "us-east-1" {
		t.Fatalf("got %q, want us-east-1", p.region)
	}
	p2 := New("eu-west-1")
	if p2.region != "eu-west-1" {
		t.Fatalf("got %q", p2.region)
	}
}

func TestGetSupportedParams(t *testing.T) {
	p := New("")
	params := p.GetSupportedParams()
	if len(params) == 0 {
		t.Fatal("expected params")
	}
}

func TestMapParams(t *testing.T) {
	p := New("")
	input := map[string]any{"temperature": 0.5}
	result := p.MapParams(input)
	if result["temperature"] != 0.5 {
		t.Fatalf("got %v", result["temperature"])
	}
}

func TestGetRequestURL(t *testing.T) {
	p := New("us-west-2")
	u := p.GetRequestURL("my-endpoint")
	expected := "https://runtime.sagemaker.us-west-2.amazonaws.com/endpoints/my-endpoint/invocations"
	if u != expected {
		t.Fatalf("got %q", u)
	}
}

func TestSetupHeaders(t *testing.T) {
	p := New("")
	req, _ := http.NewRequest("POST", "http://example.com", nil)
	p.SetupHeaders(req, "key")
	if req.Header.Get("Content-Type") != "application/json" {
		t.Fatal("missing content-type")
	}
}

func TestTransformResponse(t *testing.T) {
	modelResp := model.ModelResponse{
		Object: "chat.completion",
		Model:  "my-endpoint",
		Choices: []model.Choice{
			{Index: 0, Message: &model.Message{Role: "assistant", Content: "hi"}},
		},
		Usage: model.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
	}
	body, _ := json.Marshal(modelResp)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	p := New("")
	result, err := p.TransformResponse(context.Background(), resp)
	if err != nil {
		t.Fatal(err)
	}
	if result.Model != "my-endpoint" {
		t.Fatalf("model: got %q", result.Model)
	}
	if len(result.Choices) != 1 {
		t.Fatalf("choices: got %d", len(result.Choices))
	}
}

func TestTransformResponseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error"))
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	p := New("")
	_, err = p.TransformResponse(context.Background(), resp)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestTransformStreamChunk(t *testing.T) {
	p := New("")

	finish := "stop"
	chunk := model.StreamChunk{
		Model: "ep",
		Choices: []model.StreamChoice{
			{Index: 0, FinishReason: &finish},
		},
	}
	data, _ := json.Marshal(chunk)

	result, isDone, err := p.TransformStreamChunk(context.Background(), data)
	if err != nil {
		t.Fatal(err)
	}
	if !isDone {
		t.Fatal("expected done")
	}
	if result.Model != "ep" {
		t.Fatalf("model: got %q", result.Model)
	}
}

func TestTransformStreamChunkNotDone(t *testing.T) {
	p := New("")
	chunk := model.StreamChunk{
		Model:   "ep",
		Choices: []model.StreamChoice{{Index: 0}},
	}
	data, _ := json.Marshal(chunk)
	_, isDone, err := p.TransformStreamChunk(context.Background(), data)
	if err != nil {
		t.Fatal(err)
	}
	if isDone {
		t.Fatal("should not be done")
	}
}

func TestTransformStreamChunkInvalid(t *testing.T) {
	p := New("")
	_, _, err := p.TransformStreamChunk(context.Background(), []byte("bad"))
	if err == nil {
		t.Fatal("expected error")
	}
}
