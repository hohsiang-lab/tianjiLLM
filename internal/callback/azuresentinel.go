package callback

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// AzureSentinelCallback sends SIEM log data to Azure Sentinel (Log Analytics).
type AzureSentinelCallback struct {
	workspaceID string
	sharedKey   string
	baseURL     string
}

func NewAzureSentinelCallback(workspaceID, sharedKey, baseURL string) *AzureSentinelCallback {
	if baseURL == "" && workspaceID != "" {
		baseURL = "https://" + workspaceID + ".ods.opinsights.azure.com"
	}
	return &AzureSentinelCallback{workspaceID: workspaceID, sharedKey: sharedKey, baseURL: baseURL}
}

func (c *AzureSentinelCallback) LogSuccess(data LogData) { c.log(data) }
func (c *AzureSentinelCallback) LogFailure(data LogData) { c.log(data) }

func (c *AzureSentinelCallback) log(data LogData) {
	record := map[string]any{
		"model":             data.Model,
		"provider":          data.Provider,
		"prompt_tokens":     data.PromptTokens,
		"completion_tokens": data.CompletionTokens,
		"total_tokens":      data.TotalTokens,
		"cost":              data.Cost,
		"latency_ms":        data.Latency.Milliseconds(),
		"user_id":           data.UserID,
		"team_id":           data.TeamID,
		"tags":              data.RequestTags,
		"timestamp":         data.EndTime.Format(time.RFC3339),
	}
	if data.Error != nil {
		record["error"] = data.Error.Error()
	}

	body, _ := json.Marshal([]any{record})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/logs?api-version=2016-04-01", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Log-Type", "TianjiLLM_CL")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}
