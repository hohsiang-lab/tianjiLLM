package handler

import (
	"errors"
	"fmt"
	"strings"
)

type responsesWebSocketState struct {
	responsePayloads map[string]map[string]any
	nextSyntheticID  int
}

var errPreviousResponseStateNotFound = errors.New("previous response state not found on this WebSocket connection")

func isGenerateFalse(payload map[string]any) bool {
	generate, ok := payload["generate"].(bool)
	return ok && !generate
}

func (s *responsesWebSocketState) storeSyntheticResponse(payload map[string]any) string {
	s.nextSyntheticID++
	responseID := fmt.Sprintf("resp_tianji_prewarm_%d", s.nextSyntheticID)
	s.storeResponse(responseID, payload)
	return responseID
}

func (s *responsesWebSocketState) storeResponse(responseID string, payload map[string]any) {
	if strings.TrimSpace(responseID) == "" || payload == nil {
		return
	}
	if s.responsePayloads == nil {
		s.responsePayloads = make(map[string]map[string]any)
	}
	s.responsePayloads[responseID] = cloneMap(payload)
}

func (s *responsesWebSocketState) expandPayload(payload map[string]any) (map[string]any, error) {
	previousResponseID, _ := payload["previous_response_id"].(string)
	if strings.TrimSpace(previousResponseID) == "" {
		return cloneMap(payload), nil
	}
	previousPayload, ok := s.responsePayloads[previousResponseID]
	if !ok || previousPayload == nil {
		return nil, errPreviousResponseStateNotFound
	}

	expanded := cloneMap(previousPayload)
	for key, value := range payload {
		if key == "previous_response_id" || key == "generate" {
			continue
		}
		if key == "input" {
			if inputIsEmpty(value) {
				continue
			}
			expanded[key] = mergeResponseInputs(expanded["input"], cloneJSONValue(value))
			continue
		}
		expanded[key] = cloneJSONValue(value)
	}
	delete(expanded, "previous_response_id")
	delete(expanded, "generate")
	return expanded, nil
}

func cloneMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	clone := make(map[string]any, len(input))
	for key, value := range input {
		clone[key] = cloneJSONValue(value)
	}
	return clone
}

func cloneJSONValue(value any) any {
	switch value := value.(type) {
	case map[string]any:
		return cloneMap(value)
	case []any:
		clone := make([]any, len(value))
		for i, item := range value {
			clone[i] = cloneJSONValue(item)
		}
		return clone
	case []map[string]any:
		clone := make([]map[string]any, len(value))
		for i, item := range value {
			clone[i] = cloneMap(item)
		}
		return clone
	default:
		return value
	}
}

func inputIsEmpty(input any) bool {
	if input == nil {
		return true
	}
	items, ok := input.([]any)
	return ok && len(items) == 0
}

func mergeResponseInputs(base, incremental any) any {
	baseItems, baseOK := base.([]any)
	incrementalItems, incrementalOK := incremental.([]any)
	if !baseOK || !incrementalOK {
		return cloneJSONValue(incremental)
	}
	merged := make([]any, 0, len(baseItems)+len(incrementalItems))
	for _, item := range baseItems {
		merged = append(merged, cloneJSONValue(item))
	}
	for _, item := range incrementalItems {
		merged = append(merged, cloneJSONValue(item))
	}
	return merged
}

func responseConversationPayload(payload map[string]any, completedResponse map[string]any, completedOutputItems []any) map[string]any {
	conversation := cloneMap(payload)
	output, ok := completedResponse["output"].([]any)
	if !ok || len(output) == 0 {
		output = completedOutputItems
	}
	if len(output) == 0 {
		return conversation
	}
	conversation["input"] = mergeResponseInputs(conversation["input"], output)
	return conversation
}
