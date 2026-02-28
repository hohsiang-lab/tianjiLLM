package callback

import (
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// LogData holds all information about an LLM call for logging.
type LogData struct {
	Model            string
	Provider         string
	APIKey           string
	Request          *model.ChatCompletionRequest
	Response         *model.ModelResponse
	Error            error
	StartTime        time.Time
	EndTime          time.Time
	Latency          time.Duration
	LLMAPILatency    time.Duration
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	Cost             float64
	UserID           string
	TeamID           string
	RequestTags      []string
	CacheHit                 bool
	CacheReadInputTokens     int
	CacheCreationInputTokens int
}

// CustomLogger is the interface for observability callbacks.
type CustomLogger interface {
	LogSuccess(data LogData)
	LogFailure(data LogData)
}
