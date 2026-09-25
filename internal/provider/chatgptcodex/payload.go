package chatgptcodex

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

type Payload struct {
	Model           string           `json:"model"`
	Store           *bool            `json:"store"`
	Stream          *bool            `json:"stream,omitempty"`
	Instructions    string           `json:"instructions,omitempty"`
	Input           []InputMessage   `json:"input"`
	Text            map[string]any   `json:"text,omitempty"`
	Tools           []map[string]any `json:"tools,omitempty"`
	ToolChoice      any              `json:"tool_choice,omitempty"`
	Temperature     *float64         `json:"temperature,omitempty"`
	TopP            *float64         `json:"top_p,omitempty"`
	MaxOutputTokens *int             `json:"max_output_tokens,omitempty"`
	Metadata        map[string]any   `json:"metadata,omitempty"`
}

type InputMessage struct {
	Type      string        `json:"type"`
	Role      string        `json:"role,omitempty"`
	Content   []ContentPart `json:"content,omitempty"`
	CallID    string        `json:"call_id,omitempty"`
	Name      string        `json:"name,omitempty"`
	Arguments *string       `json:"arguments,omitempty"`
	Output    *string       `json:"output,omitempty"`
}

type ContentPart struct {
	Type     string  `json:"type"`
	Text     string  `json:"text,omitempty"`
	Refusal  string  `json:"refusal,omitempty"`
	ImageURL string  `json:"image_url,omitempty"`
	Detail   *string `json:"detail,omitempty"`
}

func BuildPayload(req *model.ChatCompletionRequest) (Payload, error) {
	if err := validateSupported(req); err != nil {
		return Payload{}, err
	}

	modelName := normalizeModel(req.Model)
	payload := Payload{
		Model:  modelName,
		Store:  ptrBool(false),
		Stream: req.Stream,
		Input:  make([]InputMessage, 0, len(req.Messages)),
		TopP:   req.TopP,
	}
	if !omitsUpstreamUnsupportedSampling(modelName) {
		payload.Temperature = req.Temperature
		if req.MaxCompletionTokens != nil {
			payload.MaxOutputTokens = req.MaxCompletionTokens
		} else {
			payload.MaxOutputTokens = req.MaxTokens
		}
	}
	if req.ResponseFormat != nil {
		format, err := mapResponseFormat(req.ResponseFormat)
		if err != nil {
			return Payload{}, err
		}
		if format != nil {
			payload.Text = map[string]any{"format": format}
		}
	}
	if len(req.Tools) > 0 {
		tools, err := mapTools(req.Tools)
		if err != nil {
			return Payload{}, err
		}
		payload.Tools = tools
	}
	if req.ToolChoice != nil {
		choice, err := mapToolChoice(req.ToolChoice)
		if err != nil {
			return Payload{}, err
		}
		payload.ToolChoice = choice
	}
	if len(req.Metadata) > 0 || req.User != nil {
		payload.Metadata = make(map[string]any, len(req.Metadata)+1)
		for k, v := range req.Metadata {
			payload.Metadata[k] = v
		}
		if req.User != nil && strings.TrimSpace(*req.User) != "" {
			payload.Metadata["user"] = *req.User
		}
	}

	var instructions []string
	inConversation := false
	for _, msg := range req.Messages {
		if !inConversation && isInstructionRole(msg.Role) {
			text, err := contentText(msg.Content)
			if err != nil {
				return Payload{}, unsupportedPayloadError(fmt.Sprintf("%s message content", msg.Role))
			}
			if strings.TrimSpace(text) != "" {
				instructions = append(instructions, msg.Role+": "+text)
			}
			continue
		}
		inConversation = true
		name := ""
		if msg.Name != nil {
			name = *msg.Name
		}

		if msg.Role == "tool" {
			output, err := contentText(msg.Content)
			if err != nil {
				return Payload{}, unsupportedPayloadError("tool message content")
			}
			payload.Input = append(payload.Input, InputMessage{
				Type:   "function_call_output",
				CallID: *msg.ToolCallID,
				Output: &output,
			})
			continue
		}

		if len(msg.ToolCalls) > 0 {
			if msg.Content != nil || msg.Refusal != nil {
				parts, err := mapMessageContentParts(msg)
				if err != nil {
					return Payload{}, err
				}
				payload.Input = append(payload.Input, InputMessage{
					Type:    "message",
					Role:    msg.Role,
					Content: parts,
					Name:    name,
				})
			}
			for _, call := range msg.ToolCalls {
				arguments := call.Function.Arguments
				payload.Input = append(payload.Input, InputMessage{
					Type:      "function_call",
					CallID:    call.ID,
					Name:      call.Function.Name,
					Arguments: &arguments,
				})
			}
			continue
		}

		parts, err := mapMessageContentParts(msg)
		if err != nil {
			return Payload{}, err
		}
		payload.Input = append(payload.Input, InputMessage{
			Type:    "message",
			Role:    msg.Role,
			Content: parts,
			Name:    name,
		})
	}
	payload.Instructions = strings.Join(instructions, "\n")
	return payload, nil
}

