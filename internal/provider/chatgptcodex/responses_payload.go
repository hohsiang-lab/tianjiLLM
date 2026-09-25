package chatgptcodex

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// PreparedResponsesPayload is an immutable, validated Codex backend request
// snapshot. Prepare it once, then reuse it across credential attempts.
type PreparedResponsesPayload struct {
	body                  []byte
	stream                bool
	responsesLitePrepared bool
}

// PreparedCompactResponsesPayload is an immutable, validated Codex backend
// compact request snapshot. Prepare it once, then reuse it across credential
// attempts.
type PreparedCompactResponsesPayload struct {
	body                  []byte
	responsesLitePrepared bool
}

func PrepareResponsesPayload(payload any, stream bool) (PreparedResponsesPayload, error) {
	normalized, err := normalizeResponsesPayload(payload, stream)
	if err != nil {
		return PreparedResponsesPayload{}, err
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		return PreparedResponsesPayload{}, fmt.Errorf("marshal ChatGPT Codex backend responses request: %w", err)
	}
	return PreparedResponsesPayload{
		body:   data,
		stream: stream,
	}, nil
}

// Snapshot returns an owned copy of the normalized payload used to build the
// request body. Mutating the returned value cannot affect later requests or
// snapshots.
func (p PreparedResponsesPayload) Snapshot() any {
	return snapshotPreparedPayload(p.body)
}

func PrepareCompactResponsesPayload(payload any) (PreparedCompactResponsesPayload, error) {
	normalized, err := normalizeCompactResponsesPayload(payload)
	if err != nil {
		return PreparedCompactResponsesPayload{}, err
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		return PreparedCompactResponsesPayload{}, fmt.Errorf("marshal ChatGPT Codex backend compact request: %w", err)
	}
	return PreparedCompactResponsesPayload{body: data}, nil
}

// Snapshot returns an owned copy of the normalized compact payload used to
// build the request body.
func (p PreparedCompactResponsesPayload) Snapshot() any {
	return snapshotPreparedPayload(p.body)
}

func snapshotPreparedPayload(body []byte) any {
	if len(body) == 0 {
		return nil
	}
	var snapshot any
	if err := json.Unmarshal(body, &snapshot); err != nil {
		return nil
	}
	return snapshot
}

func normalizeResponsesLitePayload(body []byte) ([]byte, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return body, nil
	}
	if trimmed[0] != '{' {
		var value any
		if err := json.Unmarshal(body, &value); err != nil {
			return nil, fmt.Errorf("decode ChatGPT Codex Responses Lite request: %w", err)
		}
		return body, nil
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode ChatGPT Codex Responses Lite request: %w", err)
	}
	if payload == nil {
		return body, nil
	}

	input, err := normalizeResponsesLiteInput(payload["input"])
	if err != nil {
		return nil, err
	}
	tools, err := normalizeResponsesLiteTools(payload["tools"])
	if err != nil {
		return nil, err
	}

	additionalTools, err := marshalResponsesLiteItem(map[string]any{
		"type":  "additional_tools",
		"role":  "developer",
		"tools": tools,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal ChatGPT Codex Responses Lite additional_tools: %w", err)
	}
	prefix := []json.RawMessage{additionalTools}
	if instructions, ok := payload["instructions"]; ok {
		var text string
		if unmarshalErr := json.Unmarshal(instructions, &text); unmarshalErr != nil {
			return nil, fmt.Errorf("decode ChatGPT Codex Responses Lite instructions: %w", unmarshalErr)
		}
		if strings.TrimSpace(text) != "" {
			developerMessage, marshalErr := marshalResponsesLiteItem(map[string]any{
				"type": "message",
				"role": "developer",
				"content": []map[string]any{{
					"type": "input_text",
					"text": text,
				}},
			})
			if marshalErr != nil {
				return nil, fmt.Errorf("marshal ChatGPT Codex Responses Lite instructions item: %w", marshalErr)
			}
			prefix = append(prefix, developerMessage)
		}
	}
	var normalizedInput []byte
	normalizedInput, err = json.Marshal(append(prefix, input...))
	if err != nil {
		return nil, fmt.Errorf("marshal ChatGPT Codex Responses Lite input: %w", err)
	}
	payload["input"] = json.RawMessage(normalizedInput)
	delete(payload, "instructions")
	delete(payload, "tools")
	payload["tool_choice"] = json.RawMessage(`"auto"`)
	payload["parallel_tool_calls"] = json.RawMessage(`false`)

	reasoning := make(map[string]json.RawMessage)
	if rawReasoning, ok := payload["reasoning"]; ok && !bytes.Equal(bytes.TrimSpace(rawReasoning), []byte("null")) {
		if unmarshalErr := json.Unmarshal(rawReasoning, &reasoning); unmarshalErr != nil || reasoning == nil {
			reasoning = make(map[string]json.RawMessage)
		}
	}
	reasoning["context"] = json.RawMessage(`"all_turns"`)
	var normalizedReasoning []byte
	normalizedReasoning, err = json.Marshal(reasoning)
	if err != nil {
		return nil, fmt.Errorf("marshal ChatGPT Codex Responses Lite reasoning: %w", err)
	}
	payload["reasoning"] = json.RawMessage(normalizedReasoning)
	return json.Marshal(payload)
}

