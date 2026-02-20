package callback

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

// ArizePhoenixCallback sends trace data to Arize or Phoenix via OTLP HTTP
// using OpenInference semantic conventions.
type ArizePhoenixCallback struct {
	endpoint    string
	client      *http.Client
	headers     map[string]string
	projectName string
}

// NewArizeCallback creates an Arize-compatible OTEL callback.
func NewArizeCallback() *ArizePhoenixCallback {
	endpoint := os.Getenv("ARIZE_HTTP_ENDPOINT")
	if endpoint == "" {
		endpoint = "https://otlp.arize.com/v1/traces"
	}

	headers := map[string]string{
		"space_id": os.Getenv("ARIZE_SPACE_ID"),
		"api_key":  os.Getenv("ARIZE_API_KEY"),
	}

	project := os.Getenv("ARIZE_PROJECT_NAME")
	if project == "" {
		project = "default"
	}

	return &ArizePhoenixCallback{
		endpoint:    endpoint,
		client:      &http.Client{Timeout: 5 * time.Second},
		headers:     headers,
		projectName: project,
	}
}

// NewPhoenixCallback creates a Phoenix-compatible OTEL callback.
func NewPhoenixCallback() *ArizePhoenixCallback {
	endpoint := os.Getenv("PHOENIX_COLLECTOR_HTTP_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:6006/v1/traces"
	}

	headers := make(map[string]string)
	if apiKey := os.Getenv("PHOENIX_API_KEY"); apiKey != "" {
		headers["Authorization"] = "Bearer " + apiKey
	}

	project := os.Getenv("PHOENIX_PROJECT_NAME")
	if project == "" {
		project = "default"
	}

	return &ArizePhoenixCallback{
		endpoint:    endpoint,
		client:      &http.Client{Timeout: 5 * time.Second},
		headers:     headers,
		projectName: project,
	}
}

func (a *ArizePhoenixCallback) LogSuccess(data LogData) {
	a.sendSpan(data, "success")
}

func (a *ArizePhoenixCallback) LogFailure(data LogData) {
	a.sendSpan(data, "error")
}

func (a *ArizePhoenixCallback) sendSpan(data LogData, status string) {
	attrs := a.buildOpenInferenceAttributes(data)
	attrs = append(attrs,
		otelAttr("openinference.project.name", a.projectName),
	)

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
								"name":              "tianji_request",
								"startTimeUnixNano": data.StartTime.UnixNano(),
								"endTimeUnixNano":   data.EndTime.UnixNano(),
								"status":            map[string]any{"code": statusCode(status)},
								"attributes":        attrs,
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

	req, err := http.NewRequest(http.MethodPost, a.endpoint, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range a.headers {
		req.Header.Set(k, v)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		log.Printf("warn: arize/phoenix export failed: %v", err)
		return
	}
	resp.Body.Close()
}

func (a *ArizePhoenixCallback) buildOpenInferenceAttributes(data LogData) []map[string]any {
	attrs := []map[string]any{
		otelAttr("openinference.span.kind", "LLM"),
		otelAttr("llm.model_name", data.Model),
		otelAttr("llm.provider", data.Provider),
		otelIntAttr("llm.token_count.prompt", data.PromptTokens),
		otelIntAttr("llm.token_count.completion", data.CompletionTokens),
		otelIntAttr("llm.token_count.total", data.TotalTokens),
		otelDoubleAttr("llm.cost", data.Cost),
	}

	// Input messages
	if data.Request != nil {
		for i, msg := range data.Request.Messages {
			prefix := fmt.Sprintf("llm.input_messages.%d.message", i)
			attrs = append(attrs,
				otelAttr(prefix+".role", msg.Role),
			)
			if s, ok := msg.Content.(string); ok {
				attrs = append(attrs, otelAttr(prefix+".content", s))
			}
		}

		if data.Request.IsStreaming() {
			attrs = append(attrs, otelAttr("llm.is_streaming", "true"))
		}
	}

	// Output messages
	if data.Response != nil && len(data.Response.Choices) > 0 {
		choice := data.Response.Choices[0]
		attrs = append(attrs,
			otelAttr("llm.output_messages.0.message.role", choice.Message.Role),
		)
		if s, ok := choice.Message.Content.(string); ok {
			attrs = append(attrs, otelAttr("llm.output_messages.0.message.content", s))
			attrs = append(attrs, otelAttr("output.value", s))
		}
	}

	if data.UserID != "" {
		attrs = append(attrs, otelAttr("user.id", data.UserID))
	}

	return attrs
}

func otelAttr(key, value string) map[string]any {
	return map[string]any{"key": key, "value": map[string]string{"stringValue": value}}
}

func otelIntAttr(key string, value int) map[string]any {
	return map[string]any{"key": key, "value": map[string]int{"intValue": value}}
}

func otelDoubleAttr(key string, value float64) map[string]any {
	return map[string]any{"key": key, "value": map[string]float64{"doubleValue": value}}
}
