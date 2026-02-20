package contract

import (
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/a2a"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentRegistry_RegisterAndGet(t *testing.T) {
	reg := a2a.NewAgentRegistry()

	cfg := &a2a.AgentConfig{
		AgentID:   "agent-1",
		AgentName: "test-agent",
		TianjiParams: map[string]any{
			"model": "gpt-4",
		},
	}

	reg.RegisterAgent(cfg)

	agent, ok := reg.GetAgentByID("agent-1")
	require.True(t, ok)
	assert.Equal(t, "test-agent", agent.AgentName)
}

func TestAgentRegistry_GetByName(t *testing.T) {
	reg := a2a.NewAgentRegistry()

	reg.RegisterAgent(&a2a.AgentConfig{
		AgentID:   "agent-2",
		AgentName: "my-agent",
	})

	agent, ok := reg.GetAgentByName("my-agent")
	require.True(t, ok)
	assert.Equal(t, "agent-2", agent.AgentID)
}

func TestAgentRegistry_GetNotFound(t *testing.T) {
	reg := a2a.NewAgentRegistry()

	_, ok := reg.GetAgentByID("nonexistent")
	assert.False(t, ok)
}

func TestAgentRegistry_Deregister(t *testing.T) {
	reg := a2a.NewAgentRegistry()

	reg.RegisterAgent(&a2a.AgentConfig{
		AgentID:   "agent-3",
		AgentName: "temp-agent",
	})
	reg.DeregisterAgent("temp-agent")

	_, ok := reg.GetAgentByID("agent-3")
	assert.False(t, ok)
}

func TestAgentRegistry_ListAgents(t *testing.T) {
	reg := a2a.NewAgentRegistry()

	reg.RegisterAgent(&a2a.AgentConfig{AgentID: "a1", AgentName: "first"})
	reg.RegisterAgent(&a2a.AgentConfig{AgentID: "a2", AgentName: "second"})

	agents := reg.ListAgents()
	assert.Len(t, agents, 2)
}

func TestAgentPermission_AllAllowed(t *testing.T) {
	reg := a2a.NewAgentRegistry()
	reg.RegisterAgent(&a2a.AgentConfig{AgentID: "a1", AgentName: "agent1"})
	reg.RegisterAgent(&a2a.AgentConfig{AgentID: "a2", AgentName: "agent2"})

	ph := a2a.NewAgentPermissionHandler(reg)

	allowed := ph.GetAllowedAgents(nil, nil, nil)
	assert.Len(t, allowed, 2, "all agents should be returned when unrestricted")
}

func TestAgentPermission_TeamRestricted(t *testing.T) {
	reg := a2a.NewAgentRegistry()
	reg.RegisterAgent(&a2a.AgentConfig{AgentID: "a1", AgentName: "agent-1"})
	reg.RegisterAgent(&a2a.AgentConfig{AgentID: "a2", AgentName: "agent-2"})
	reg.RegisterAgent(&a2a.AgentConfig{AgentID: "a3", AgentName: "agent-3"})

	ph := a2a.NewAgentPermissionHandler(reg)

	teamModels := []string{"agent-1", "agent-2"}
	allowed := ph.GetAllowedAgents(nil, teamModels, nil)

	assert.True(t, a2a.IsAgentAllowed("a1", allowed))
	assert.True(t, a2a.IsAgentAllowed("a2", allowed))
	assert.False(t, a2a.IsAgentAllowed("a3", allowed))
}

func TestAgentPermission_Intersection(t *testing.T) {
	reg := a2a.NewAgentRegistry()
	reg.RegisterAgent(&a2a.AgentConfig{AgentID: "a1", AgentName: "agent-1"})
	reg.RegisterAgent(&a2a.AgentConfig{AgentID: "a2", AgentName: "agent-2"})
	reg.RegisterAgent(&a2a.AgentConfig{AgentID: "a3", AgentName: "agent-3"})
	reg.RegisterAgent(&a2a.AgentConfig{AgentID: "a4", AgentName: "agent-4"})

	ph := a2a.NewAgentPermissionHandler(reg)

	keyModels := []string{"agent-1", "agent-2", "agent-3"}
	teamModels := []string{"agent-2", "agent-3", "agent-4"}

	allowed := ph.GetAllowedAgents(keyModels, teamModels, nil)

	assert.True(t, a2a.IsAgentAllowed("a2", allowed))
	assert.True(t, a2a.IsAgentAllowed("a3", allowed))
	assert.False(t, a2a.IsAgentAllowed("a1", allowed))
	assert.False(t, a2a.IsAgentAllowed("a4", allowed))
}

func TestBuildAgentCard(t *testing.T) {
	cfg := &a2a.AgentConfig{
		AgentID:   "test-id",
		AgentName: "Test Agent",
		AgentCardParams: map[string]any{
			"description": "A test agent",
		},
	}

	card := a2a.BuildAgentCard(cfg, "https://example.com")
	assert.Equal(t, "Test Agent", card.Name)
	assert.Equal(t, "https://example.com/a2a/test-id", card.URL)
	assert.Equal(t, "A test agent", card.Description)
}

func TestJSONRPCErrorResponse(t *testing.T) {
	resp := a2a.NewErrorResponse("req-1", -32600, "Invalid request")
	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, "req-1", resp.ID)
	assert.NotNil(t, resp.Error)
	assert.Equal(t, -32600, resp.Error.Code)
}

func TestJSONRPCSuccessResponse(t *testing.T) {
	resp := a2a.NewSuccessResponse("req-2", map[string]string{"status": "ok"})
	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, "req-2", resp.ID)
	assert.Nil(t, resp.Error)
	assert.NotNil(t, resp.Result)
}