func marshalResponsesLiteItem(value any) (json.RawMessage, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func normalizeResponsesLiteInput(raw json.RawMessage) ([]json.RawMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil
	}
	var input []json.RawMessage
	if err := json.Unmarshal(raw, &input); err == nil {
		return input, nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		item, err := marshalResponsesLiteItem(map[string]any{
			"type": "message",
			"role": "user",
			"content": []map[string]any{{
				"type": "input_text",
				"text": text,
			}},
		})
		if err != nil {
			return nil, fmt.Errorf("marshal ChatGPT Codex Responses Lite input item: %w", err)
		}
		return []json.RawMessage{item}, nil
	}
	return nil, fmt.Errorf("decode ChatGPT Codex Responses Lite input: expected an array or string")
}

func normalizeResponsesLiteTools(raw json.RawMessage) ([]json.RawMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return []json.RawMessage{}, nil
	}
	var tools []json.RawMessage
	if err := json.Unmarshal(raw, &tools); err != nil {
		return nil, fmt.Errorf("decode ChatGPT Codex Responses Lite tools: expected an array")
	}

	var functions []json.RawMessage
	functionsIndex := -1
	functionsDescription := ""
	normalized := make([]json.RawMessage, 0, len(tools))
	for _, rawTool := range tools {
		var tool map[string]json.RawMessage
		if err := json.Unmarshal(rawTool, &tool); err != nil {
			return nil, fmt.Errorf("decode ChatGPT Codex Responses Lite tool: %w", err)
		}
		var toolType string
		_ = json.Unmarshal(tool["type"], &toolType)
		switch toolType {
		case "function", "custom":
			if functionsIndex < 0 {
				functionsIndex = len(normalized)
			}
			functions = append(functions, rawTool)
		case "namespace":
			var name string
			_ = json.Unmarshal(tool["name"], &name)
			if name == "functions" {
				if functionsIndex < 0 {
					functionsIndex = len(normalized)
				}
				var description string
				if err := json.Unmarshal(tool["description"], &description); err == nil && strings.TrimSpace(description) != "" {
					functionsDescription = description
				}
				var namespaceTools []json.RawMessage
				if err := json.Unmarshal(tool["tools"], &namespaceTools); err != nil && len(bytes.TrimSpace(tool["tools"])) != 0 {
					return nil, fmt.Errorf("decode ChatGPT Codex Responses Lite functions namespace tools: expected an array")
				}
				for _, namespaceTool := range namespaceTools {
					var child map[string]json.RawMessage
					if err := json.Unmarshal(namespaceTool, &child); err != nil {
						return nil, fmt.Errorf("decode ChatGPT Codex Responses Lite functions namespace tool: %w", err)
					}
					var childType string
					_ = json.Unmarshal(child["type"], &childType)
					if childType == "function" || childType == "custom" {
						functions = append(functions, namespaceTool)
					}
				}
				continue
			}
			normalized = append(normalized, rawTool)
		case "tool_search":
			var execution string
			if err := json.Unmarshal(tool["execution"], &execution); err == nil && execution == "client" {
				normalized = append(normalized, rawTool)
			}
		}
	}
	if len(functions) > 0 {
		namespace := map[string]any{
			"type":        "namespace",
			"name":        "functions",
			"description": functionsDescription,
			"tools":       functions,
		}
		normalized = append(normalized, nil)
		copy(normalized[functionsIndex+1:], normalized[functionsIndex:])
		normalizedNamespace, err := marshalResponsesLiteItem(namespace)
		if err != nil {
			return nil, fmt.Errorf("marshal ChatGPT Codex Responses Lite functions namespace: %w", err)
		}
		normalized[functionsIndex] = normalizedNamespace
	}
	return normalized, nil
}

