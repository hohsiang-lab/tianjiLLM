package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestResponsesNativeLiteDispatch(t *testing.T) {
	for _, tt := range []struct {
		frame string
		want  bool
	}{
		{`{"client_metadata":{"ws_request_header_x_openai_internal_codex_responses_lite":"true"}}`, true},
		{`{"type":"response.create","model":"legacy"}`, false},
		{`{"client_metadata":{"ws_request_header_x_openai_internal_codex_responses_lite":"false"}}`, false},
		{`{"client_metadata":`, false},
	} {
		if got := nativeResponsesLiteFrame([]byte(tt.frame)); got != tt.want {
			t.Fatalf("dispatch %s: got %v", tt.frame, got)
		}
	}
}

func TestResponsesNativeRelayDuplex(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, e := websocket.Accept(w, r, nil)
		if e != nil {
			return
		}
		defer func() { _ = c.CloseNow() }()
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		for _, expected := range []string{`{"type":"response.create","native":true}`, `{"type":"response.interrupt","mode":"discard_partial_items"}`, `{"type":"response.create","next":true}`} {
			_, b, e := c.Read(ctx)
			if e != nil || string(b) != expected {
				t.Errorf("upstream frame = %s, err=%v", b, e)
				return
			}
			if c.Write(ctx, websocket.MessageText, []byte(`{"type":"unknown.native.event","keep":true}`)) != nil {
				return
			}
		}
	}))
	defer upstream.Close()
	relay := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		down, e := websocket.Accept(w, r, nil)
		if e != nil {
			return
		}
		defer func() { _ = down.CloseNow() }()
		up, _, e := websocket.Dial(r.Context(), "ws"+strings.TrimPrefix(upstream.URL, "http"), nil)
		if e != nil {
			t.Error(e)
			return
		}
		defer func() { _ = up.CloseNow() }()
		relayNativeResponses(r.Context(), down, up, nil)
	}))
	defer relay.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, e := websocket.Dial(ctx, "ws"+strings.TrimPrefix(relay.URL, "http"), nil)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = c.CloseNow() }()
	for _, frame := range []string{`{"type":"response.create","native":true}`, `{"type":"response.interrupt","mode":"discard_partial_items"}`, `{"type":"response.create","next":true}`} {
		if e = c.Write(ctx, websocket.MessageText, []byte(frame)); e != nil {
			t.Fatal(e)
		}
		_, b, e := c.Read(ctx)
		if e != nil || string(b) != `{"type":"unknown.native.event","keep":true}` {
			t.Fatalf("raw event lost: %s %v", b, e)
		}
	}
}