func omitsUpstreamUnsupportedSampling(modelName string) bool {
	// capability_scoped_drop: Graphiti sends max_tokens and temperature, while
	// these exact reasoning models reject the translated upstream fields.
	switch modelName {
	case "gpt-5.4-mini", "gpt-5.6-luna":
		return true
	default:
		return false
	}
}

func mapMessageContentParts(msg model.Message) ([]ContentPart, error) {
	var parts []ContentPart
	if msg.Content != nil {
		mapped, err := mapContentParts(msg.Content)
		if err != nil {
			return nil, err
		}
		parts = append(parts, mapped...)
	}
	if msg.Refusal != nil {
		parts = append(parts, ContentPart{Type: "refusal", Refusal: *msg.Refusal})
	}
	if len(parts) == 0 {
		return nil, unsupportedPayloadError("message content")
	}
	return parts, nil
}

func validateSupported(req *model.ChatCompletionRequest) error {
	switch {
	case req.N != nil && *req.N > 1:
		return unsupportedPayloadError("n > 1")
	case req.LogProbs != nil && *req.LogProbs:
		return unsupportedPayloadError("logprobs")
	case req.TopLogProbs != nil:
		return unsupportedPayloadError("top_logprobs")
	case hasStoreTrue(req.ExtraParams):
		return unsupportedPayloadError("store must be false")
	case hasUnsupportedExtraParams(req.ExtraParams):
		return unsupportedPayloadError("unknown extra parameters")
	case len(req.Modalities) > 0:
		return unsupportedPayloadError("modalities")
	case req.Stop != nil:
		return unsupportedPayloadError("stop")
	case req.Seed != nil:
		return unsupportedPayloadError("seed")
	case req.FrequencyPenalty != nil:
		return unsupportedPayloadError("frequency_penalty")
	case req.PresencePenalty != nil:
		return unsupportedPayloadError("presence_penalty")
	}
	for _, msg := range req.Messages {
		if msg.Role == "tool" {
			if msg.ToolCallID == nil || strings.TrimSpace(*msg.ToolCallID) == "" {
				return unsupportedPayloadError("tool_call_id")
			}
		} else if msg.ToolCallID != nil {
			return unsupportedPayloadError("tool_call_id")
		}
		if len(msg.ToolCalls) > 0 && msg.Role != "assistant" {
			return unsupportedPayloadError("assistant tool_calls")
		}
		for _, call := range msg.ToolCalls {
			if call.Type != "function" || strings.TrimSpace(call.ID) == "" || strings.TrimSpace(call.Function.Name) == "" {
				return unsupportedPayloadError("assistant tool_calls")
			}
		}
	}
	return nil
}

func mapResponseFormat(value any) (map[string]any, error) {
	format, ok := value.(map[string]any)
	if !ok {
		return nil, unsupportedPayloadError("response_format")
	}
	formatType, ok := format["type"].(string)
	if !ok {
		return nil, unsupportedPayloadError("response_format.type")
	}
	switch formatType {
	case "text":
		return nil, nil
	case "json_object":
		return map[string]any{"type": formatType}, nil
	case "json_schema":
		schema, ok := format["json_schema"].(map[string]any)
		if !ok {
			return nil, unsupportedPayloadError("response_format.json_schema")
		}
		mapped := make(map[string]any, len(schema)+1)
		mapped["type"] = "json_schema"
		for key, value := range schema {
			mapped[key] = value
		}
		return mapped, nil
	default:
		return nil, unsupportedPayloadError("response_format.type")
	}
}

func mapTools(tools []model.Tool) ([]map[string]any, error) {
	mapped := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		if tool.Type != "function" || strings.TrimSpace(tool.Function.Name) == "" {
			return nil, unsupportedPayloadError("tools")
		}
		item := map[string]any{
			"type": "function",
			"name": tool.Function.Name,
		}
		if tool.Function.Description != "" {
			item["description"] = tool.Function.Description
		}
		if tool.Function.Parameters != nil {
			item["parameters"] = tool.Function.Parameters
		}
		if tool.Function.Strict != nil {
			item["strict"] = *tool.Function.Strict
		}
		mapped = append(mapped, item)
	}
	return mapped, nil
}

func mapToolChoice(choice any) (any, error) {
	if value, ok := choice.(string); ok {
		switch value {
		case "auto", "required", "none":
			return value, nil
		default:
			return nil, unsupportedPayloadError("tool_choice")
		}
	}
	raw, ok := choice.(map[string]any)
	if !ok || raw["type"] != "function" {
		return nil, unsupportedPayloadError("tool_choice")
	}
	function, ok := raw["function"].(map[string]any)
	if !ok {
		return nil, unsupportedPayloadError("tool_choice.function")
	}
	name, ok := function["name"].(string)
	if !ok || strings.TrimSpace(name) == "" {
		return nil, unsupportedPayloadError("tool_choice.function.name")
	}
	return map[string]any{"type": "function", "name": name}, nil
}

