package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
)

const (
	defaultBaseURL   = "https://api.anthropic.com"
	messagesEndpoint = "/v1/messages"
	apiVersion       = "2023-06-01"
)

// Provider implements the Anthropic translation layer.
type Provider struct {
	baseURL string
}

func New() *Provider {
	return &Provider{baseURL: defaultBaseURL}
}

func NewWithBaseURL(baseURL string) *Provider {
	return &Provider{baseURL: baseURL}
}

func (p *Provider) TransformRequest(ctx context.Context, req *model.ChatCompletionRequest, apiKey string) (*http.Request, error) {
	body := p.transformRequestBody(req)

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal anthropic request: %w", err)
	}

	url := p.GetRequestURL(req.Model)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create anthropic request: %w", err)
	}

	p.SetupHeaders(httpReq, apiKey)
	return httpReq, nil
}

func (p *Provider) TransformResponse(_ context.Context, resp *http.Response) (*model.ModelResponse, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseErrorResponse(resp)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read anthropic response: %w", err)
	}

	var anthropicResp anthropicResponse
	if err := json.Unmarshal(body, &anthropicResp); err != nil {
		return nil, fmt.Errorf("parse anthropic response: %w", err)
	}

	return transformToOpenAI(&anthropicResp), nil
}

func (p *Provider) TransformStreamChunk(_ context.Context, data []byte) (*model.StreamChunk, bool, error) {
	return ParseStreamEvent(data)
}

func (p *Provider) GetSupportedParams() []string {
	return []string{
		"model", "messages", "max_tokens", "temperature", "top_p",
		"stop", "stream", "tools", "tool_choice", "system",
		"top_k", "metadata",
	}
}

func (p *Provider) MapParams(params map[string]any) map[string]any {
	result := make(map[string]any, len(params))
	for k, v := range params {
		switch k {
		case "max_tokens", "max_completion_tokens":
			result["max_tokens"] = v
		default:
			result[k] = v
		}
	}
	return result
}

func (p *Provider) GetRequestURL(_ string) string {
	return p.baseURL + messagesEndpoint
}

func (p *Provider) SetupHeaders(req *http.Request, apiKey string) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("anthropic-version", apiVersion)

	if IsOAuthToken(apiKey) {
		SetOAuthHeaders(req, apiKey)
		return
	}
	req.Header.Set("x-api-key", apiKey)
}

// transformRequestBody converts OpenAI format to Anthropic format.
func (p *Provider) transformRequestBody(req *model.ChatCompletionRequest) map[string]any {
	// Separate system messages
	var systemParts []map[string]any
	var messages []map[string]any

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			contentStr, ok := msg.Content.(string)
			if ok && contentStr != "" {
				systemParts = append(systemParts, map[string]any{
					"type": "text",
					"text": contentStr,
				})
			}
			continue
		}

		anthropicMsg := transformMessage(msg)
		if anthropicMsg != nil {
			messages = append(messages, anthropicMsg)
		}
	}

	body := map[string]any{
		"model":    req.Model,
		"messages": messages,
	}

	if len(systemParts) > 0 {
		body["system"] = systemParts
	}

	maxTokens := 4096
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	}
	body["max_tokens"] = maxTokens

	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		body["top_p"] = *req.TopP
	}
	if req.Stop != nil {
		body["stop_sequences"] = req.Stop
	}
	if req.Stream != nil {
		body["stream"] = *req.Stream
	}

	if len(req.Tools) > 0 {
		body["tools"] = transformTools(req.Tools)
	}
	if req.ToolChoice != nil {
		body["tool_choice"] = transformToolChoice(req.ToolChoice)
	}

	return body
}

func transformMessage(msg model.Message) map[string]any {
	result := map[string]any{
		"role": msg.Role,
	}

	switch content := msg.Content.(type) {
	case string:
		result["content"] = content
	case []any:
		var parts []map[string]any
		for _, part := range content {
			if m, ok := part.(map[string]any); ok {
				parts = append(parts, transformContentPart(m))
			}
		}
		result["content"] = parts
	default:
		result["content"] = msg.Content
	}

	// Handle tool results
	if msg.ToolCallID != nil {
		result["role"] = "user"
		result["content"] = []map[string]any{
			{
				"type":        "tool_result",
				"tool_use_id": *msg.ToolCallID,
				"content":     msg.Content,
			},
		}
	}

	// Handle assistant tool calls
	if len(msg.ToolCalls) > 0 {
		var content []map[string]any
		// Include any text content first
		if s, ok := msg.Content.(string); ok && s != "" {
			content = append(content, map[string]any{
				"type": "text",
				"text": s,
			})
		}
		for _, tc := range msg.ToolCalls {
			var input any
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
				input = map[string]any{}
			}
			content = append(content, map[string]any{
				"type":  "tool_use",
				"id":    tc.ID,
				"name":  tc.Function.Name,
				"input": input,
			})
		}
		result["content"] = content
	}

	return result
}

