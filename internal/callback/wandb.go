package callback

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

// WandbLogger sends batched log data to Weights & Biases.
type WandbLogger struct {
	*BatchLogger
	apiKey  string
	baseURL string
	project string
	entity  string
}

// NewWandbLogger creates a W&B callback.
func NewWandbLogger(apiKey, project, entity string) *WandbLogger {
	l := &WandbLogger{
		apiKey:  apiKey,
		baseURL: "https://api.wandb.ai",
		project: project,
		entity:  entity,
	}
	l.BatchLogger = NewBatchLogger(l.flush)
	return l
}

func (l *WandbLogger) flush(batch []LogData) error {
	records := make([]map[string]any, 0, len(batch))
	for _, d := range batch {
		record := map[string]any{
			"_wandb":            map[string]any{"id": uuid.New().String()},
			"model":             d.Model,
			"provider":          d.Provider,
			"latency_ms":        d.Latency.Milliseconds(),
			"prompt_tokens":     d.PromptTokens,
			"completion_tokens": d.CompletionTokens,
			"total_tokens":      d.TotalTokens,
			"cost":              d.Cost,
		}
		if d.Error != nil {
			record["error"] = d.Error.Error()
		}
		records = append(records, record)
	}

	body, _ := json.Marshal(records)

	url := fmt.Sprintf("%s/api/v1/%s/%s/logs", l.baseURL, l.entity, l.project)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+l.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("wandb: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("wandb: status %d", resp.StatusCode)
	}
	return nil
}
