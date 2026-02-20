package bedrock

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
)

// Provider implements the AWS Bedrock Converse API translation layer.
type Provider struct {
	region string
}

func New() *Provider {
	return &Provider{region: "us-east-1"}
}

func NewWithRegion(region string) *Provider {
	return &Provider{region: region}
}

func (p *Provider) TransformRequest(ctx context.Context, req *model.ChatCompletionRequest, apiKey string) (*http.Request, error) {
	// For Bedrock, we use the AWS SDK directly via Converse API.
	// However, for the Provider interface we still return an http.Request
	// that will be used by the handler.
	body := p.transformRequestBody(req)

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal bedrock request: %w", err)
	}

	url := p.GetRequestURL(req.Model)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create bedrock request: %w", err)
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
		return nil, fmt.Errorf("read bedrock response: %w", err)
	}

	var converseResp converseResponse
	if err := json.Unmarshal(body, &converseResp); err != nil {
		return nil, fmt.Errorf("parse bedrock response: %w", err)
	}

	return transformToOpenAI(&converseResp), nil
}

func (p *Provider) TransformStreamChunk(_ context.Context, data []byte) (*model.StreamChunk, bool, error) {
	return ParseStreamEvent(data)
}

func (p *Provider) GetSupportedParams() []string {
	return []string{
		"model", "messages", "max_tokens", "temperature", "top_p",
		"stop", "stream", "tools", "tool_choice",
	}
}

func (p *Provider) MapParams(params map[string]any) map[string]any {
	result := make(map[string]any, len(params))
	for k, v := range params {
		switch k {
		case "max_tokens", "max_completion_tokens":
			result["maxTokens"] = v
		default:
			result[k] = v
		}
	}
	return result
}

func (p *Provider) GetRequestURL(modelName string) string {
	return fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com/model/%s/converse",
		p.region, modelName)
}

func (p *Provider) SetupHeaders(req *http.Request, _ string) {
	req.Header.Set("Content-Type", "application/json")
	// SigV4 signing is handled by AWS SDK
}

// transformRequestBody converts OpenAI format to Bedrock Converse format.
func (p *Provider) transformRequestBody(req *model.ChatCompletionRequest) map[string]any {
	messages := transformMessages(req.Messages)

	body := map[string]any{
		"modelId":  req.Model,
		"messages": messages,
	}

	// Inference config
	inferenceConfig := map[string]any{}
	if req.MaxTokens != nil {
		inferenceConfig["maxTokens"] = *req.MaxTokens
	}
	if req.Temperature != nil {
		inferenceConfig["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		inferenceConfig["topP"] = *req.TopP
	}
	if req.Stop != nil {
		inferenceConfig["stopSequences"] = req.Stop
	}
	if len(inferenceConfig) > 0 {
		body["inferenceConfig"] = inferenceConfig
	}

	// System prompt
	var systemPrompts []map[string]any
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			if s, ok := msg.Content.(string); ok {
				systemPrompts = append(systemPrompts, map[string]any{
					"text": s,
				})
			}
		}
	}
	if len(systemPrompts) > 0 {
		body["system"] = systemPrompts
	}

	// Tool config
	if len(req.Tools) > 0 {
		body["toolConfig"] = map[string]any{
			"tools": transformTools(req.Tools),
		}
	}

	return body
}

func transformMessages(messages []model.Message) []map[string]any {
	var result []map[string]any

	for _, msg := range messages {
		if msg.Role == "system" {
			continue // handled separately
		}

		converseMsg := map[string]any{
			"role": mapRole(msg.Role),
		}

		content := transformContent(msg)
		if len(content) > 0 {
			converseMsg["content"] = content
		}

		result = append(result, converseMsg)
	}

	return result
}

func mapRole(role string) string {
	switch role {
	case "assistant":
		return "assistant"
	case "tool":
		return "user"
	default:
		return "user"
	}
}

func transformContent(msg model.Message) []map[string]any {
	// Handle tool results
	if msg.ToolCallID != nil {
		contentStr, _ := msg.Content.(string)
		return []map[string]any{
			{
				"toolResult": map[string]any{
					"toolUseId": *msg.ToolCallID,
					"content": []map[string]any{
						{"text": contentStr},
					},
				},
			},
		}
	}

	// Handle tool calls
	if len(msg.ToolCalls) > 0 {
		var parts []map[string]any
		for _, tc := range msg.ToolCalls {
			var input any
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)
			parts = append(parts, map[string]any{
				"toolUse": map[string]any{
					"toolUseId": tc.ID,
					"name":      tc.Function.Name,
					"input":     input,
				},
			})
		}
		return parts
	}

	switch content := msg.Content.(type) {
	case string:
		return []map[string]any{{"text": content}}
	default:
		return []map[string]any{{"text": fmt.Sprintf("%v", content)}}
	}
}

func transformTools(tools []model.Tool) []map[string]any {
	result := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		result = append(result, map[string]any{
			"toolSpec": map[string]any{
				"name":        tool.Function.Name,
				"description": tool.Function.Description,
				"inputSchema": map[string]any{
					"json": tool.Function.Parameters,
				},
			},
		})
	}
	return result
}

// Bedrock Converse response types

type converseResponse struct {
	Output     converseOutput `json:"output"`
	StopReason string         `json:"stopReason"`
	Usage      converseUsage  `json:"usage"`
}

type converseOutput struct {
	Message converseMessage `json:"message"`
}

type converseMessage struct {
	Role    string            `json:"role"`
	Content []converseContent `json:"content"`
}

type converseContent struct {
	Text    string           `json:"text,omitempty"`
	ToolUse *converseToolUse `json:"toolUse,omitempty"`
}

type converseToolUse struct {
	ToolUseID string `json:"toolUseId"`
	Name      string `json:"name"`
	Input     any    `json:"input"`
}

type converseUsage struct {
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
	TotalTokens  int `json:"totalTokens"`
}

func transformToOpenAI(resp *converseResponse) *model.ModelResponse {
	finishReason := mapStopReason(resp.StopReason)

	var textContent strings.Builder
	var toolCalls []model.ToolCall
	toolIndex := 0

	for _, block := range resp.Output.Message.Content {
		if block.Text != "" {
			textContent.WriteString(block.Text)
		}
		if block.ToolUse != nil {
			args, _ := json.Marshal(block.ToolUse.Input)
			toolCalls = append(toolCalls, model.ToolCall{
				ID:   block.ToolUse.ToolUseID,
				Type: "function",
				Function: model.ToolCallFunction{
					Name:      block.ToolUse.Name,
					Arguments: string(args),
				},
				Index: &toolIndex,
			})
			toolIndex++
		}
	}

	msg := &model.Message{
		Role:    "assistant",
		Content: textContent.String(),
	}
	if len(toolCalls) > 0 {
		msg.ToolCalls = toolCalls
	}

	return &model.ModelResponse{
		Object: "chat.completion",
		Choices: []model.Choice{
			{
				Index:        0,
				Message:      msg,
				FinishReason: &finishReason,
			},
		},
		Usage: model.Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.TotalTokens,
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

	msg := string(body)
	var errResp struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &errResp) == nil && errResp.Message != "" {
		msg = errResp.Message
	}

	return &model.TianjiError{
		StatusCode: resp.StatusCode,
		Message:    msg,
		Type:       "api_error",
		Provider:   "bedrock",
		Err:        model.MapHTTPStatusToError(resp.StatusCode),
	}
}

func init() {
	provider.Register("bedrock", New())
}
