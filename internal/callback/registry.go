package callback

import (
	"fmt"
	"strings"
	"sync"
)

// Registry holds registered callback loggers and dispatches events.
type Registry struct {
	mu      sync.RWMutex
	loggers []CustomLogger
}

// NewRegistry creates a new callback registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds a logger to the registry.
func (r *Registry) Register(logger CustomLogger) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.loggers = append(r.loggers, logger)
}

// LogSuccess dispatches a success event to all registered loggers.
func (r *Registry) LogSuccess(data LogData) {
	r.mu.RLock()
	loggers := make([]CustomLogger, len(r.loggers))
	copy(loggers, r.loggers)
	r.mu.RUnlock()

	for _, l := range loggers {
		l.LogSuccess(data)
	}
}

// LogFailure dispatches a failure event to all registered loggers.
func (r *Registry) LogFailure(data LogData) {
	r.mu.RLock()
	loggers := make([]CustomLogger, len(r.loggers))
	copy(loggers, r.loggers)
	r.mu.RUnlock()

	for _, l := range loggers {
		l.LogFailure(data)
	}
}

// Count returns the number of registered loggers.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.loggers)
}

// Names returns the type names of all registered loggers.
func (r *Registry) Names() []string {
	r.mu.RLock()
	loggers := make([]CustomLogger, len(r.loggers))
	copy(loggers, r.loggers)
	r.mu.RUnlock()

	names := make([]string, len(loggers))
	for i, l := range loggers {
		name := fmt.Sprintf("%T", l)
		// Strip package path: *callback.WebhookCallback → WebhookCallback
		if idx := strings.LastIndex(name, "."); idx >= 0 {
			name = name[idx+1:]
		}
		name = strings.TrimPrefix(name, "*")
		names[i] = name
	}
	return names
}
