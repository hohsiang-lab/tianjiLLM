package chatgptcodex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"strings"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

type Transport struct {
	BaseURL          string
	UseResponsesLite bool
}

func (t Transport) BuildRequest(ctx context.Context, req *model.ChatCompletionRequest, bearerToken, accountID string) (*http.Request, error) {
	payload, err := BuildPayload(req)
	if err != nil {
		return nil, err
	}
	prepared, err := t.PrepareResponsesPayload(payload, req.IsStreaming())
	if err != nil {
		return nil, err
	}
	return t.BuildPreparedResponsesRequest(ctx, prepared, bearerToken, accountID)
}

func (t Transport) PrepareResponsesPayload(payload any, stream bool) (PreparedResponsesPayload, error) {
	prepared, err := PrepareResponsesPayload(payload, stream)
	if err != nil || !t.UseResponsesLite {
		return prepared, err
	}
	prepared.body, err = normalizeResponsesLitePayload(prepared.body)
	if err != nil {
		return PreparedResponsesPayload{}, fmt.Errorf("prepare ChatGPT Codex Responses Lite request: %w", err)
	}
	prepared.responsesLitePrepared = true
	return prepared, nil
}

func (t Transport) PrepareCompactResponsesPayload(payload any) (PreparedCompactResponsesPayload, error) {
	prepared, err := PrepareCompactResponsesPayload(payload)
	if err != nil || !t.UseResponsesLite {
		return prepared, err
	}
	prepared.body, err = normalizeResponsesLitePayload(prepared.body)
	if err != nil {
		return PreparedCompactResponsesPayload{}, fmt.Errorf("prepare ChatGPT Codex Responses Lite request: %w", err)
	}
	prepared.responsesLitePrepared = true
	return prepared, nil
}

func (t Transport) BuildPreparedResponsesRequest(ctx context.Context, prepared PreparedResponsesPayload, bearerToken, accountID string, clientHeaders ...http.Header) (*http.Request, error) {
	if len(prepared.body) == 0 {
		return nil, fmt.Errorf("build ChatGPT Codex backend responses request: payload is not prepared")
	}
	body := prepared.body
	if t.UseResponsesLite && !prepared.responsesLitePrepared {
		var err error
		body, err = normalizeResponsesLitePayload(body)
		if err != nil {
			return nil, fmt.Errorf("prepare ChatGPT Codex Responses Lite request: %w", err)
		}
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, t.responsesURL(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create ChatGPT Codex backend responses request: %w", err)
	}
	t.applyHeaders(httpReq, bearerToken, accountID)
	copyCodexClientHeaders(httpReq, clientHeaders)
	return httpReq, nil
}

func (t Transport) BuildCompactRequest(ctx context.Context, prepared PreparedCompactResponsesPayload, bearerToken, accountID string, clientHeaders ...http.Header) (*http.Request, error) {
	if len(prepared.body) == 0 {
		return nil, fmt.Errorf("build ChatGPT Codex backend compact request: payload is not prepared")
	}
	body := prepared.body
	if t.UseResponsesLite && !prepared.responsesLitePrepared {
		var err error
		body, err = normalizeResponsesLitePayload(body)
		if err != nil {
			return nil, fmt.Errorf("prepare ChatGPT Codex Responses Lite request: %w", err)
		}
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, t.responsesCompactURL(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create ChatGPT Codex backend compact request: %w", err)
	}
	t.applyHeaders(httpReq, bearerToken, accountID)
	copyCodexClientHeaders(httpReq, clientHeaders)
	return httpReq, nil
}

func (t Transport) BuildImageGenerationRequest(ctx context.Context, prepared PreparedResponsesPayload, bearerToken, accountID string, clientHeaders ...http.Header) (*http.Request, error) {
	httpReq, err := t.BuildPreparedResponsesRequest(ctx, prepared, bearerToken, accountID, clientHeaders...)
	if err != nil {
		return nil, err
	}
	if os.Getenv("TIANJI_DEBUG_CODEX_IMAGE") == "1" {
		payload, _ := prepared.Snapshot().(map[string]any)
		log.Printf("debug: codex-image outbound url=%s account_id_set=%t payload=%s", t.responsesURL(), strings.TrimSpace(accountID) != "", imageGenerationPayloadSummary(payload))
	}
	return httpReq, nil
}

func (t Transport) applyHeaders(httpReq *http.Request, bearerToken, accountID string) {
	httpReq.Header.Set("Authorization", "Bearer "+bearerToken)
	httpReq.Header.Set("Content-Type", "application/json")
	if t.UseResponsesLite {
		httpReq.Header.Set("x-openai-internal-codex-responses-lite", "true")
	}
	if strings.TrimSpace(accountID) != "" {
		httpReq.Header.Set("ChatGPT-Account-Id", accountID)
	}
}

func copyCodexClientHeaders(httpReq *http.Request, clientHeaders []http.Header) {
	if httpReq == nil || len(clientHeaders) == 0 {
		return
	}
	for _, name := range []string{
		"Accept",
		"OpenAI-Beta",
		"Version",
		"originator",
		"x-codex-turn-state",
		"x-codex-turn-metadata",
		"x-codex-routing-hint",
	} {
		values := clientHeaders[0].Values(name)
		if len(values) == 0 {
			continue
		}
		httpReq.Header.Del(name)
		for _, value := range values {
			httpReq.Header.Add(name, value)
		}
	}
	// The outbound payload is re-encoded JSON. Preserve a client JSON media
	// type when it still describes those bytes; do not forward a source type
	// such as multipart/form-data after that body has been converted to JSON.
	contentTypes := clientHeaders[0].Values("Content-Type")
	if len(contentTypes) == 1 {
		if mediaType, _, err := mime.ParseMediaType(contentTypes[0]); err == nil && strings.EqualFold(mediaType, "application/json") {
			httpReq.Header.Set("Content-Type", contentTypes[0])
		}
	}
}

func (t Transport) TransformResponse(resp *http.Response, fallbackModel string) (*model.ModelResponse, error) {
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, parseCodexErrorResponse(resp, "chatgpt_codex_backend")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read ChatGPT Codex backend response: %w", err)
	}
	var already model.ModelResponse
	if err := json.Unmarshal(body, &already); err == nil && len(already.Choices) > 0 {
		return &already, nil
	}
	return transformCodexResponseBody(body, fallbackModel)
}

func (t Transport) responsesURL() string {
	base := strings.TrimRight(strings.TrimSpace(t.BaseURL), "/")
	if base == "" {
		base = "https://chatgpt.com/backend-api"
	}
	if strings.HasSuffix(base, "/codex/responses") {
		return base
	}
	if !strings.HasSuffix(base, "/backend-api") {
		base += "/backend-api"
	}
	return base + "/codex/responses"
}

func (t Transport) responsesCompactURL() string {
	return strings.TrimRight(t.responsesURL(), "/") + "/compact"
}