func normalizeCompactResponsesPayload(payload any) (any, error) {
	normalized, err := normalizeResponsesPayload(payload, false)
	if err != nil {
		return nil, err
	}
	if raw, ok := normalized.(map[string]any); ok {
		delete(raw, "stream")
		delete(raw, "store")
	}
	return normalized, nil
}

func normalizeResponsesPayload(payload any, stream bool) (any, error) {
	raw, ok := payload.(map[string]any)
	if !ok {
		return cloneResponsesJSONValue(payload), nil
	}

	normalized := cloneResponsesMap(raw)
	if modelName, ok := normalized["model"].(string); ok {
		normalized["model"] = normalizeModel(modelName)
	}
	if input, ok := normalized["input"].(string); ok {
		normalized["input"] = normalizeStringResponsesInput(input)
	} else if input, ok := normalized["input"]; ok {
		normalizedInput, err := normalizeResponsesInput(input)
		if err != nil {
			return nil, err
		}
		normalized["input"] = normalizedInput
	}
	if _, ok := normalized["store"]; !ok {
		normalized["store"] = false
	}
	if store, _ := normalized["store"].(bool); !store {
		stripResponseItemIDs(normalized["input"])
	}
	if stream {
		normalized["stream"] = true
	}
	return normalized, nil
}

func cloneResponsesMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	clone := make(map[string]any, len(input))
	for key, value := range input {
		clone[key] = cloneResponsesJSONValue(value)
	}
	return clone
}

func cloneResponsesJSONValue(value any) any {
	cloned := cloneResponsesReflectValue(reflect.ValueOf(value))
	if !cloned.IsValid() {
		return nil
	}
	return cloned.Interface()
}

func cloneResponsesReflectValue(value reflect.Value) reflect.Value {
	if !value.IsValid() {
		return value
	}
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := reflect.New(value.Type()).Elem()
		cloned.Set(cloneResponsesReflectValue(value.Elem()))
		return cloned
	case reflect.Map:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := reflect.MakeMapWithSize(value.Type(), value.Len())
		iter := value.MapRange()
		for iter.Next() {
			cloned.SetMapIndex(iter.Key(), cloneResponsesReflectValue(iter.Value()))
		}
		return cloned
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for i := 0; i < value.Len(); i++ {
			cloned.Index(i).Set(cloneResponsesReflectValue(value.Index(i)))
		}
		return cloned
	case reflect.Array:
		cloned := reflect.New(value.Type()).Elem()
		for i := 0; i < value.Len(); i++ {
			cloned.Index(i).Set(cloneResponsesReflectValue(value.Index(i)))
		}
		return cloned
	default:
		return value
	}
}

func stripResponseItemIDs(input any) {
	// With store:false, replayed response items are inlined and their
	// server-issued top-level IDs are not resolvable by the Codex backend.
	switch items := input.(type) {
	case []any:
		for _, item := range items {
			if mapped, ok := item.(map[string]any); ok {
				delete(mapped, "id")
			}
		}
	case []map[string]any:
		for _, item := range items {
			delete(item, "id")
		}
	}
}

func normalizeStringResponsesInput(input string) []map[string]any {
	return []map[string]any{{
		"type": "message",
		"role": "user",
		"content": []map[string]any{{
			"type": "input_text",
			"text": input,
		}},
	}}
}

func normalizeResponsesInput(input any) (any, error) {
	switch messages := input.(type) {
	case []any:
		normalized := make([]any, 0, len(messages))
		for i, message := range messages {
			item, err := normalizeResponsesMessage(message, fmt.Sprintf("input[%d]", i))
			if err != nil {
				return nil, err
			}
			normalized = append(normalized, item)
		}
		return normalized, nil
	case []map[string]any:
		normalized := make([]map[string]any, 0, len(messages))
		for i, message := range messages {
			item, err := normalizeResponsesMessageMap(message, fmt.Sprintf("input[%d]", i))
			if err != nil {
				return nil, err
			}
			normalized = append(normalized, item)
		}
		return normalized, nil
	default:
		return input, nil
	}
}

func normalizeResponsesMessage(message any, path string) (any, error) {
	raw, ok := message.(map[string]any)
	if !ok {
		return message, nil
	}
	return normalizeResponsesMessageMap(raw, path)
}

