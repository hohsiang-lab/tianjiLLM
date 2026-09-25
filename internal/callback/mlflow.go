package callback

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// MLflowLogger sends log data to an MLflow tracking server.
// Lifecycle: create run → log batch → update run.
type MLflowLogger struct {
	*BatchLogger
	trackingURI  string
	experimentID string
}

// NewMLflowLogger creates an MLflow callback.
func NewMLflowLogger(trackingURI, experimentID string) *MLflowLogger {
	l := &MLflowLogger{
		trackingURI:  trackingURI,
		experimentID: experimentID,
	}
	l.BatchLogger = NewBatchLogger(l.flush)
	return l
}

func (l *MLflowLogger) flush(batch []LogData) error {
	for _, d := range batch {
		runID, err := l.createRun(d)
		if err != nil {
			return err
		}
		if err := l.logMetrics(runID, d); err != nil {
			return err
		}
		if err := l.endRun(runID); err != nil {
			return err
		}
	}
	return nil
}

func (l *MLflowLogger) createRun(d LogData) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"experiment_id": l.experimentID,
		"start_time":    d.StartTime.UnixMilli(),
		"tags": []map[string]string{
			{"key": "model", "value": d.Model},
			{"key": "provider", "value": d.Provider},
		},
	})

	resp, err := l.post("/api/2.0/mlflow/runs/create", body)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Run struct {
			Info struct {
				RunID string `json:"run_id"`
			} `json:"info"`
		} `json:"run"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&result)
	return result.Run.Info.RunID, nil
}

func (l *MLflowLogger) logMetrics(runID string, d LogData) error {
	ts := d.EndTime.UnixMilli()
	body, _ := json.Marshal(map[string]any{
		"run_id": runID,
		"metrics": []map[string]any{
			{"key": "latency_ms", "value": d.Latency.Milliseconds(), "timestamp": ts, "step": 0},
			{"key": "prompt_tokens", "value": d.PromptTokens, "timestamp": ts, "step": 0},
			{"key": "completion_tokens", "value": d.CompletionTokens, "timestamp": ts, "step": 0},
			{"key": "total_tokens", "value": d.TotalTokens, "timestamp": ts, "step": 0},
			{"key": "cost", "value": d.Cost, "timestamp": ts, "step": 0},
		},
	})

	resp, err := l.post("/api/2.0/mlflow/runs/log-batch", body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (l *MLflowLogger) endRun(runID string) error {
	body, _ := json.Marshal(map[string]any{
		"run_id":   runID,
		"status":   "FINISHED",
		"end_time": time.Now().UnixMilli(),
	})

	resp, err := l.post("/api/2.0/mlflow/runs/update", body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (l *MLflowLogger) post(path string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, l.trackingURI+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mlflow: %w", err)
	}
	if resp.StatusCode >= 400 {
		resp.Body.Close()
		return nil, fmt.Errorf("mlflow %s: status %d", path, resp.StatusCode)
	}
	return resp, nil
}
