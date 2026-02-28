package a2a

import (
	"testing"
)

// --- Registry tests ---

func TestNewAgentRegistry(t *testing.T) {
	r := NewAgentRegistry()
	if r == nil {
		t.Fatal("nil registry")
	}
	if len(r.ListAgents()) != 0 {
		t.Fatal("new registry should be empty")
	}
}

func TestRegisterAndGet(t *testing.T) {
	r := NewAgentRegistry()
	cfg := &AgentConfig{AgentID: "id1", AgentName: "bot1"}
	r.RegisterAgent(cfg)

	got, ok := r.GetAgentByID("id1")
	if !ok || got.AgentName != "bot1" {
		t.Fatal("GetAgentByID failed")
	}

	got, ok = r.GetAgentByName("bot1")
	if !ok || got.AgentID != "id1" {
		t.Fatal("GetAgentByName failed")
	}
}

func TestGetNotFound(t *testing.T) {
	r := NewAgentRegistry()
	_, ok := r.GetAgentByID("nope")
	if ok {
		t.Fatal("should not find")
	}
	_, ok = r.GetAgentByName("nope")
	if ok {
		t.Fatal("should not find")
	}
}

func TestDeregisterAgent(t *testing.T) {
	r := NewAgentRegistry()
	r.RegisterAgent(&AgentConfig{AgentID: "id1", AgentName: "bot1"})
	r.DeregisterAgent("bot1")
	if len(r.ListAgents()) != 0 {
		t.Fatal("should be empty after deregister")
	}
}

func TestDeregisterNonExistent(t *testing.T) {
	r := NewAgentRegistry()
	r.DeregisterAgent("nope") // should not panic
}

func TestListAgents(t *testing.T) {
	r := NewAgentRegistry()
	r.RegisterAgent(&AgentConfig{AgentID: "a", AgentName: "aa"})
	r.RegisterAgent(&AgentConfig{AgentID: "b", AgentName: "bb"})
	if len(r.ListAgents()) != 2 {
		t.Fatalf("ListAgents = %d, want 2", len(r.ListAgents()))
	}
}

func TestLoadFromConfig(t *testing.T) {
	r := NewAgentRegistry()
	r.LoadFromConfig([]AgentConfig{
		{AgentID: "x", AgentName: "xx"},
		{AgentID: "y", AgentName: "yy"},
	})
	if len(r.ListAgents()) != 2 {
		t.Fatal("LoadFromConfig should register 2 agents")
	}
}

// --- Permission tests ---

func TestIsAgentAllowed(t *testing.T) {
	if !IsAgentAllowed("a", []string{"a", "b"}) {
		t.Fatal("should be allowed")
	}
	if IsAgentAllowed("c", []string{"a", "b"}) {
		t.Fatal("should not be allowed")
	}
	if IsAgentAllowed("a", nil) {
		t.Fatal("nil list = not allowed")
	}
}

func setupRegistry() *AgentRegistry {
	r := NewAgentRegistry()
	r.RegisterAgent(&AgentConfig{AgentID: "id1", AgentName: "bot1", AccessGroups: []string{"group-a"}})
	r.RegisterAgent(&AgentConfig{AgentID: "id2", AgentName: "bot2", AccessGroups: []string{"group-b"}})
	r.RegisterAgent(&AgentConfig{AgentID: "id3", AgentName: "bot3", AccessGroups: []string{"group-a", "group-b"}})
	return r
}

func TestGetAllowedAgentsNoRestrictions(t *testing.T) {
	r := setupRegistry()
	h := NewAgentPermissionHandler(r)
	allowed := h.GetAllowedAgents(nil, nil, nil)
	if len(allowed) != 3 {
		t.Fatalf("got %d, want 3", len(allowed))
	}
}

func TestGetAllowedAgentsKeyRestricted(t *testing.T) {
	r := setupRegistry()
	h := NewAgentPermissionHandler(r)
	allowed := h.GetAllowedAgents([]string{"bot1"}, nil, nil)
	if len(allowed) != 1 || allowed[0] != "id1" {
		t.Fatalf("got %v, want [id1]", allowed)
	}
}

func TestGetAllowedAgentsTeamRestricted(t *testing.T) {
	r := setupRegistry()
	h := NewAgentPermissionHandler(r)
	allowed := h.GetAllowedAgents(nil, []string{"bot2"}, nil)
	if len(allowed) != 1 || allowed[0] != "id2" {
		t.Fatalf("got %v, want [id2]", allowed)
	}
}

func TestGetAllowedAgentsBothRestricted(t *testing.T) {
	r := setupRegistry()
	h := NewAgentPermissionHandler(r)
	// Only bot3 is in both key and team sets
	allowed := h.GetAllowedAgents([]string{"bot1", "bot3"}, []string{"bot2", "bot3"}, nil)
	if len(allowed) != 1 || allowed[0] != "id3" {
		t.Fatalf("got %v, want [id3]", allowed)
	}
}

func TestGetAllowedAgentsAccessGroups(t *testing.T) {
	r := setupRegistry()
	h := NewAgentPermissionHandler(r)
	allowed := h.GetAllowedAgents(nil, nil, []string{"group-a"})
	// bot1 and bot3 have group-a
	if len(allowed) != 2 {
		t.Fatalf("got %d, want 2", len(allowed))
	}
}

// --- Protocol tests ---

func TestBuildAgentCard(t *testing.T) {
	cfg := &AgentConfig{
		AgentID:   "abc",
		AgentName: "mybot",
		AgentCardParams: map[string]any{
			"description": "A test bot",
			"provider":    "acme",
		},
	}
	card := BuildAgentCard(cfg, "https://example.com")
	if card.Name != "mybot" {
		t.Fatalf("Name = %q", card.Name)
	}
	if card.URL != "https://example.com/a2a/abc" {
		t.Fatalf("URL = %q", card.URL)
	}
	if card.Description != "A test bot" {
		t.Fatalf("Description = %q", card.Description)
	}
	if card.Provider != "acme" {
		t.Fatalf("Provider = %q", card.Provider)
	}
}

func TestBuildAgentCardNoParams(t *testing.T) {
	cfg := &AgentConfig{AgentID: "x", AgentName: "y"}
	card := BuildAgentCard(cfg, "http://localhost")
	if card.Name != "y" {
		t.Fatal("wrong name")
	}
	if card.Description != "" {
		t.Fatal("should have no description")
	}
}

func TestNewErrorResponse(t *testing.T) {
	resp := NewErrorResponse("req1", ErrCodeMethodNotFound, "not found")
	if resp.JSONRPC != "2.0" {
		t.Fatal("wrong jsonrpc")
	}
	if resp.Error == nil || resp.Error.Code != ErrCodeMethodNotFound {
		t.Fatal("wrong error")
	}
	if resp.ID != "req1" {
		t.Fatal("wrong id")
	}
}

func TestNewSuccessResponse(t *testing.T) {
	resp := NewSuccessResponse(42, "ok")
	if resp.Result != "ok" {
		t.Fatal("wrong result")
	}
	if resp.ID != 42 {
		t.Fatal("wrong id")
	}
}

// --- Helper tests ---

func TestToSet(t *testing.T) {
	s := toSet([]string{"a", "b", "a"})
	if len(s) != 2 {
		t.Fatalf("set size = %d, want 2", len(s))
	}
}

func TestHasOverlap(t *testing.T) {
	s := toSet([]string{"x", "y"})
	if !hasOverlap([]string{"y", "z"}, s) {
		t.Fatal("should overlap")
	}
	if hasOverlap([]string{"z", "w"}, s) {
		t.Fatal("should not overlap")
	}
}
