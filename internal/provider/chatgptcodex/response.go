package chatgptcodex

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/security/redact"
)

func transformCodexResponseBody(body []byte, fallbackModel string) (*model.ModelResponse, error) {
	var result codexResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse codex response: %w", err)
	}

	content := collectOutputText(result.Output)
	refusal := collectOutputRefusal(result.Output)
	toolCalls := collectOutputToolCalls(result.Output)
	if content == "" && refusal == "" && len(toolCalls) == 0 {
		return nil, fmt.Errorf("codex response has no assistant output_text")
	}

	finishReason := "stop"
	message := &model.Message{Role: "assistant"}
	if content != "" {
		message.Content = content
	}
	if refusal != "" {
		message.Refusal = &refusal
	}
	if len(toolCalls) > 0 {
		message.ToolCalls = toolCalls
		finishReason = "tool_calls"
	}
	created := result.CreatedAt
	if created == 0 {
		created = time.Now().Unix()
	}
	totalTokens := result.Usage.TotalTokens
	if totalTokens == 0 {
		totalTokens = result.Usage.InputTokens + result.Usage.OutputTokens
	}
	responseModel := result.Model
	if responseModel == "" {
		responseModel = normalizeModel(fallbackModel)
	}
	id := result.ID
	if id == "" {
		id = "chatcmpl-codex"
	}

	return &model.ModelResponse{
		ID:      id,
		Object:  "chat.completion",
		Created: created,
		Model:   responseModel,
		Choices: []model.Choice{{
			Index:        0,
			Message:      message,
			FinishReason: &finishReason,
		}},
		Usage: model.Usage{
			PromptTokens:     result.Usage.InputTokens,
			CompletionTokens: result.Usage.OutputTokens,
			TotalTokens:      totalTokens,
		},
	}, nil
}

var doneMarker = []byte("[DONE]")

type StreamTranslator struct {
	toolIndexes   map[int]int
	textOutput    strings.Builder
	refusalOutput strings.Builder
	toolOutputs   map[int]*streamedToolOutput
}

type streamedToolOutput struct {
	announced bool
	id        string
	name      string
	arguments strings.Builder
}

func TransformStreamChunk(data []byte, fallbackModel string) (*model.StreamChunk, bool, error) {
	var translator StreamTranslator
	return translator.Transform(data, fallbackModel)
}

