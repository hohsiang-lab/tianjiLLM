package prompt

import (
	"context"
	"fmt"
	"sync"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

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
	Messages []model.Message
	Metadata map[string]string
}

// Factory creates a PromptSource from config.
type Factory func(cfg map[string]any) (PromptSource, error)

var (
	factoryMu sync.RWMutex
	factories = make(map[string]Factory)
)

// Register adds a prompt source factory.
func Register(name string, f Factory) {
	factoryMu.Lock()
	factories[name] = f
	factoryMu.Unlock()
}

// New creates a PromptSource by name.
func New(name string, cfg map[string]any) (PromptSource, error) {
	factoryMu.RLock()
	f, ok := factories[name]
	factoryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown prompt source: %s", name)
	}
	return f(cfg)
}
