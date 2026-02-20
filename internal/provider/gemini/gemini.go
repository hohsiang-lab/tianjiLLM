package gemini

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

const defaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"

// Provider implements the Google Gemini/Vertex AI translation layer.
type Provider struct {
	baseURL   string
	isVertex  bool
	projectID string
	location  string
}

func New() *Provider {
	return &Provider{baseURL: defaultBaseURL}
}

func NewVertex(projectID, location string) *Provider {
	return &Provider{
		baseURL:   fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1", location),
		isVertex:  true,
		projectID: projectID,
		location:  location,
	}
}

func (p *Provider) TransformRequest(ctx context.Context, req *model.ChatCompletionRequest, apiKey string) (*http.Request, error) {
	body := p.transformRequestBody(req)

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal gemini request: %w", err)
	}

	method := "generateContent"
	if req.IsStreaming() {
		method = "streamGenerateContent?alt=sse"
	}

	url := p.buildURL(req.Model, method, apiKey)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create gemini request: %w", err)
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
		return nil, fmt.Errorf("read gemini response: %w", err)
	}

	var geminiResp geminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return nil, fmt.Errorf("parse gemini response: %w", err)
	}

	return transformToOpenAI(&geminiResp), nil
}

func (p *Provider) TransformStreamChunk(_ context.Context, data []byte) (*model.StreamChunk, bool, error) {
	return ParseStreamChunk(data)
}

func (p *Provider) GetSupportedParams() []string {
	return []string{
		"model", "messages", "temperature", "max_tokens", "top_p",
		"top_k", "stop", "stream", "tools", "tool_choice",
		"response_format",
	}
}

func (p *Provider) MapParams(params map[string]any) map[string]any {
	result := make(map[string]any, len(params))
	for k, v := range params {
		switch k {
		case "max_tokens", "max_completion_tokens":
			result["maxOutputTokens"] = v
		case "temperature":
			result["temperature"] = v
		case "top_p":
			result["topP"] = v
		case "top_k":
			result["topK"] = v
		case "stop":
			result["stopSequences"] = v
		default:
			result[k] = v
		}
	}
	return result
}

func (p *Provider) GetRequestURL(modelName string) string {
	return p.buildURL(modelName, "generateContent", "")
}

