package handler

import (
	"context"
	"strings"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/chatgptcodex"
)

func (h *Handlers) chatGPTCodexBackendTransport() chatgptcodex.Transport {
	cfg := config.ResolveOpenAIOAuthConfig(h.openAIOAuthConfig())
	return chatgptcodex.Transport{
		BaseURL: cfg.CodexBackendBaseURL,
	}
}

func (h *Handlers) chatGPTCodexBackendTransportForParams(ctx context.Context, params config.TianjiParams, modelName string, candidates []resolvedOpenAISubscriptionCredential) chatgptcodex.Transport {
	transport := h.chatGPTCodexBackendTransport()
	catalogParams := params
	if modelName != "" {
		catalogParams.Model = modelName
	}
	if len(candidates) > 0 {
		candidateIDs := make([]string, 0, len(candidates))
		for _, candidate := range candidates {
			if candidate.CredentialID != "" {
				candidateIDs = append(candidateIDs, candidate.CredentialID)
			}
		}
		catalogParams.OpenAISubscriptionCredentialIDs = candidateIDs
		params.OpenAISubscriptionCredentialIDs = candidateIDs
	}
	lookupParams := []config.TianjiParams{catalogParams}
	if catalogParams.Model != params.Model && !strings.Contains(params.Model, "*") {
		lookupParams = append(lookupParams, params)
	}
	for _, lookup := range lookupParams {
		if useResponsesLite, ok := h.codexResponsesLiteForParamsCached(ctx, lookup); ok {
			transport.UseResponsesLite = useResponsesLite
			return transport
		}
	}
	for _, lookup := range lookupParams {
		if useResponsesLite, ok := h.codexResponsesLiteForParams(ctx, lookup); ok {
			transport.UseResponsesLite = useResponsesLite
			return transport
		}
	}
	return transport
}
