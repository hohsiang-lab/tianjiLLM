package a2a

import (
	"context"
	"fmt"
	"sync"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

// AgentConfig represents an agent's configuration.
type AgentConfig struct {
	AgentID         string   `json:"agent_id"`
	AgentName       string   `json:"agent_name"`
	TianjiParams    any      `json:"tianji_params"`
	AgentCardParams any      `json:"agent_card_params"`
	AccessGroups    []string `json:"agent_access_groups"`
	CreatedBy       string   `json:"created_by"`
}

// AgentRegistry manages A2A agent registrations in memory + DB.
type AgentRegistry struct {
	mu     sync.RWMutex
	byID   map[string]*AgentConfig
	byName map[string]*AgentConfig
}

// NewAgentRegistry creates an empty agent registry.
func NewAgentRegistry() *AgentRegistry {
	return &AgentRegistry{
		byID:   make(map[string]*AgentConfig),
		byName: make(map[string]*AgentConfig),
	}
}

// RegisterAgent adds an agent to the in-memory registry.
func (r *AgentRegistry) RegisterAgent(cfg *AgentConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[cfg.AgentID] = cfg
	r.byName[cfg.AgentName] = cfg
}

// DeregisterAgent removes an agent by name.
func (r *AgentRegistry) DeregisterAgent(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if cfg, ok := r.byName[name]; ok {
		delete(r.byID, cfg.AgentID)
		delete(r.byName, name)
	}
}

// GetAgentByID returns an agent by ID.
func (r *AgentRegistry) GetAgentByID(id string) (*AgentConfig, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cfg, ok := r.byID[id]
	return cfg, ok
}

// GetAgentByName returns an agent by name.
func (r *AgentRegistry) GetAgentByName(name string) (*AgentConfig, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cfg, ok := r.byName[name]
	return cfg, ok
}

// ListAgents returns all registered agents.
func (r *AgentRegistry) ListAgents() []*AgentConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*AgentConfig, 0, len(r.byID))
	for _, cfg := range r.byID {
		result = append(result, cfg)
	}
	return result
}

// LoadFromDB populates the in-memory registry from the database.
func (r *AgentRegistry) LoadFromDB(ctx context.Context, queries *db.Queries) error {
	if queries == nil {
		return nil
	}
	agents, err := queries.ListAgents(ctx, db.ListAgentsParams{
		Limit:  1000,
		Offset: 0,
	})
	if err != nil {
		return fmt.Errorf("load agents from DB: %w", err)
	}
	for _, a := range agents {
		r.RegisterAgent(&AgentConfig{
			AgentID:      a.AgentID,
			AgentName:    a.AgentName,
			AccessGroups: a.AgentAccessGroups,
			CreatedBy:    a.CreatedBy,
		})
	}
	return nil
}

// LoadFromConfig populates the in-memory registry from YAML config entries.
func (r *AgentRegistry) LoadFromConfig(configs []AgentConfig) {
	for i := range configs {
		r.RegisterAgent(&configs[i])
	}
}