func transformContentPart(part map[string]any) map[string]any {
	partType, _ := part["type"].(string)
	switch partType {
	case "text":
		return part
	case "image_url":
		if imageURL, ok := part["image_url"].(map[string]any); ok {
			url, _ := imageURL["url"].(string)
			return map[string]any{
				"type": "image",
				"source": map[string]any{
					"type": "url",
					"url":  url,
				},
			}
		}
	}
	return part
}

func transformTools(tools []model.Tool) []map[string]any {
	result := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		result = append(result, map[string]any{
			"name":         tool.Function.Name,
			"description":  tool.Function.Description,
			"input_schema": tool.Function.Parameters,
		})
	}
	return result
}

func transformToolChoice(choice any) map[string]any {
	switch v := choice.(type) {
	case string:
		switch v {
		case "auto":
			return map[string]any{"type": "auto"}
		case "required":
			return map[string]any{"type": "any"}
		case "none":
			return map[string]any{"type": "none"}
		}
	case map[string]any:
		if fn, ok := v["function"].(map[string]any); ok {
			name, _ := fn["name"].(string)
			return map[string]any{"type": "tool", "name": name}
		}
	}
	return map[string]any{"type": "auto"}
}

// Anthropic response types

type anthropicResponse struct {
	ID           string             `json:"id"`
	Type         string             `json:"type"`
	Role         string             `json:"role"`
	Content      []anthropicContent `json:"content"`
	Model        string             `json:"model"`
	StopReason   *string            `json:"stop_reason"`
	StopSequence *string            `json:"stop_sequence"`
	Usage        anthropicUsage     `json:"usage"`
}

type anthropicContent struct {
	Type  string `json:"type"`
	Text  string `json:"text,omitempty"`
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Input any    `json:"input,omitempty"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

func transformToOpenAI(resp *anthropicResponse) *model.ModelResponse {
	// Map stop_reason to finish_reason
	var finishReason *string
	if resp.StopReason != nil {
		fr := mapStopReason(*resp.StopReason)
		finishReason = &fr
	}

	// Build message content and tool calls
	var textContent string
	var toolCalls []model.ToolCall
	toolIndex := 0

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			textContent += block.Text
		case "tool_use":
			args, _ := json.Marshal(block.Input)
			toolCalls = append(toolCalls, model.ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: model.ToolCallFunction{
					Name:      block.Name,
					Arguments: string(args),
				},
				Index: &toolIndex,
			})
			toolIndex++
		}
	}

	msg := &model.Message{
		Role:    "assistant",
		Content: textContent,
	}
	if len(toolCalls) > 0 {
		msg.ToolCalls = toolCalls
	}

	return &model.ModelResponse{
		ID:      resp.ID,
		Object:  "chat.completion",
		Created: 0, // Anthropic doesn't return created timestamp
		Model:   resp.Model,
		Choices: []model.Choice{
			{
				Index:        0,
				Message:      msg,
				FinishReason: finishReason,
			},
		},
		Usage: model.Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}
}

func mapStopReason(reason string) string {
	switch reason {
	case "end_turn":
		return "stop"
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	case "stop_sequence":
		return "stop"
	default:
		return reason
	}
}

func parseErrorResponse(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	var errResp struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}

	msg := string(body)
	errType := "api_error"
	if json.Unmarshal(body, &errResp) == nil && errResp.Error.Message != "" {
		msg = errResp.Error.Message
		errType = errResp.Error.Type
	}

	return &model.TianjiError{
		StatusCode: resp.StatusCode,
		Message:    msg,
		Type:       errType,
		Provider:   "anthropic",
		Err:        model.MapHTTPStatusToError(resp.StatusCode),
	}
}

func init() {
	provider.Register("anthropic", New())
}
