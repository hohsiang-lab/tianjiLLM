package a2a

import (
	"encoding/json"
	"fmt"
)

// JSON-RPC 2.0 types for A2A protocol.

// JSONRPCRequest represents an incoming JSON-RPC 2.0 request.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      any             `json:"id,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	Result  any           `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
	ID      any           `json:"id"`
}

// JSONRPCError represents a JSON-RPC 2.0 error.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Standard JSON-RPC error codes.
const (
	ErrCodeParse          = -32700
	ErrCodeInvalidRequest = -32600
	ErrCodeMethodNotFound = -32601
	ErrCodeInvalidParams  = -32602
	ErrCodeInternal       = -32603
)

// SendMessageParams holds parameters for message/send.
type SendMessageParams struct {
	Message   UserMessage `json:"message"`
	AgentCard any         `json:"agent_card,omitempty"`
}

// UserMessage represents a user message in A2A.
type UserMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// SendMessageResult holds the result of message/send.
type SendMessageResult struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Model   string `json:"model,omitempty"`
	Usage   any    `json:"usage,omitempty"`
}

// AgentCard represents the .well-known/agent-card.json response.
type AgentCard struct {
	Name               string   `json:"name"`
	Description        string   `json:"description,omitempty"`
	URL                string   `json:"url"`
	Provider           string   `json:"provider,omitempty"`
	Version            string   `json:"version,omitempty"`
	Capabilities       any      `json:"capabilities,omitempty"`
	Authentication     any      `json:"authentication,omitempty"`
	DefaultInputModes  []string `json:"defaultInputModes,omitempty"`
	DefaultOutputModes []string `json:"defaultOutputModes,omitempty"`
	Skills             []any    `json:"skills,omitempty"`
}

// BuildAgentCard creates an AgentCard from an AgentConfig.
func BuildAgentCard(cfg *AgentConfig, baseURL string) AgentCard {
	card := AgentCard{
		Name:               cfg.AgentName,
		URL:                fmt.Sprintf("%s/a2a/%s", baseURL, cfg.AgentID),
		Version:            "1.0",
		DefaultInputModes:  []string{"text"},
		DefaultOutputModes: []string{"text"},
	}

	if params, ok := cfg.AgentCardParams.(map[string]any); ok {
		if desc, ok := params["description"].(string); ok {
			card.Description = desc
		}
		if provider, ok := params["provider"].(string); ok {
			card.Provider = provider
		}
		if caps := params["capabilities"]; caps != nil {
			card.Capabilities = caps
		}
		if skills, ok := params["skills"].([]any); ok {
			card.Skills = skills
		}
	}

	return card
}

// NewErrorResponse creates a JSON-RPC error response.
func NewErrorResponse(id any, code int, message string) JSONRPCResponse {
	return JSONRPCResponse{
		JSONRPC: "2.0",
		Error:   &JSONRPCError{Code: code, Message: message},
		ID:      id,
	}
}

// NewSuccessResponse creates a JSON-RPC success response.
func NewSuccessResponse(id any, result any) JSONRPCResponse {
	return JSONRPCResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      id,
	}
}
