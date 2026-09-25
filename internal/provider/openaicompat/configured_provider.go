package openaicompat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
)

// configuredProvider decorates a first-class provider with the optional
// providers.json settings without replacing its provider-specific behavior.
type configuredProvider struct {
	inner  provider.Provider
	config SimpleProviderConfig
}

// configuredEmbeddingProvider keeps the EmbeddingProvider capability only when
// the decorated first-class provider actually supports embeddings.
type configuredEmbeddingProvider struct {
	*configuredProvider
	embedding provider.EmbeddingProvider
}

func newConfiguredProvider(inner provider.Provider, cfg SimpleProviderConfig) provider.Provider {
	configured := &configuredProvider{
		inner:  inner,
		config: normalizeConfig(cfg),
	}
	if embedding, ok := inner.(provider.EmbeddingProvider); ok {
		return &configuredEmbeddingProvider{
			configuredProvider: configured,
			embedding:          embedding,
		}
	}
	return configured
}

func (p *configuredProvider) TransformRequest(ctx context.Context, req *model.ChatCompletionRequest, apiKey string) (*http.Request, error) {
	httpReq, err := p.inner.TransformRequest(ctx, req, apiKey)
	if err != nil {
		return nil, err
	}
	return p.applyRequestConfig(httpReq, apiKey)
}

func (p *configuredProvider) TransformResponse(ctx context.Context, resp *http.Response) (*model.ModelResponse, error) {
	return p.inner.TransformResponse(ctx, resp)
}

func (p *configuredProvider) TransformStreamChunk(ctx context.Context, data []byte) (*model.StreamChunk, bool, error) {
	return p.inner.TransformStreamChunk(ctx, data)
}

func (p *configuredProvider) GetSupportedParams() []string {
	if len(p.config.SupportedParams) > 0 {
		return p.config.SupportedParams
	}
	return p.inner.GetSupportedParams()
}

func (p *configuredProvider) MapParams(params map[string]any) map[string]any {
	if len(p.config.ParamMappings) == 0 {
		return p.inner.MapParams(params)
	}

	unmapped := make(map[string]any, len(params))
	customMapped := make(map[string]any, len(p.config.ParamMappings))
	for key, value := range params {
		if mapped, ok := p.config.ParamMappings[key]; ok {
			customMapped[mapped] = value
		} else {
			unmapped[key] = value
		}
	}

	result := p.inner.MapParams(unmapped)
	for key, value := range customMapped {
		result[key] = value
	}
	return result
}

func (p *configuredProvider) GetRequestURL(modelName string) string {
	return p.inner.GetRequestURL(modelName)
}

func (p *configuredProvider) SetupHeaders(req *http.Request, apiKey string) {
	p.inner.SetupHeaders(req, apiKey)

	if !strings.EqualFold(p.config.AuthHeader, "Authorization") {
		req.Header.Del("Authorization")
	}
	req.Header.Set(p.config.AuthHeader, p.config.AuthPrefix+apiKey)
	for key, value := range p.config.Headers {
		req.Header.Set(key, value)
	}
}

func (p *configuredProvider) UnderlyingProvider() provider.Provider {
	return p.inner
}

func (p *configuredProvider) PreservesStandardParameter(param string) bool {
	mapped := p.MapParams(map[string]any{param: true})
	_, ok := mapped[param]
	return ok && len(mapped) == 1
}

func (p *configuredProvider) WithBaseURL(baseURL string) provider.Provider {
	configurable, ok := p.inner.(provider.BaseURLConfigurable)
	if !ok {
		return p
	}

	cfg := p.config
	cfg.BaseURL = baseURL
	return newConfiguredProvider(configurable.WithBaseURL(baseURL), cfg)
}

func (p *configuredEmbeddingProvider) TransformEmbeddingRequest(ctx context.Context, req *model.EmbeddingRequest, apiKey string) (*http.Request, error) {
	httpReq, err := p.embedding.TransformEmbeddingRequest(ctx, req, apiKey)
	if err != nil {
		return nil, err
	}
	return p.applyRequestConfig(httpReq, apiKey)
}

func (p *configuredEmbeddingProvider) TransformEmbeddingResponse(ctx context.Context, resp *http.Response) (*model.EmbeddingResponse, error) {
	return p.embedding.TransformEmbeddingResponse(ctx, resp)
}

func (p *configuredProvider) applyRequestConfig(req *http.Request, apiKey string) (*http.Request, error) {
	if req.Body != nil && (len(p.config.ParamMappings) > 0 || len(p.config.Constraints) > 0) {
		body, readErr := io.ReadAll(req.Body)
		if readErr != nil {
			return nil, fmt.Errorf("read configured provider request: %w", readErr)
		}
		if closeErr := req.Body.Close(); closeErr != nil {
			return nil, fmt.Errorf("close configured provider request: %w", closeErr)
		}

		var params map[string]any
		if decodeErr := json.Unmarshal(body, &params); decodeErr != nil {
			return nil, fmt.Errorf("parse configured provider request: %w", decodeErr)
		}
		params = p.MapParams(params)
		params = ApplyConstraints(params, p.config.Constraints)

		body, marshalErr := json.Marshal(params)
		if marshalErr != nil {
			return nil, fmt.Errorf("marshal configured provider request: %w", marshalErr)
		}
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.ContentLength = int64(len(body))
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(body)), nil
		}
	}

	p.SetupHeaders(req, apiKey)
	return req, nil
}
