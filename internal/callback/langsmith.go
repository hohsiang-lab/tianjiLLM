package callback

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// LangsmithLogger sends batched traces to Langsmith API.
type LangsmithLogger struct {
	*BatchLogger
	apiKey  string
	baseURL string
	project string
}

// NewLangsmithLogger creates a Langsmith callback.
func NewLangsmithLogger(apiKey, baseURL, project string) *LangsmithLogger {
	if baseURL == "" {
		baseURL = "https://api.smith.langchain.com"
	}
	if project == "" {
		project = "default"
	}
	l := &LangsmithLogger{
		apiKey:  apiKey,
		baseURL: baseURL,
		project: project,
	}
	l.BatchLogger = NewBatchLogger(l.flush)
	return l
}

func (l *LangsmithLogger) flush(batch []LogData) error {
	runs := make([]map[string]any, 0, len(batch))
	for _, d := range batch {
		run := map[string]any{
			"id":         uuid.New().String(),
			"name":       d.Model,
			"run_type":   "llm",
			"start_time": d.StartTime.Format(time.RFC3339Nano),
			"end_time":   d.EndTime.Format(time.RFC3339Nano),
			"extra": map[string]any{
				"provider":          d.Provider,
				"prompt_tokens":     d.PromptTokens,
				"completion_tokens": d.CompletionTokens,
				"total_tokens":      d.TotalTokens,
				"cost":              d.Cost,
			},
			"session_name": l.project,
		}
		if d.Error != nil {
			run["error"] = d.Error.Error()
			run["status"] = "error"
		} else {
			run["status"] = "completed"
		}
		runs = append(runs, run)
	}

	body, _ := json.Marshal(map[string]any{"post": runs})
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, l.baseURL+"/runs/batch", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", l.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("langsmith: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("langsmith: status %d", resp.StatusCode)
	}
	return nil
}
