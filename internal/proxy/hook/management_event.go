package hook

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// ManagementEvent represents a management operation event (key/team/user CRUD).
type ManagementEvent struct {
	EventType string `json:"event_type"` // e.g. "key_created", "team_deleted"
	ObjectID  string `json:"object_id"`
	Payload   any    `json:"payload,omitempty"`
	Timestamp string `json:"timestamp"`
}

// ManagementEventDispatcher fires webhooks on management events.
type ManagementEventDispatcher struct {
	webhookURL string
	client     *http.Client
}

// NewManagementEventDispatcher creates a dispatcher. If webhookURL is empty, events are logged only.
func NewManagementEventDispatcher(webhookURL string) *ManagementEventDispatcher {
	return &ManagementEventDispatcher{
		webhookURL: webhookURL,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

// Dispatch sends a management event. Non-blocking — errors are logged, not returned.
func (d *ManagementEventDispatcher) Dispatch(ctx context.Context, event ManagementEvent) {
	event.Timestamp = time.Now().UTC().Format(time.RFC3339)

	if d.webhookURL == "" {
		log.Printf("management_event: %s object=%s", event.EventType, event.ObjectID)
		return
	}

	go func() {
		body, err := json.Marshal(event)
		if err != nil {
			log.Printf("management_event: marshal error: %v", err)
			return
		}

		reqCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, d.webhookURL, bytes.NewReader(body))
		if err != nil {
			log.Printf("management_event: request error: %v", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := d.client.Do(req)
		if err != nil {
			log.Printf("management_event: webhook error: %v", err)
			return
		}
		resp.Body.Close()

		if resp.StatusCode >= 400 {
			log.Printf("management_event: webhook returned %d for %s", resp.StatusCode, event.EventType)
		}
	}()
}
