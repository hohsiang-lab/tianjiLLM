package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coder/websocket"
)

func nativeResponsesLiteFrame(raw []byte) bool {
	var frame struct {
		ClientMetadata map[string]string `json:"client_metadata"`
	}
	return json.Unmarshal(raw, &frame) == nil && frame.ClientMetadata["ws_request_header_x_openai_internal_codex_responses_lite"] == "true"
}

// relayNativeResponses keeps one reader/writer per direction and no frame queue.
// Awaited writes provide backpressure; either close cancels both pumps.
func relayNativeResponses(ctx context.Context, down, up *websocket.Conn, observe func([]byte), validate ...func([]byte) bool) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 2)
	pump := func(dst, src *websocket.Conn, watch bool) {
		for {
			kind, b, e := src.Read(ctx)
			if e != nil {
				done <- e
				return
			}
			if !watch && len(validate) > 0 && !validate[0](b) {
				done <- errors.New("socket route changed")
				return
			}
			if watch && observe != nil {
				observe(b)
			}
			if e = dst.Write(ctx, kind, b); e != nil {
				done <- e
				return
			}
		}
	}
	go pump(up, down, false)
	go pump(down, up, true)
	err := <-done
	code := websocket.CloseStatus(err)
	if code < 1000 || code == 1005 || code == 1006 || code == 1015 {
		code = websocket.StatusGoingAway
	}
	cancel()
	// Do not expose upstream error text, URLs or credentials in close reasons.
	_ = down.Close(code, "")
	_ = up.Close(code, "")
	<-done
}

func (h *Handlers) handleNativeResponsesSocket(ctx context.Context, conn *websocket.Conn, first []byte, headers http.Header) {
	payload, frame, err := decodeResponsesCreateFrame(first)
	if err != nil || frame.Type != "response.create" || strings.TrimSpace(frame.Model) == "" {
		_ = conn.Close(websocket.StatusPolicyViolation, "first frame must select a model")
		return
	}
	route, err := h.resolveOpenAIEndpointRouteForModelStrict(ctx, frame.Model, false)
	if err != nil || route.Transport != openAISubscriptionTransportChatGPTCodexBackend || len(route.Candidates) == 0 {
		_ = conn.Close(websocket.StatusPolicyViolation, "native subscription route unavailable")
		return
	}
	candidates := h.orderOpenAISubscriptionCodexCandidatesWithIdentity(ctx, route.TianjiParams, route.Candidates, extractCodexSessionIdentity(payload))
	candidate := candidates[0] // Socket lifetime binding: never retry/fail over after a frame.
	if !h.nativeFileReferences(ctx, first, candidate) {
		_ = conn.Close(websocket.StatusPolicyViolation, "inaccessible native file")
		return
	}
	defer h.refreshOpenAISubscriptionCodexUsageAfterResponse(ctx, candidates[:1])
	transport := h.chatGPTCodexBackendTransportForParams(ctx, route.TianjiParams, frame.Model, candidates[:1])
	target, err := url.Parse(transport.NativeResponsesURL())
	if err != nil {
		_ = conn.Close(websocket.StatusInternalError, "")
		return
	}
	switch target.Scheme {
	case "https":
		target.Scheme = "wss"
	case "http":
		target.Scheme = "ws"
	default:
		_ = conn.Close(websocket.StatusInternalError, "")
		return
	}
	outbound := headers.Clone()
	outbound.Set("Authorization", "Bearer "+candidate.BearerToken)
	outbound.Set("ChatGPT-Account-Id", candidate.AccountID)
	up, resp, err := websocket.Dial(ctx, target.String(), &websocket.DialOptions{HTTPClient: h.openAIUpstreamHTTPClient(), HTTPHeader: outbound})
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		_ = conn.Close(websocket.StatusTryAgainLater, "native upstream unavailable")
		return
	}
	defer func() { _ = up.CloseNow() }()
	up.SetReadLimit(responsesWebSocketReadLimit)
	if err = up.Write(ctx, websocket.MessageText, first); err != nil {
		return
	}
	started := time.Now()
	seen := map[string]bool{}
	attr := openAISubscriptionProviderAttempt{credentialID: candidate.CredentialID, accountID: candidate.AccountID}.attribution(ctx, "success", "")
	relayNativeResponses(ctx, conn, up, func(raw []byte) {
		var event struct {
			Type     string `json:"type"`
			Response struct {
				ID string `json:"id"`
			} `json:"response"`
		}
		if json.Unmarshal(raw, &event) != nil {
			return
		}
		switch event.Type {
		case "response.completed", "response.incomplete", "response.failed":
		default:
			return
		}
		if event.Response.ID == "" || seen[event.Response.ID] {
			return
		}
		seen[event.Response.ID] = true
		// Observe only real upstream terminal usage; never synthesize completion/ack.
		if event.Type == "response.failed" {
			h.logResponsesFailure(ctx, frame.Model, "", started, errors.New("native upstream response failed"), attr)
			return
		}
		h.logResponsesSuccess(ctx, frame.Model, "", extractResponsesUsage(raw), started, time.Now(), time.Since(started), attr, nil, responsesResponseAuditPayload(raw))
		started = time.Now()
	}, func(raw []byte) bool {
		var envelope struct {
			Type  string `json:"type"`
			Model string `json:"model"`
		}
		if json.Unmarshal(raw, &envelope) != nil {
			return false
		}
		return (envelope.Type != "response.create" || envelope.Model == frame.Model) && h.nativeFileReferences(ctx, raw, candidate)
	})
}