func (t *StreamTranslator) Transform(data []byte, fallbackModel string) (*model.StreamChunk, bool, error) {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, doneMarker) {
		return nil, true, nil
	}

	var event codexStreamEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, false, fmt.Errorf("parse codex stream event: %w", err)
	}

	switch event.Type {
	case "response.output_text.delta":
		if event.Delta == "" {
			return nil, false, nil
		}
		t.textOutput.WriteString(event.Delta)
		return newCodexDeltaChunk(
			event.responseID(),
			event.modelName(fallbackModel),
			model.Delta{Content: &event.Delta},
		), false, nil
	case "response.refusal.delta":
		if event.Delta == "" {
			return nil, false, nil
		}
		t.refusalOutput.WriteString(event.Delta)
		return newCodexDeltaChunk(
			event.responseID(),
			event.modelName(fallbackModel),
			model.Delta{Refusal: &event.Delta},
		), false, nil
	case "response.output_item.added":
		if event.Item.Type != "function_call" {
			return nil, false, nil
		}
		output := t.toolOutput(event.OutputIndex)
		output.announced = true
		index := t.toolIndex(event.OutputIndex)
		callID := event.Item.CallID
		if callID == "" {
			callID = event.Item.ID
		}
		output.id = callID
		output.name = event.Item.Name
		output.arguments.WriteString(event.Item.Arguments)
		return newCodexDeltaChunk(
			event.responseID(),
			event.modelName(fallbackModel),
			model.Delta{ToolCalls: []model.ToolCall{{
				ID:    callID,
				Type:  "function",
				Index: &index,
				Function: model.ToolCallFunction{
					Name:      event.Item.Name,
					Arguments: event.Item.Arguments,
				},
			}}},
		), false, nil
	case "response.function_call_arguments.delta":
		if event.Delta == "" {
			return nil, false, nil
		}
		t.toolOutput(event.OutputIndex).arguments.WriteString(event.Delta)
		index := t.toolIndex(event.OutputIndex)
		return newCodexDeltaChunk(
			event.responseID(),
			event.modelName(fallbackModel),
			model.Delta{ToolCalls: []model.ToolCall{{
				Index: &index,
				Function: model.ToolCallFunction{
					Arguments: event.Delta,
				},
			}}},
		), false, nil
	case "response.function_call_arguments.done":
		output := t.toolOutput(event.OutputIndex)
		name := unstreamedSuffix(event.Name, output.name)
		arguments := unstreamedSuffix(event.Arguments, output.arguments.String())
		if event.Name != "" {
			output.name = event.Name
		}
		output.arguments.Reset()
		output.arguments.WriteString(event.Arguments)
		if name == "" && arguments == "" {
			return nil, false, nil
		}
		index := t.toolIndex(event.OutputIndex)
		return newCodexDeltaChunk(
			event.responseID(),
			event.modelName(fallbackModel),
			model.Delta{ToolCalls: []model.ToolCall{{
				Index: &index,
				Function: model.ToolCallFunction{
					Name:      name,
					Arguments: arguments,
				},
			}}},
		), false, nil
	case "response.output_item.done":
		if event.Item.Type != "function_call" {
			return nil, false, nil
		}
		output := t.toolOutput(event.OutputIndex)
		index := t.toolIndex(event.OutputIndex)
		callID := event.Item.CallID
		if callID == "" {
			callID = event.Item.ID
		}
		fragment := model.ToolCall{Index: &index}
		if !output.announced {
			fragment.ID = callID
			fragment.Type = "function"
		} else if callID != "" && callID != output.id {
			fragment.ID = callID
		}
		fragment.Function.Name = unstreamedSuffix(event.Item.Name, output.name)
		fragment.Function.Arguments = unstreamedSuffix(event.Item.Arguments, output.arguments.String())
		output.announced = true
		if callID != "" {
			output.id = callID
		}
		if event.Item.Name != "" {
			output.name = event.Item.Name
		}
		output.arguments.Reset()
		output.arguments.WriteString(event.Item.Arguments)
		if fragment.ID == "" && fragment.Type == "" && fragment.Function.Name == "" && fragment.Function.Arguments == "" {
			return nil, false, nil
		}
		return newCodexDeltaChunk(
			event.responseID(),
			event.modelName(fallbackModel),
			model.Delta{ToolCalls: []model.ToolCall{fragment}},
		), false, nil
	case "response.refusal.done":
		refusal := unstreamedSuffix(event.Refusal, t.refusalOutput.String())
		t.refusalOutput.Reset()
		t.refusalOutput.WriteString(event.Refusal)
		if refusal == "" {
			return nil, false, nil
		}
		return newCodexDeltaChunk(
			event.responseID(),
			event.modelName(fallbackModel),
			model.Delta{Refusal: &refusal},
		), false, nil
	case "response.completed":
		usage := event.responseUsage()
		return t.completedChunk(event, fallbackModel, usage), true, nil
	case "response.failed", "response.incomplete":
		return nil, true, event.failureError()
	default:
		return nil, false, nil
	}
}

