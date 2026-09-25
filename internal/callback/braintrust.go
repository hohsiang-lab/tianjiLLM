package callback

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

// BraintrustLogger sends batched log data to Braintrust.
type BraintrustLogger struct {
	*BatchLogger
	apiKey  string
	baseURL string
	project string
}

// NewBraintrustLogger creates a Braintrust callback.
func NewBraintrustLogger(apiKey, baseURL, project string) *BraintrustLogger {
	if baseURL == "" {
		baseURL = "https://api.braintrustdata.com"
	}
	l := &BraintrustLogger{
		apiKey:  apiKey,
		baseURL: baseURL,
		project: project,
	}
	l.BatchLogger = NewBatchLogger(l.flush)
	return l
}

func (l *BraintrustLogger) flush(batch []LogData) error {
	events := make([]map[string]any, 0, len(batch))
	for _, d := range batch {
		event := map[string]any{
			"id":     uuid.New().String(),
			"input":  d.Request,
			"output": d.Response,
			"metadata": map[string]any{
				"model":             d.Model,
				"provider":          d.Provider,
				"prompt_tokens":     d.PromptTokens,
				"completion_tokens": d.CompletionTokens,
				"cost":              d.Cost,
			},
			"metrics": map[string]any{
				"latency_ms": d.Latency.Milliseconds(),
			},
		}
		events = append(events, event)
	}

	body, _ := json.Marshal(map[string]any{
		"events": events,
	})

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, l.baseURL+"/v1/project_logs/"+l.project+"/insert", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+l.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("braintrust: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("braintrust: status %d", resp.StatusCode)
	}
	return nil
}
