// Package contracts defines the Go interfaces for Phase 2.
// This is a design artifact — NOT compilable production code.
package contracts

import (
	"context"
	"time"
)

// ============================================================
// Work Stream A: Secret Manager
// ============================================================

// SecretManager resolves credential references from external vaults.
type SecretManager interface {
	Name() string
	Get(ctx context.Context, path string) (string, error)
	Health(ctx context.Context) error
}

// SecretManagerFactory creates a SecretManager from config.
type SecretManagerFactory func(cfg map[string]any) (SecretManager, error)

// ============================================================
// Work Stream B: Guardrails (extends existing Guardrail interface)
// ============================================================

// FailurePolicy controls behavior when a guardrail service is unreachable.
type FailurePolicy string

const (
	FailOpen   FailurePolicy = "fail_open"
	FailClosed FailurePolicy = "fail_closed"
)

// ============================================================
// Work Stream C: Cloud Loggers
// ============================================================

// BatchFlusher is the flush callback for batch loggers.
type BatchFlusher func(ctx context.Context, batch []LogData) error

// LogData matches internal/callback/callback.go LogData.
type LogData struct {
	Model            string
	Provider         string
	Request          any
	Response         any
	Error            error
	StartTime        time.Time
	EndTime          time.Time
	Latency          time.Duration
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	Cost             float64
	UserID           string
	TeamID           string
	RequestTags      []string
}

// ============================================================
// Work Stream G: Prompt Management
// ============================================================

// PromptSource resolves prompt templates from external services.
type PromptSource interface {
	Name() string
	GetPrompt(ctx context.Context, promptID string, opts PromptOptions) (*ResolvedPrompt, error)
}

// PromptOptions specifies which version/label to fetch.
type PromptOptions struct {
	Version   *int
	Label     *string
	Variables map[string]string
}

// ResolvedPrompt is the compiled prompt ready for use.
type ResolvedPrompt struct {
	Messages []Message
	Metadata map[string]string
}

// Message is a chat message (simplified).
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
