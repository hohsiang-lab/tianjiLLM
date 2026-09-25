package handler

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/coder/websocket"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// WebSocketRelay handles bidirectional WebSocket proxying for the Realtime API.
type WebSocketRelay struct {
	resolveUpstream func(model string) (upstreamURL, apiKey string, err error)
}

// NewWebSocketRelay creates a relay that resolves upstream WebSocket URLs.
func NewWebSocketRelay(resolver func(model string) (string, string, error)) *WebSocketRelay {
	return &WebSocketRelay{resolveUpstream: resolver}
}

// ServeHTTP upgrades the client connection, dials the upstream, and relays messages bidirectionally.
func (ws *WebSocketRelay) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	modelName := r.URL.Query().Get("model")
	if modelName == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "missing model query parameter", Type: "invalid_request_error"},
		})
		return
	}

	upstreamURL, apiKey, err := ws.resolveUpstream(modelName)
	if err != nil {
		writeJSON(w, http.StatusNotFound, model.ErrorResponse{
			Error: model.ErrorDetail{Message: err.Error(), Type: "model_not_found"},
		})
		return
	}

	// Accept the client WebSocket
	clientConn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Printf("realtime: failed to accept client WebSocket: %v", err)
		return
	}
	defer func() { _ = clientConn.CloseNow() }()

	// Dial the upstream WebSocket
	upstreamHeaders := http.Header{}
	if apiKey != "" {
		upstreamHeaders.Set("Authorization", "Bearer "+apiKey)
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	upstreamConn, _, err := websocket.Dial(ctx, upstreamURL, &websocket.DialOptions{
		HTTPHeader: upstreamHeaders,
	})
	if err != nil {
		clientConn.Close(websocket.StatusInternalError, fmt.Sprintf("upstream dial failed: %v", err))
		return
	}
	defer func() { _ = upstreamConn.CloseNow() }()

	// Bidirectional relay: two goroutines, context cancellation propagates cleanup
	errc := make(chan error, 2)

	// Client → Upstream
	go func() {
		errc <- relay(ctx, clientConn, upstreamConn)
	}()

	// Upstream → Client
	go func() {
		errc <- relay(ctx, upstreamConn, clientConn)
	}()

	// Wait for either direction to finish
	err = <-errc
	cancel() // cancel the other direction

	if err != nil {
		log.Printf("realtime: relay ended: %v", err)
	}

	// Close both sides gracefully
	clientConn.Close(websocket.StatusNormalClosure, "session ended")
	upstreamConn.Close(websocket.StatusNormalClosure, "session ended")
}

// relay reads messages from src and writes them to dst until ctx is cancelled or an error occurs.
func relay(ctx context.Context, src, dst *websocket.Conn) error {
	for {
		msgType, data, err := src.Read(ctx)
		if err != nil {
			return err
		}
		if err := dst.Write(ctx, msgType, data); err != nil {
			return err
		}
	}
}
