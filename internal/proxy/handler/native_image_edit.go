package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/pricing"
)

// Native JSON uses only finalized, owner-bound files. Never search accounts for IDs.
func (h *Handlers) nativeImageCandidate(ctx context.Context, body []byte, candidates []resolvedOpenAISubscriptionCredential) (resolvedOpenAISubscriptionCredential, error) {
	var request struct {
		Images []struct {
			FileID string `json:"file_id"`
		} `json:"images"`
		Mask json.RawMessage `json:"mask"`
	}
	if json.Unmarshal(body, &request) != nil || len(request.Images) == 0 || len(request.Images) > 16 || len(request.Mask) > 0 {
		return resolvedOpenAISubscriptionCredential{}, errNativeFile
	}
	var binding nativeFileBinding
	for i, image := range request.Images {
		b, err := h.loadNativeFileBinding(ctx, image.FileID)
		if err != nil || !nativeFileID(image.FileID) || !b.Ready {
			return resolvedOpenAISubscriptionCredential{}, errNativeFile
		}
		if i > 0 && (binding.CredentialID != b.CredentialID || binding.AccountID != b.AccountID) {
			return resolvedOpenAISubscriptionCredential{}, errNativeFile
		}
		binding = b
	}
	// Candidates have already passed current route, tenant, credential and usage gates.
	for _, c := range candidates {
		if c.CredentialID == binding.CredentialID && c.AccountID == binding.AccountID && h.nativeFileReferences(ctx, body, c) {
			return c, nil
		}
	}
	return resolvedOpenAISubscriptionCredential{}, errNativeFile
}

func (h *Handlers) handleNativeImageEdit(w http.ResponseWriter, r *http.Request, body []byte, candidates []resolvedOpenAISubscriptionCredential, codex bool) {
	if !codex {
		nativeFileError(w, 400, "native file images require a subscription backend route")
		return
	}
	c, err := h.nativeImageCandidate(r.Context(), body, candidates)
	if err != nil {
		nativeFileError(w, 404, errNativeFile.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	// Do not rebuild JSON: native and unknown fields and Content-Type stay byte-exact.
	started := time.Now()
	defer h.refreshOpenAISubscriptionCodexUsageAfterResponse(r.Context(), []resolvedOpenAISubscriptionCredential{c})
	resp, err := h.nativeFileRequest(ctx, http.MethodPost, h.nativeFilesBase()+"/codex/images/edits", body, r.Header.Get("Content-Type"), c, r.Header)
	if err != nil {
		nativeFileError(w, 502, "native image transport failed; do not replay")
		return
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, (64<<20)+1))
	if err != nil || len(raw) > 64<<20 {
		nativeFileError(w, 502, "native image response unavailable; do not replay")
		return
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 && h.Callbacks != nil {
		var request struct {
			Model string `json:"model"`
		}
		_ = json.Unmarshal(body, &request)
		usage := extractResponsesUsage(raw)
		data := buildBaseLogData(r.Context(), started)
		data.Model = request.Model
		data.Provider = "chatgpt_codex_backend"
		data.CallType = "image_edit"
		data.EndTime = time.Now()
		data.Latency = time.Since(started)
		data.LLMAPILatency = data.Latency
		data.PromptTokens = usage.PromptTokens
		data.CompletionTokens = usage.CompletionTokens
		data.TotalTokens = usage.TotalTokens
		data.Cost = pricing.Default().TotalCost(request.Model, pricing.TokenUsage{PromptTokens: usage.PromptTokens, CompletionTokens: usage.CompletionTokens})
		data.ResponsePayload = map[string]any{"type": "native_image_edit", "response_bytes": len(raw)}
		applyOpenAISubscriptionLogAttribution(&data, openAISubscriptionProviderAttempt{credentialID: c.CredentialID, accountID: c.AccountID}.attribution(r.Context(), "success", ""))
		go h.Callbacks.LogSuccess(data)
	}
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(raw)
}
