package mcp

import (
	"context"
	"testing"
)

func TestNewManagerEmpty(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
	tools := m.ListTools()
	if len(tools) != 0 {
		t.Fatalf("expected 0 tools, got %d", len(tools))
	}
}

func TestGetServerNotFound(t *testing.T) {
	m := NewManager()
	_, ok := m.GetServer("nonexistent")
	if ok {
		t.Fatal("expected false")
	}
}

func TestCallToolNotFound(t *testing.T) {
	m := NewManager()
	result, err := m.CallTool(context.Background(), "nonexistent-tool", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected error result")
	}
}

func TestCallToolServerUnavailable(t *testing.T) {
	m := NewManager()
	// Manually register a tool without a session
	m.tools["test-tool"] = MCPTool{
		Name:         "tool",
		PrefixedName: "test-tool",
		ServerID:     "srv1",
	}
	m.toolToServer["test-tool"] = "srv1"

	result, err := m.CallTool(context.Background(), "test-tool", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected error for unavailable server")
	}
}

func TestLoadFromConfigUnsupportedTransport(t *testing.T) {
	m := NewManager()
	entries := map[string]MCPServerEntry{
		"bad": {Transport: "grpc"},
	}
	// Should not return error (logs warning), but server won't be connected
	err := m.LoadFromConfig(context.Background(), entries)
	if err != nil {
		t.Fatal(err)
	}
	_, ok := m.GetServer("bad")
	if !ok {
		t.Fatal("server entry should still be registered")
	}
}

func TestLoadFromConfigStdioNoCommand(t *testing.T) {
	m := NewManager()
	entries := map[string]MCPServerEntry{
		"s1": {Transport: "stdio"},
	}
	err := m.LoadFromConfig(context.Background(), entries)
	if err != nil {
		t.Fatal(err)
	}
	// Should be registered but not connected
	if len(m.ListTools()) != 0 {
		t.Fatal("expected no tools for failed connection")
	}
}

func TestLoadFromConfigSSENoURL(t *testing.T) {
	m := NewManager()
	entries := map[string]MCPServerEntry{
		"s2": {Transport: "sse"},
	}
	err := m.LoadFromConfig(context.Background(), entries)
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoadFromConfigHTTPNoURL(t *testing.T) {
	m := NewManager()
	entries := map[string]MCPServerEntry{
		"s3": {Transport: "http"},
	}
	err := m.LoadFromConfig(context.Background(), entries)
	if err != nil {
		t.Fatal(err)
	}
}

func TestToSet(t *testing.T) {
	s := toSet(nil)
	if s != nil {
		t.Fatal("expected nil for empty input")
	}

	s = toSet([]string{"a", "b", " c "})
	if !s["a"] || !s["b"] || !s["c"] {
		t.Fatalf("unexpected set: %v", s)
	}
}