func (p *Provider) SetupHeaders(req *http.Request, apiKey string) {
	req.Header.Set("Content-Type", "application/json")
	if p.isVertex {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	// For Gemini API, the key is in the URL query parameter
}

func (p *Provider) buildURL(modelName, method, apiKey string) string {
	if p.isVertex {
		return fmt.Sprintf("%s/projects/%s/locations/%s/publishers/google/models/%s:%s",
			p.baseURL, p.projectID, p.location, modelName, method)
	}
	url := fmt.Sprintf("%s/models/%s:%s", p.baseURL, modelName, method)
	if apiKey != "" && !strings.Contains(url, "key=") {
		url += "?key=" + apiKey
	}
	return url
}

func (p *Provider) transformRequestBody(req *model.ChatCompletionRequest) map[string]any {
	contents := transformMessages(req.Messages)

	body := map[string]any{
		"contents": contents,
	}

	// Generation config
	genConfig := map[string]any{}
	if req.Temperature != nil {
		genConfig["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil {
		genConfig["maxOutputTokens"] = *req.MaxTokens
	}
	if req.TopP != nil {
		genConfig["topP"] = *req.TopP
	}
	if req.Stop != nil {
		genConfig["stopSequences"] = req.Stop
	}
	if req.ResponseFormat != nil {
		if rf, ok := req.ResponseFormat.(map[string]any); ok {
			if rf["type"] == "json_object" {
				genConfig["responseMimeType"] = "application/json"
			}
		}
	}
	if len(genConfig) > 0 {
		body["generationConfig"] = genConfig
	}

	// Tools
	if len(req.Tools) > 0 {
		body["tools"] = []map[string]any{
			{"functionDeclarations": transformTools(req.Tools)},
		}
	}

	return body
}

func transformMessages(messages []model.Message) []map[string]any {
	var contents []map[string]any

	for _, msg := range messages {
		role := mapRole(msg.Role)

		// System messages become a separate system_instruction
		if msg.Role == "system" {
			continue // handled separately if needed
		}

		parts := transformContent(msg)

		if len(parts) > 0 {
			contents = append(contents, map[string]any{
				"role":  role,
				"parts": parts,
			})
		}
	}

	return contents
}

func mapRole(role string) string {
	switch role {
	case "assistant":
		return "model"
	case "user", "system":
		return "user"
	case "tool":
		return "user"
	default:
		return role
	}
}

func transformContent(msg model.Message) []map[string]any {
	// Handle tool results
	if msg.ToolCallID != nil {
		contentStr, _ := msg.Content.(string)
		return []map[string]any{
			{
				"functionResponse": map[string]any{
					"name": msg.ToolCallID,
					"response": map[string]any{
						"content": contentStr,
					},
				},
			},
		}
	}

	// Handle tool calls from assistant
	if len(msg.ToolCalls) > 0 {
		var parts []map[string]any
		for _, tc := range msg.ToolCalls {
			var args any
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
			parts = append(parts, map[string]any{
				"functionCall": map[string]any{
					"name": tc.Function.Name,
					"args": args,
				},
			})
		}
		return parts
	}

	switch content := msg.Content.(type) {
	case string:
		return []map[string]any{{"text": content}}
	case []any:
		var parts []map[string]any
		for _, part := range content {
			if m, ok := part.(map[string]any); ok {
				parts = append(parts, transformContentPart(m))
			}
		}
		return parts
	default:
		return []map[string]any{{"text": fmt.Sprintf("%v", content)}}
	}
}

func transformContentPart(part map[string]any) map[string]any {
	partType, _ := part["type"].(string)
	switch partType {
	case "text":
		text, _ := part["text"].(string)
		return map[string]any{"text": text}
	case "image_url":
		if imageURL, ok := part["image_url"].(map[string]any); ok {
			url, _ := imageURL["url"].(string)
			return map[string]any{
				"inlineData": map[string]any{
					"mimeType": "image/jpeg",
					"data":     url,
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
			"name":        tool.Function.Name,
			"description": tool.Function.Description,
			"parameters":  tool.Function.Parameters,
		})
	}
	return result
}

// Gemini response types

type geminiResponse struct {
	Candidates    []geminiCandidate `json:"candidates"`
	UsageMetadata *geminiUsage      `json:"usageMetadata"`
}

type geminiCandidate struct {
	Content       geminiContent `json:"content"`
	FinishReason  string        `json:"finishReason"`
	SafetyRatings []any         `json:"safetyRatings"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
	Role  string       `json:"role"`
}

type geminiPart struct {
	Text         string          `json:"text,omitempty"`
	FunctionCall *geminiFuncCall `json:"functionCall,omitempty"`
}

type geminiFuncCall struct {
	Name string `json:"name"`
	Args any    `json:"args"`
}

type geminiUsage struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

func transformToOpenAI(resp *geminiResponse) *model.ModelResponse {
	var choices []model.Choice

	for i, candidate := range resp.Candidates {
		var textContent strings.Builder
		var toolCalls []model.ToolCall
		toolIndex := 0

		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				textContent.WriteString(part.Text)
			}
			if part.FunctionCall != nil {
				args, _ := json.Marshal(part.FunctionCall.Args)
				toolCalls = append(toolCalls, model.ToolCall{
					ID:   fmt.Sprintf("call_%d", toolIndex),
					Type: "function",
					Function: model.ToolCallFunction{
						Name:      part.FunctionCall.Name,
						Arguments: string(args),
					},
					Index: &toolIndex,
				})
				toolIndex++
			}
		}

		finishReason := mapFinishReason(candidate.FinishReason)
		msg := &model.Message{
			Role:    "assistant",
			Content: textContent.String(),
		}
		if len(toolCalls) > 0 {
			msg.ToolCalls = toolCalls
		}

		choices = append(choices, model.Choice{
			Index:        i,
			Message:      msg,
			FinishReason: &finishReason,
		})
	}

	result := &model.ModelResponse{
		Object:  "chat.completion",
		Choices: choices,
	}

	if resp.UsageMetadata != nil {
		result.Usage = model.Usage{
			PromptTokens:     resp.UsageMetadata.PromptTokenCount,
			CompletionTokens: resp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      resp.UsageMetadata.TotalTokenCount,
		}
	}

	return result
}

func mapFinishReason(reason string) string {
	switch reason {
	case "STOP":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "SAFETY":
		return "content_filter"
	case "TOOL_CALLS":
		return "tool_calls"
	default:
		return reason
	}
}

func parseErrorResponse(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	msg := string(body)
	var errResp struct {
		Error struct {
			Message string `json:"message"`
			Status  string `json:"status"`
			Code    int    `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &errResp) == nil && errResp.Error.Message != "" {
		msg = errResp.Error.Message
	}

	return &model.TianjiError{
		StatusCode: resp.StatusCode,
		Message:    msg,
		Type:       "api_error",
		Provider:   "gemini",
		Err:        model.MapHTTPStatusToError(resp.StatusCode),
	}
}

func init() {
	provider.Register("gemini", New())
}
