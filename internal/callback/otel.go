package callback

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// OTelCallback sends trace data to an OpenTelemetry collector via OTLP HTTP.
type OTelCallback struct {
	endpoint string
	client   *http.Client
	headers  map[string]string
}

// NewOTelCallback creates an OpenTelemetry trace callback.
// endpoint should be the OTLP HTTP endpoint (e.g., "http://localhost:4318/v1/traces").
func NewOTelCallback(endpoint string, headers map[string]string) *OTelCallback {
	return &OTelCallback{
		endpoint: endpoint,
		client:   &http.Client{Timeout: 5 * time.Second},
		headers:  headers,
	}
}

func (o *OTelCallback) LogSuccess(data LogData) {
	o.sendSpan(data, "success")
}

func (o *OTelCallback) LogFailure(data LogData) {
	o.sendSpan(data, "error")
}

func (o *OTelCallback) sendSpan(data LogData, status string) {
	span := map[string]any{
		"resourceSpans": []map[string]any{
			{
				"resource": map[string]any{
					"attributes": []map[string]any{
						{"key": "service.name", "value": map[string]string{"stringValue": "tianji-proxy"}},
					},
				},
				"scopeSpans": []map[string]any{
					{
						"scope": map[string]any{"name": "tianji"},
						"spans": []map[string]any{
							{
								"name":              "llm.completion",
								"startTimeUnixNano": data.StartTime.UnixNano(),
								"endTimeUnixNano":   data.EndTime.UnixNano(),
								"status":            map[string]any{"code": statusCode(status)},
								"attributes": []map[string]any{
									{"key": "llm.model", "value": map[string]string{"stringValue": data.Model}},
									{"key": "llm.provider", "value": map[string]string{"stringValue": data.Provider}},
									{"key": "llm.prompt_tokens", "value": map[string]int{"intValue": data.PromptTokens}},
									{"key": "llm.completion_tokens", "value": map[string]int{"intValue": data.CompletionTokens}},
									{"key": "llm.cost", "value": map[string]float64{"doubleValue": data.Cost}},
								},
							},
						},
					},
				},
			},
		},
	}

	body, err := json.Marshal(span)
	if err != nil {
		return
	}

	req, err := http.NewRequest(http.MethodPost, o.endpoint, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range o.headers {
		req.Header.Set(k, v)
	}

	resp, err := o.client.Do(req)
	if err != nil {
		log.Printf("warn: otel export failed: %v", err)
		return
	}
	resp.Body.Close()
}

func statusCode(s string) int {
	if s == "error" {
		return 2
	}
	return 1 // OK
}
