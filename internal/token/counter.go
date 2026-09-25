package token

import (
	"strings"
	"sync"

	"github.com/pkoukk/tiktoken-go"
)

// Counter provides token counting for chat completion requests.
// Caches tiktoken encoders per model for efficiency.
type Counter struct {
	mu       sync.Mutex
	encoders map[string]*tiktoken.Tiktoken
}

// New creates a new token counter.
func New() *Counter {
	return &Counter{
		encoders: make(map[string]*tiktoken.Tiktoken),
	}
}

// CountText returns the number of tokens in a text string for the given model.
// Returns -1 if the model is not supported (non-OpenAI models).
func (c *Counter) CountText(model, text string) int {
	enc := c.getEncoder(model)
	if enc == nil {
		return -1
	}
	return len(enc.Encode(text, nil, nil))
}

// CountMessages returns the number of tokens for a list of chat messages.
// Returns -1 if the model is not supported.
// Token counting follows OpenAI's formula: each message adds overhead tokens
// for role/name fields, plus the content tokens.
func (c *Counter) CountMessages(model string, messages []Message) int {
	enc := c.getEncoder(model)
	if enc == nil {
		return -1
	}

	// Per-message overhead varies by model family
	tokensPerMessage := 3 // default for gpt-3.5-turbo and later
	tokensPerName := 1

	total := 0
	for _, msg := range messages {
		total += tokensPerMessage
		total += len(enc.Encode(msg.Role, nil, nil))
		total += len(enc.Encode(msg.Content, nil, nil))
		if msg.Name != "" {
			total += tokensPerName
			total += len(enc.Encode(msg.Name, nil, nil))
		}
	}
	total += 3 // every reply is primed with <|start|>assistant<|message|>
	return total
}

// Message is a simplified message for token counting.
type Message struct {
	Role    string
	Content string
	Name    string
}

// getEncoder returns a cached tiktoken encoder for the model.
func (c *Counter) getEncoder(model string) *tiktoken.Tiktoken {
	encoding := modelToEncoding(model)
	if encoding == "" {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if enc, ok := c.encoders[encoding]; ok {
		return enc
	}

	enc, err := tiktoken.GetEncoding(encoding)
	if err != nil {
		return nil
	}

	c.encoders[encoding] = enc
	return enc
}

// modelToEncoding maps model names to tiktoken encoding names.
// Returns empty string for unsupported models.
func modelToEncoding(model string) string {
	// Strip provider prefix (e.g. "openai/gpt-4o" → "gpt-4o")
	if idx := strings.Index(model, "/"); idx >= 0 {
		model = model[idx+1:]
	}

	// o200k_base: GPT-4o, GPT-4.1, GPT-4.5, o1, o3, o4-mini
	switch {
	case strings.HasPrefix(model, "gpt-4o"),
		strings.HasPrefix(model, "gpt-4.1"),
		strings.HasPrefix(model, "gpt-4.5"),
		strings.HasPrefix(model, "o1"),
		strings.HasPrefix(model, "o3"),
		strings.HasPrefix(model, "o4"),
		strings.HasPrefix(model, "chatgpt-4o"):
		return "o200k_base"

	// cl100k_base: GPT-4, GPT-3.5-turbo
	case strings.HasPrefix(model, "gpt-4"),
		strings.HasPrefix(model, "gpt-3.5"):
		return "cl100k_base"

	default:
		// Unknown model — try o200k_base as default for OpenAI-like models
		if strings.Contains(model, "gpt") {
			return "o200k_base"
		}
		return "" // non-OpenAI model
	}
}