func (t *StreamTranslator) completedChunk(event codexStreamEvent, fallbackModel string, usage model.Usage) *model.StreamChunk {
	var delta model.Delta
	if content := unstreamedSuffix(collectOutputText(event.Response.Output), t.textOutput.String()); content != "" {
		delta.Content = &content
	}
	if refusal := unstreamedSuffix(collectOutputRefusal(event.Response.Output), t.refusalOutput.String()); refusal != "" {
		delta.Refusal = &refusal
	}
	for outputIndex, output := range event.Response.Output {
		if output.Type != "function_call" {
			continue
		}
		streamed := t.toolOutputs[outputIndex]
		index := t.toolIndex(outputIndex)
		callID := output.CallID
		if callID == "" {
			callID = output.ID
		}
		fragment := model.ToolCall{Index: &index}
		if streamed == nil || !streamed.announced {
			fragment.ID = callID
			fragment.Type = "function"
		} else if callID != "" && callID != streamed.id {
			fragment.ID = callID
		}
		if streamed == nil {
			fragment.Function.Name = output.Name
			fragment.Function.Arguments = output.Arguments
		} else {
			fragment.Function.Name = unstreamedSuffix(output.Name, streamed.name)
			fragment.Function.Arguments = unstreamedSuffix(output.Arguments, streamed.arguments.String())
		}
		if fragment.ID != "" || fragment.Type != "" || fragment.Function.Name != "" || fragment.Function.Arguments != "" {
			delta.ToolCalls = append(delta.ToolCalls, fragment)
		}
	}
	if delta.Content == nil && delta.Refusal == nil && len(delta.ToolCalls) == 0 {
		return newCodexUsageChunk(event.responseID(), event.modelName(fallbackModel), usage)
	}

	reason := "stop"
	if len(delta.ToolCalls) > 0 || len(t.toolIndexes) > 0 {
		reason = "tool_calls"
	}
	chunk := newCodexDeltaChunk(event.responseID(), event.modelName(fallbackModel), delta)
	chunk.Choices[0].FinishReason = &reason
	if usage != (model.Usage{}) {
		chunk.Usage = &usage
	}
	return chunk
}

func (t *StreamTranslator) toolOutput(outputIndex int) *streamedToolOutput {
	if t.toolOutputs == nil {
		t.toolOutputs = make(map[int]*streamedToolOutput)
	}
	if t.toolOutputs[outputIndex] == nil {
		t.toolOutputs[outputIndex] = &streamedToolOutput{}
	}
	return t.toolOutputs[outputIndex]
}

func (t *StreamTranslator) toolIndex(outputIndex int) int {
	if index, ok := t.toolIndexes[outputIndex]; ok {
		return index
	}
	if t.toolIndexes == nil {
		t.toolIndexes = make(map[int]int)
	}
	index := len(t.toolIndexes)
	t.toolIndexes[outputIndex] = index
	return index
}

func unstreamedSuffix(full, streamed string) string {
	if full == "" || full == streamed {
		return ""
	}
	if strings.HasPrefix(full, streamed) {
		return full[len(streamed):]
	}
	return ""
}

type codexResponse struct {
	ID        string        `json:"id"`
	CreatedAt int64         `json:"created_at"`
	Model     string        `json:"model"`
	Output    []codexOutput `json:"output"`
	Usage     codexUsage    `json:"usage"`
	Error     *codexError   `json:"error"`
}

type codexOutput struct {
	Type      string               `json:"type"`
	ID        string               `json:"id"`
	CallID    string               `json:"call_id"`
	Name      string               `json:"name"`
	Arguments string               `json:"arguments"`
	Role      string               `json:"role"`
	Content   []codexOutputContent `json:"content"`
}

type codexOutputContent struct {
	Type    string `json:"type"`
	Text    string `json:"text"`
	Refusal string `json:"refusal"`
}

type codexUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

type codexError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

type codexStreamEvent struct {
	Type        string        `json:"type"`
	ResponseID  string        `json:"response_id"`
	Delta       string        `json:"delta"`
	Name        string        `json:"name"`
	Arguments   string        `json:"arguments"`
	Refusal     string        `json:"refusal"`
	Model       string        `json:"model"`
	OutputIndex int           `json:"output_index"`
	Item        codexOutput   `json:"item"`
	Response    codexResponse `json:"response"`
	Usage       codexUsage    `json:"usage"`
}

func (e codexStreamEvent) responseID() string {
	if e.Response.ID != "" {
		return e.Response.ID
	}
	if e.ResponseID != "" {
		return e.ResponseID
	}
	return "chatcmpl-codex"
}

func (e codexStreamEvent) modelName(fallbackModel string) string {
	if e.Response.Model != "" {
		return e.Response.Model
	}
	if e.Model != "" {
		return e.Model
	}
	return normalizeModel(fallbackModel)
}

