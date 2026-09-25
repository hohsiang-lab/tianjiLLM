package a2a

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
)

// CompletionBridge routes A2A messages through the existing chat completion handler.
type CompletionBridge struct {
	handler http.Handler // the /v1/chat/completions handler
}

// NewCompletionBridge creates a bridge that routes A2A messages to chat completions.
func NewCompletionBridge(chatHandler http.Handler) *CompletionBridge {
	return &CompletionBridge{handler: chatHandler}
}

// SendMessage converts an A2A message to a chat completion request,
// invokes the handler, and returns the result.
func (b *CompletionBridge) SendMessage(ctx context.Context, agent *AgentConfig, userMessage string) (*SendMessageResult, error) {
	if b.handler == nil {
		return nil, fmt.Errorf("no chat completion handler configured")
	}

	// Extract model from agent's tianji_params
	modelName := extractModel(agent.TianjiParams)
	if modelName == "" {
		return nil, fmt.Errorf("agent %q has no model configured", agent.AgentName)
	}

	// Build chat completion request
	chatReq := map[string]any{
		"model": modelName,
		"messages": []map[string]string{
			{"role": "user", "content": userMessage},
		},
	}

	// Add system message from agent card params if present
	if systemMsg := extractSystemMessage(agent.AgentCardParams); systemMsg != "" {
		msgs, _ := chatReq["messages"].([]map[string]string)
		chatReq["messages"] = append(
			[]map[string]string{{"role": "system", "content": systemMsg}},
			msgs...,
		)
	}

	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("marshal chat request: %w", err)
	}

	// Create internal HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Execute through the handler
	rec := httptest.NewRecorder()
	b.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		respBody, _ := io.ReadAll(rec.Body)
		return nil, fmt.Errorf("chat completion failed (%d): %s", rec.Code, string(respBody))
	}

	// Parse response
	var chatResp struct {
		Choices []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Model string `json:"model"`
		Usage any    `json:"usage"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("decode chat response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("empty response from chat completion")
	}

	return &SendMessageResult{
		Role:    chatResp.Choices[0].Message.Role,
		Content: chatResp.Choices[0].Message.Content,
		Model:   chatResp.Model,
		Usage:   chatResp.Usage,
	}, nil
}

func extractModel(tianjiParams any) string {
	params, ok := tianjiParams.(map[string]any)
	if !ok {
		return ""
	}
	if model, ok := params["model"].(string); ok {
		return model
	}
	return ""
}

func extractSystemMessage(agentCardParams any) string {
	params, ok := agentCardParams.(map[string]any)
	if !ok {
		return ""
	}
	if sys, ok := params["system_message"].(string); ok {
		return sys
	}
	return ""
}
