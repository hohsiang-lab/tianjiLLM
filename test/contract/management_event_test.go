package contract

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/proxy/hook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManagementEventDispatcher_LogOnly(t *testing.T) {
	d := hook.NewManagementEventDispatcher("")

	// Should not panic with empty webhook URL
	d.Dispatch(context.Background(), hook.ManagementEvent{
		EventType: "key_created",
		ObjectID:  "test-key-123",
		Payload:   map[string]string{"key_name": "test"},
	})
}

func TestManagementEventDispatcher_WebhookDelivery(t *testing.T) {
	var received hook.ManagementEvent
	done := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(done)
		require.Equal(t, "POST", r.Method)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))

		err := json.NewDecoder(r.Body).Decode(&received)
		require.NoError(t, err)

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := hook.NewManagementEventDispatcher(srv.URL)
	d.Dispatch(context.Background(), hook.ManagementEvent{
		EventType: "team_deleted",
		ObjectID:  "team-456",
	})

	select {
	case <-done:
		assert.Equal(t, "team_deleted", received.EventType)
		assert.Equal(t, "team-456", received.ObjectID)
		assert.NotEmpty(t, received.Timestamp)
	case <-time.After(5 * time.Second):
		t.Fatal("webhook not received within timeout")
	}
}

func TestManagementEventDispatcher_WebhookErrorNoBlock(t *testing.T) {
	// Webhook returns error — should not block or panic
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	d := hook.NewManagementEventDispatcher(srv.URL)
	d.Dispatch(context.Background(), hook.ManagementEvent{
		EventType: "user_created",
		ObjectID:  "user-789",
	})

	// Give time for the goroutine to execute
	time.Sleep(100 * time.Millisecond)
}
