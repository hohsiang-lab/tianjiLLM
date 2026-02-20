package callback

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// CloudZeroCallback sends cloud cost intelligence data to CloudZero.
type CloudZeroCallback struct {
	apiKey  string
	baseURL string
}

func NewCloudZeroCallback(apiKey, baseURL string) *CloudZeroCallback {
	if baseURL == "" {
		baseURL = "https://api.cloudzero.com"
	}
	return &CloudZeroCallback{apiKey: apiKey, baseURL: baseURL}
}

func (c *CloudZeroCallback) LogSuccess(data LogData) { c.log(data) }
func (c *CloudZeroCallback) LogFailure(data LogData) { c.log(data) }

func (c *CloudZeroCallback) log(data LogData) {
	payload := map[string]any{
		"granularity": "HOURLY",
		"telemetry_records": []map[string]any{
			{
				"timestamp": data.EndTime.Format(time.RFC3339),
				"value":     data.Cost,
				"groups": map[string]string{
					"model":    data.Model,
					"provider": data.Provider,
					"user_id":  data.UserID,
					"team_id":  data.TeamID,
				},
			},
		},
	}

	body, _ := json.Marshal(payload)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v2/telemetry", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", c.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}
