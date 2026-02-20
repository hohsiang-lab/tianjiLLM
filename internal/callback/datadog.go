package callback

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// DatadogCallback sends LLM events to Datadog.
type DatadogCallback struct {
	apiKey string
	site   string
	client *http.Client
}

// NewDatadogCallback creates a Datadog logging callback.
func NewDatadogCallback(apiKey, site string) *DatadogCallback {
	if site == "" {
		site = "datadoghq.com"
	}
	return &DatadogCallback{
		apiKey: apiKey,
		site:   site,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

func (d *DatadogCallback) LogSuccess(data LogData) {
	d.sendLog(data, "info", "llm.completion.success")
}

func (d *DatadogCallback) LogFailure(data LogData) {
	d.sendLog(data, "error", "llm.completion.failure")
}

func (d *DatadogCallback) sendLog(data LogData, status, source string) {
	errMsg := ""
	if data.Error != nil {
		errMsg = data.Error.Error()
	}

	entry := []map[string]any{
		{
			"ddsource": "tianji",
			"ddtags":   "model:" + data.Model + ",provider:" + data.Provider,
			"hostname": "tianji-proxy",
			"service":  "tianji",
			"status":   status,
			"message":  source,
			"attributes": map[string]any{
				"model":             data.Model,
				"provider":          data.Provider,
				"prompt_tokens":     data.PromptTokens,
				"completion_tokens": data.CompletionTokens,
				"total_tokens":      data.TotalTokens,
				"cost":              data.Cost,
				"latency_ms":        data.Latency.Milliseconds(),
				"user_id":           data.UserID,
				"team_id":           data.TeamID,
				"cache_hit":         data.CacheHit,
				"error":             errMsg,
			},
		},
	}

	body, err := json.Marshal(entry)
	if err != nil {
		return
	}

	url := "https://http-intake.logs." + d.site + "/api/v2/logs"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("DD-API-KEY", d.apiKey)

	resp, err := d.client.Do(req)
	if err != nil {
		log.Printf("warn: datadog log failed: %v", err)
		return
	}
	resp.Body.Close()
}
