package callback

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// GCSPubSubCallback publishes LLM call data to Google Cloud Pub/Sub via HTTP.
type GCSPubSubCallback struct {
	projectID string
	topicID   string
	baseURL   string
}

func NewGCSPubSubCallback(projectID, topicID, baseURL string) *GCSPubSubCallback {
	if baseURL == "" {
		baseURL = "https://pubsub.googleapis.com"
	}
	return &GCSPubSubCallback{projectID: projectID, topicID: topicID, baseURL: baseURL}
}

func (c *GCSPubSubCallback) LogSuccess(data LogData) { c.log(data) }
func (c *GCSPubSubCallback) LogFailure(data LogData) { c.log(data) }

func (c *GCSPubSubCallback) log(data LogData) {
	msgData := map[string]any{
		"model":             data.Model,
		"provider":          data.Provider,
		"prompt_tokens":     data.PromptTokens,
		"completion_tokens": data.CompletionTokens,
		"total_tokens":      data.TotalTokens,
		"cost":              data.Cost,
		"latency_ms":        data.Latency.Milliseconds(),
		"start_time":        data.StartTime.Format(time.RFC3339Nano),
		"user_id":           data.UserID,
		"team_id":           data.TeamID,
		"tags":              data.RequestTags,
	}
	if data.Error != nil {
		msgData["error"] = data.Error.Error()
	}

	msgJSON, _ := json.Marshal(msgData)
	payload := map[string]any{
		"messages": []map[string]any{
			{"data": msgJSON},
		},
	}

	body, _ := json.Marshal(payload)
	url := c.baseURL + "/v1/projects/" + c.projectID + "/topics/" + c.topicID + ":publish"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}
