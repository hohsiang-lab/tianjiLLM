package handler

import (
	"strings"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

func reasoningEffortFromPayload(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	if effort, ok := payload["reasoning_effort"].(string); ok {
		if effort = strings.TrimSpace(effort); effort != "" {
			return effort
		}
	}
	reasoning, ok := payload["reasoning"].(map[string]any)
	if !ok {
		return ""
	}
	effort, _ := reasoning["effort"].(string)
	return strings.TrimSpace(effort)
}

func reasoningEffortFromChatRequest(req *model.ChatCompletionRequest) string {
	if req == nil {
		return ""
	}
	return reasoningEffortFromPayload(req.ExtraParams)
}