func hasUnsupportedExtraParams(extra map[string]any) bool {
	for key, value := range extra {
		if key == "store" && storeParamIsFalse(value) {
			continue
		}
		return true
	}
	return false
}

func hasStoreTrue(extra map[string]any) bool {
	value, ok := extra["store"]
	return ok && storeParamIsTrue(value)
}

func storeParamIsFalse(value any) bool {
	got, ok := value.(bool)
	return ok && !got
}

func storeParamIsTrue(value any) bool {
	got, ok := value.(bool)
	return ok && got
}

func mapContentParts(content any) ([]ContentPart, error) {
	switch v := content.(type) {
	case string:
		return []ContentPart{{Type: "input_text", Text: v}}, nil
	case []model.ContentPart:
		parts := make([]ContentPart, 0, len(v))
		for _, part := range v {
			mapped, err := mapModelContentPart(part)
			if err != nil {
				return nil, err
			}
			parts = append(parts, mapped)
		}
		return parts, nil
	case []any:
		parts := make([]ContentPart, 0, len(v))
		for _, item := range v {
			part, err := mapAnyContentPart(item)
			if err != nil {
				return nil, err
			}
			parts = append(parts, part)
		}
		return parts, nil
	default:
		return nil, unsupportedPayloadError("message content")
	}
}

func mapModelContentPart(part model.ContentPart) (ContentPart, error) {
	if part.ImageURL != nil {
		if strings.TrimSpace(part.ImageURL.URL) == "" {
			return ContentPart{}, unsupportedPayloadError("image content part url")
		}
		return ContentPart{Type: "input_image", ImageURL: part.ImageURL.URL, Detail: part.ImageURL.Detail}, nil
	}
	return ContentPart{Type: "input_text", Text: part.Text}, nil
}

func mapAnyContentPart(item any) (ContentPart, error) {
	raw, ok := item.(map[string]any)
	if !ok {
		return ContentPart{}, unsupportedPayloadError("message content part")
	}
	switch raw["type"] {
	case "text", "input_text":
		text, ok := raw["text"].(string)
		if !ok {
			return ContentPart{}, unsupportedPayloadError("text content part")
		}
		return ContentPart{Type: "input_text", Text: text}, nil
	case "image_url", "input_image":
		url, detail, err := imageURLStringAndDetail(raw)
		if err != nil {
			return ContentPart{}, err
		}
		return ContentPart{Type: "input_image", ImageURL: url, Detail: detail}, nil
	default:
		return ContentPart{}, unsupportedPayloadError("message content part type")
	}
}

func imageURLStringAndDetail(raw map[string]any) (string, *string, error) {
	switch image := raw["image_url"].(type) {
	case string:
		if strings.TrimSpace(image) == "" {
			return "", nil, unsupportedPayloadError("image content part url")
		}
		return image, stringFieldPtr(raw, "detail"), nil
	case map[string]any:
		url, ok := image["url"].(string)
		if !ok || strings.TrimSpace(url) == "" {
			return "", nil, unsupportedPayloadError("image content part url")
		}
		detail := stringFieldPtr(raw, "detail")
		if detail == nil {
			detail = stringFieldPtr(image, "detail")
		}
		return url, detail, nil
	default:
		return "", nil, unsupportedPayloadError("image content part")
	}
}

func stringFieldPtr(raw map[string]any, key string) *string {
	value, ok := raw[key].(string)
	if !ok {
		return nil
	}
	return &value
}

func contentText(content any) (string, error) {
	switch v := content.(type) {
	case string:
		return v, nil
	case []model.ContentPart:
		var b strings.Builder
		for _, part := range v {
			if part.ImageURL != nil || (part.Type != "" && part.Type != "text" && part.Type != "input_text") {
				return "", unsupportedPayloadError("instruction content part")
			}
			if part.Text != "" {
				if b.Len() > 0 {
					b.WriteString("\n")
				}
				b.WriteString(part.Text)
			}
		}
		return b.String(), nil
	case []any:
		var b strings.Builder
		for _, item := range v {
			part, err := mapAnyContentPart(item)
			if err != nil {
				return "", err
			}
			if part.Type != "input_text" {
				return "", unsupportedPayloadError("instruction content part")
			}
			if part.Text != "" {
				if b.Len() > 0 {
					b.WriteString("\n")
				}
				b.WriteString(part.Text)
			}
		}
		return b.String(), nil
	default:
		return "", unsupportedPayloadError("instruction content")
	}
}

func isInstructionRole(role string) bool {
	return role == "system" || role == "developer"
}

func unsupportedPayloadError(field string) error {
	return &model.TianjiError{
		StatusCode: http.StatusBadRequest,
		Message:    "unsupported ChatGPT Codex backend payload field: " + field,
		Type:       "invalid_request_error",
		Provider:   "chatgpt_codex_backend",
		Err:        model.ErrClientValidation,
	}
}

func ptrBool(value bool) *bool {
	return &value
}
