package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// transformEmbeddingRequestBody builds the upstream request body from an
// EmbeddingRequest, merging any ExtraParams for provider-specific pass-through.
func transformEmbeddingRequestBody(req *model.EmbeddingRequest) map[string]any {
	body := map[string]any{
		"model": req.Model,
		"input": req.Input,
	}
	if req.EncodingFormat != "" {
		body["encoding_format"] = req.EncodingFormat
	}
	if req.Dimensions != nil {
		body["dimensions"] = *req.Dimensions
	}
	if req.User != "" {
		body["user"] = req.User
	}
	// Merge provider-specific extra params (e.g. Jina's "task" and "normalized").
	// Skip keys already set to prevent ExtraParams from overriding known fields.
	for k, v := range req.ExtraParams {
		if _, exists := body[k]; !exists {
			body[k] = v
		}
	}
	return body
}

func (p *Provider) TransformEmbeddingRequest(ctx context.Context, req *model.EmbeddingRequest, apiKey string) (*http.Request, error) {
	data, err := json.Marshal(transformEmbeddingRequestBody(req))
	if err != nil {
		return nil, fmt.Errorf("marshal embedding request: %w", err)
	}

	url := p.baseURL + "/embeddings"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create embedding request: %w", err)
	}

	p.SetupHeaders(httpReq, apiKey)
	return httpReq, nil
}

func (p *Provider) TransformEmbeddingResponse(ctx context.Context, resp *http.Response) (*model.EmbeddingResponse, error) {
	return p.transformEmbeddingResponse(ctx, resp, false)
}

// TransformEmbeddingResponseWithTotalOnlyUsage lets compatible provider
// adapters normalize total-only embedding usage while the standard OpenAI
// path continues to require both prompt_tokens and total_tokens.
func (p *Provider) TransformEmbeddingResponseWithTotalOnlyUsage(ctx context.Context, resp *http.Response) (*model.EmbeddingResponse, error) {
	return p.transformEmbeddingResponse(ctx, resp, true)
}

func (p *Provider) transformEmbeddingResponse(_ context.Context, resp *http.Response, allowTotalOnlyUsage bool) (*model.EmbeddingResponse, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseErrorResponse(resp)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read embedding response: %w", err)
	}

	var wire struct {
		Object string                `json:"object"`
		Data   []model.EmbeddingData `json:"data"`
		Model  string                `json:"model"`
		Usage  *struct {
			PromptTokens *int `json:"prompt_tokens"`
			TotalTokens  *int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &wire); err != nil {
		return nil, fmt.Errorf("parse embedding response: %w", err)
	}
	if wire.Usage == nil || wire.Usage.TotalTokens == nil ||
		(wire.Usage.PromptTokens == nil && !allowTotalOnlyUsage) {
		return nil, fmt.Errorf("invalid embedding response usage")
	}
	promptTokens := *wire.Usage.TotalTokens
	if wire.Usage.PromptTokens != nil {
		promptTokens = *wire.Usage.PromptTokens
	}
	result := model.EmbeddingResponse{
		Object: wire.Object,
		Data:   wire.Data,
		Model:  wire.Model,
		Usage: model.EmbeddingUsage{
			PromptTokens: promptTokens,
			TotalTokens:  *wire.Usage.TotalTokens,
		},
	}
	if err := validateEmbeddingResponse(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func validateEmbeddingResponse(result *model.EmbeddingResponse) error {
	if result.Object != "list" {
		return fmt.Errorf("invalid embedding response object %q", result.Object)
	}
	if len(result.Data) == 0 {
		return fmt.Errorf("embedding response data is empty")
	}
	for i, item := range result.Data {
		if item.Object != "embedding" {
			return fmt.Errorf("invalid embedding response data[%d].object %q", i, item.Object)
		}
		if item.Index != i {
			return fmt.Errorf("invalid embedding response data[%d].index %d", i, item.Index)
		}
		if len(item.Embedding) == 0 {
			return fmt.Errorf("embedding response data[%d].embedding is empty", i)
		}
	}
	if result.Model == "" {
		return fmt.Errorf("embedding response model is empty")
	}
	if result.Usage.PromptTokens < 0 ||
		result.Usage.TotalTokens < 0 ||
		result.Usage.TotalTokens < result.Usage.PromptTokens {
		return fmt.Errorf("invalid embedding response usage")
	}
	return nil
}