func normalizeResponsesMessageMap(raw map[string]any, path string) (map[string]any, error) {
	if _, hasImageURL := raw["image_url"]; hasImageURL {
		return normalizeResponsesOutputItemMap(raw, path)
	}
	normalized := make(map[string]any, len(raw))
	for key, value := range raw {
		normalized[key] = value
	}
	if content, ok := normalized["content"]; ok {
		normalizedContent, err := normalizeResponsesContent(content)
		if err != nil {
			return nil, err
		}
		normalized["content"] = normalizedContent
	}
	if output, ok := normalized["output"]; ok {
		normalizedOutput, err := normalizeResponsesOutput(output, path+".output")
		if err != nil {
			return nil, err
		}
		normalized["output"] = normalizedOutput
	}
	return normalized, nil
}

func normalizeResponsesOutput(output any, path string) (any, error) {
	switch items := output.(type) {
	case []any:
		normalized := make([]any, 0, len(items))
		for i, item := range items {
			mapped, err := normalizeResponsesOutputItem(item, fmt.Sprintf("%s[%d]", path, i))
			if err != nil {
				return nil, err
			}
			normalized = append(normalized, mapped)
		}
		return normalized, nil
	case []map[string]any:
		normalized := make([]map[string]any, 0, len(items))
		for i, item := range items {
			mapped, err := normalizeResponsesOutputItemMap(item, fmt.Sprintf("%s[%d]", path, i))
			if err != nil {
				return nil, err
			}
			normalized = append(normalized, mapped)
		}
		return normalized, nil
	default:
		return output, nil
	}
}

func normalizeResponsesOutputItem(item any, path string) (any, error) {
	raw, ok := item.(map[string]any)
	if !ok {
		return item, nil
	}
	return normalizeResponsesOutputItemMap(raw, path)
}

func normalizeResponsesOutputItemMap(raw map[string]any, path string) (map[string]any, error) {
	normalized := make(map[string]any, len(raw)+1)
	for key, value := range raw {
		normalized[key] = value
	}
	if _, hasImageURL := raw["image_url"]; !hasImageURL {
		return normalized, nil
	}
	url, detail, err := imageURLStringAndDetail(raw)
	if err != nil {
		return nil, unsupportedPayloadError(path + ".image_url")
	}
	if !isImageDataURL(url) {
		return nil, unsupportedPayloadError(path + ".image_url must be a data:image/...;base64,... URL")
	}
	normalized["type"] = "input_image"
	normalized["image_url"] = url
	if detail != nil {
		normalized["detail"] = *detail
	}
	return normalized, nil
}

func isImageDataURL(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(lower, "data:image/") && strings.Contains(lower, ";base64,")
}

func normalizeResponsesContent(content any) (any, error) {
	switch parts := content.(type) {
	case []any:
		normalized := make([]any, 0, len(parts))
		for _, part := range parts {
			item, err := normalizeResponsesContentPart(part)
			if err != nil {
				return nil, err
			}
			normalized = append(normalized, item)
		}
		return normalized, nil
	case []map[string]any:
		normalized := make([]map[string]any, 0, len(parts))
		for _, part := range parts {
			item, err := normalizeResponsesContentPartMap(part)
			if err != nil {
				return nil, err
			}
			normalized = append(normalized, item)
		}
		return normalized, nil
	default:
		return content, nil
	}
}

func normalizeResponsesContentPart(part any) (any, error) {
	raw, ok := part.(map[string]any)
	if !ok {
		return part, nil
	}
	return normalizeResponsesContentPartMap(raw)
}

func normalizeResponsesContentPartMap(raw map[string]any) (map[string]any, error) {
	normalized := make(map[string]any, len(raw)+1)
	for key, value := range raw {
		normalized[key] = value
	}
	switch raw["type"] {
	case "input_image", "image_url":
		if _, ok := raw["image_url"]; !ok {
			if fileID, ok := raw["file_id"].(string); ok && strings.TrimSpace(fileID) != "" {
				normalized["type"] = "input_image"
				return normalized, nil
			}
		}
		url, detail, err := imageURLStringAndDetail(raw)
		if err != nil {
			return nil, err
		}
		normalized["type"] = "input_image"
		normalized["image_url"] = url
		if detail != nil {
			normalized["detail"] = *detail
		}
	}
	return normalized, nil
}

func normalizeModel(modelName string) string {
	for _, prefix := range []string{"openai/", "chatgpt/"} {
		modelName = strings.TrimPrefix(modelName, prefix)
	}
	return modelName
}
