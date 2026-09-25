package integration

import (
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/a2a"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPhase6_A2A_AgentRegistryLifecycle(t *testing.T) {
	reg := a2a.NewAgentRegistry()

	// Register
	reg.RegisterAgent(&a2a.AgentConfig{
		AgentID:   "a1",
		AgentName: "test-agent",
		TianjiParams: map[string]any{
			"model": "gpt-4",
		},
	})

	// Get by ID
	agent, ok := reg.GetAgentByID("a1")
	require.True(t, ok)
	assert.Equal(t, "test-agent", agent.AgentName)

	// Get by name
	agent, ok = reg.GetAgentByName("test-agent")
	require.True(t, ok)
	assert.Equal(t, "a1", agent.AgentID)

	// List
	agents := reg.ListAgents()
	assert.Len(t, agents, 1)

	// Deregister
	reg.DeregisterAgent("test-agent")
	_, ok = reg.GetAgentByID("a1")
	assert.False(t, ok)
}

func TestPhase6_A2A_PermissionIntersection(t *testing.T) {
	reg := a2a.NewAgentRegistry()
	reg.RegisterAgent(&a2a.AgentConfig{AgentID: "a1", AgentName: "agent-1"})
	reg.RegisterAgent(&a2a.AgentConfig{AgentID: "a2", AgentName: "agent-2"})
	reg.RegisterAgent(&a2a.AgentConfig{AgentID: "a3", AgentName: "agent-3"})

	ph := a2a.NewAgentPermissionHandler(reg)

	keyModels := []string{"agent-1", "agent-2"}
	teamModels := []string{"agent-2", "agent-3"}

	// Intersection should be agent-2 only
	allowed := ph.GetAllowedAgents(keyModels, teamModels, nil)
	assert.True(t, a2a.IsAgentAllowed("a2", allowed))
	assert.False(t, a2a.IsAgentAllowed("a1", allowed))
	assert.False(t, a2a.IsAgentAllowed("a3", allowed))
}

func TestPhase6_A2A_AgentCard(t *testing.T) {
	cfg := &a2a.AgentConfig{
		AgentID:   "test-id",
		AgentName: "Test Agent",
		AgentCardParams: map[string]any{
			"description": "A test agent for phase 6",
		},
	}

	card := a2a.BuildAgentCard(cfg, "https://proxy.example.com")
	assert.Equal(t, "Test Agent", card.Name)
	assert.Equal(t, "https://proxy.example.com/a2a/test-id", card.URL)
	assert.Equal(t, "A test agent for phase 6", card.Description)
}

func TestPhase6_A2A_JSONRPCResponses(t *testing.T) {
	// Error response
	errResp := a2a.NewErrorResponse("req-1", -32600, "Invalid request")
	assert.Equal(t, "2.0", errResp.JSONRPC)
	assert.Equal(t, "req-1", errResp.ID)
	assert.Equal(t, -32600, errResp.Error.Code)

	// Success response
	succResp := a2a.NewSuccessResponse("req-2", map[string]string{"status": "ok"})
	assert.Equal(t, "2.0", succResp.JSONRPC)
	assert.Equal(t, "req-2", succResp.ID)
	assert.Nil(t, succResp.Error)
	assert.NotNil(t, succResp.Result)
}