func (e codexStreamEvent) responseUsage() model.Usage {
	usage := e.Response.Usage
	if usage == (codexUsage{}) {
		usage = e.Usage
	}
	totalTokens := usage.TotalTokens
	if totalTokens == 0 {
		totalTokens = usage.InputTokens + usage.OutputTokens
	}
	return model.Usage{
		PromptTokens:     usage.InputTokens,
		CompletionTokens: usage.OutputTokens,
		TotalTokens:      totalTokens,
	}
}

func (e codexStreamEvent) failureError() error {
	msg := e.Type
	errType := "api_error"
	code := ""
	if e.Response.Error != nil {
		if e.Response.Error.Message != "" {
			msg = e.Response.Error.Message
		}
		if e.Response.Error.Type != "" {
			errType = e.Response.Error.Type
		}
		code = e.Response.Error.Code
	}
	return newCodexTianjiError(http.StatusBadGateway, msg, errType, code)
}

func newCodexTianjiError(statusCode int, message, errType, code string) *model.TianjiError {
	return &model.TianjiError{
		StatusCode: statusCode,
		Message:    message,
		Type:       errType,
		Code:       code,
		Provider:   "chatgpt_codex_backend",
		Err:        model.MapHTTPStatusToError(statusCode),
	}
}

func newCodexDeltaChunk(id, modelName string, delta model.Delta) *model.StreamChunk {
	return &model.StreamChunk{
		ID:      id,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   modelName,
		Choices: []model.StreamChoice{{
			Index: 0,
			Delta: delta,
		}},
	}
}

func newCodexUsageChunk(id, modelName string, usage model.Usage) *model.StreamChunk {
	chunk := &model.StreamChunk{
		ID:      id,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   modelName,
		Choices: []model.StreamChoice{},
	}
	if usage != (model.Usage{}) {
		chunk.Usage = &usage
	}
	return chunk
}

func collectOutputText(outputs []codexOutput) string {
	var b strings.Builder
	for _, output := range outputs {
		if output.Role != "" && output.Role != "assistant" {
			continue
		}
		for _, content := range output.Content {
			if content.Type == "output_text" {
				b.WriteString(content.Text)
			}
		}
	}
	return b.String()
}

func collectOutputRefusal(outputs []codexOutput) string {
	var b strings.Builder
	for _, output := range outputs {
		if output.Role != "" && output.Role != "assistant" {
			continue
		}
		for _, content := range output.Content {
			if content.Type == "refusal" {
				b.WriteString(content.Refusal)
			}
		}
	}
	return b.String()
}

func collectOutputToolCalls(outputs []codexOutput) []model.ToolCall {
	var calls []model.ToolCall
	for _, output := range outputs {
		if output.Type != "function_call" {
			continue
		}
		callID := output.CallID
		if callID == "" {
			callID = output.ID
		}
		calls = append(calls, model.ToolCall{
			ID:   callID,
			Type: "function",
			Function: model.ToolCallFunction{
				Name:      output.Name,
				Arguments: output.Arguments,
			},
		})
	}
	return calls
}

func parseCodexErrorResponse(resp *http.Response, providerName string) error {
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return &model.TianjiError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("upstream error (status %d): failed to read response body: %v", resp.StatusCode, readErr),
			Type:       "api_error",
			Provider:   providerName,
			Err:        model.MapHTTPStatusToError(resp.StatusCode),
		}
	}

	var errResp struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}

	msg := string(redact.RawJSON(body))
	errType := "api_error"
	code := ""
	if json.Unmarshal(body, &errResp) == nil && errResp.Error.Message != "" {
		msg = redact.String(errResp.Error.Message)
		errType = errResp.Error.Type
		code = errResp.Error.Code
	}

	return &model.TianjiError{
		StatusCode: resp.StatusCode,
		Message:    msg,
		Type:       errType,
		Code:       code,
		Provider:   providerName,
		Err:        model.MapHTTPStatusToError(resp.StatusCode),
	}
}

func ParseErrorResponse(resp *http.Response) error {
	return parseCodexErrorResponse(resp, "chatgpt_codex_backend")
}
