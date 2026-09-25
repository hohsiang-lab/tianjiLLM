package callback

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// LangfuseCallback sends LLM events to Langfuse for observability.
type LangfuseCallback struct {
	host      string
	publicKey string
	secretKey string
	client    *http.Client
}

// NewLangfuseCallback creates a Langfuse logging callback.
func NewLangfuseCallback(host, publicKey, secretKey string) *LangfuseCallback {
	if host == "" {
		host = "https://cloud.langfuse.com"
	}
	return &LangfuseCallback{
		host:      host,
		publicKey: publicKey,
		secretKey: secretKey,
		client:    &http.Client{Timeout: 5 * time.Second},
	}
}

func (l *LangfuseCallback) LogSuccess(data LogData) {
	l.ingest(data, "GENERATION", "SUCCESS")
}

func (l *LangfuseCallback) LogFailure(data LogData) {
	l.ingest(data, "GENERATION", "ERROR")
}

func (l *LangfuseCallback) ingest(data LogData, _, level string) {
	event := map[string]any{
		"batch": []map[string]any{
			{
				"type":      "generation-create",
				"timestamp": data.EndTime.UTC().Format(time.RFC3339Nano),
				"body": map[string]any{
					"name":                data.Model,
					"model":               data.Model,
					"startTime":           data.StartTime.UTC().Format(time.RFC3339Nano),
					"endTime":             data.EndTime.UTC().Format(time.RFC3339Nano),
					"level":               level,
					"completionStartTime": data.StartTime.Add(data.Latency - data.LLMAPILatency).UTC().Format(time.RFC3339Nano),
					"usage": map[string]any{
						"input":      data.PromptTokens,
						"output":     data.CompletionTokens,
						"total":      data.TotalTokens,
						"totalCost":  data.Cost,
						"inputCost":  data.Cost * float64(data.PromptTokens) / max(float64(data.TotalTokens), 1),
						"outputCost": data.Cost * float64(data.CompletionTokens) / max(float64(data.TotalTokens), 1),
					},
					"metadata": map[string]any{
						"provider":     data.Provider,
						"user_id":      data.UserID,
						"team_id":      data.TeamID,
						"cache_hit":    data.CacheHit,
						"request_tags": data.RequestTags,
					},
				},
			},
		},
	}

	body, err := json.Marshal(event)
	if err != nil {
		return
	}

	req, err := http.NewRequest(http.MethodPost, l.host+"/api/public/ingestion", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString(
		[]byte(l.publicKey+":"+l.secretKey)))

	resp, err := l.client.Do(req)
	if err != nil {
		log.Printf("warn: langfuse ingest failed: %v", err)
		return
	}
	resp.Body.Close()
}
