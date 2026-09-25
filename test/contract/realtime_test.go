package contract

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRealtime_MissingModel(t *testing.T) {
	relay := handler.NewWebSocketRelay(func(model string) (string, string, error) {
		return "", "", nil
	})
	srv := httptest.NewServer(relay)
	defer srv.Close()

	resp, err := http.Get(srv.URL) // no ?model= param, not a WS upgrade
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRealtime_ModelNotFound(t *testing.T) {
	relay := handler.NewWebSocketRelay(func(model string) (string, string, error) {
		return "", "", assert.AnError
	})
	srv := httptest.NewServer(relay)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "?model=unknown-model")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestRealtime_BidirectionalRelay(t *testing.T) {
	// Mock upstream WS server: echoes messages back
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer func() { _ = c.CloseNow() }()

		ctx := r.Context()
		for {
			msgType, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			// Echo back with "echo:" prefix
			if err := c.Write(ctx, msgType, []byte("echo:"+string(data))); err != nil {
				return
			}
		}
	}))
	defer upstream.Close()

	upstreamWS := "ws" + strings.TrimPrefix(upstream.URL, "http")

	relay := handler.NewWebSocketRelay(func(model string) (string, string, error) {
		return upstreamWS, "test-key", nil
	})
	srv := httptest.NewServer(relay)
	defer srv.Close()

	// Connect client to relay
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	clientURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "?model=gpt-4o-realtime-preview"
	c, _, err := websocket.Dial(ctx, clientURL, nil)
	require.NoError(t, err)
	defer func() { _ = c.CloseNow() }()

	// Send a message through the relay
	err = c.Write(ctx, websocket.MessageText, []byte("hello"))
	require.NoError(t, err)

	// Read the echoed response
	msgType, data, err := c.Read(ctx)
	require.NoError(t, err)
	assert.Equal(t, websocket.MessageText, msgType)
	assert.Equal(t, "echo:hello", string(data))

	// Send another message
	err = c.Write(ctx, websocket.MessageText, []byte("world"))
	require.NoError(t, err)

	_, data, err = c.Read(ctx)
	require.NoError(t, err)
	assert.Equal(t, "echo:world", string(data))

	// Clean close
	c.Close(websocket.StatusNormalClosure, "done")
}

func TestRealtime_UpstreamAuthHeader(t *testing.T) {
	gotAuth := make(chan string, 1)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Send auth header first before Accept (avoid race if Accept fails)
		gotAuth <- r.Header.Get("Authorization")
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			t.Logf("upstream websocket.Accept failed: %v", err)
			return
		}
		defer func() { _ = c.CloseNow() }()
		c.Close(websocket.StatusNormalClosure, "ok")
	}))
	defer upstream.Close()

	upstreamWS := "ws" + strings.TrimPrefix(upstream.URL, "http")

	relay := handler.NewWebSocketRelay(func(model string) (string, string, error) {
		return upstreamWS, "sk-secret-123", nil
	})
	srv := httptest.NewServer(relay)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	clientURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "?model=gpt-4o-realtime-preview"
	c, _, err := websocket.Dial(ctx, clientURL, nil)
	require.NoError(t, err, "client dial should succeed")
	defer func() {
		if c != nil {
			_ = c.CloseNow()
		}
	}()

	select {
	case auth := <-gotAuth:
		assert.Equal(t, "Bearer sk-secret-123", auth)
	case <-ctx.Done():
		t.Fatal("timed out waiting for upstream to receive auth header")
	}
}
