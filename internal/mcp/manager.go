package mcp

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPServerManager manages upstream MCP server connections and tool-to-server mapping.
type MCPServerManager struct {
	mu            sync.RWMutex
	servers       map[string]MCPServerEntry
	sessions      map[string]*mcp.ClientSession
	tools         map[string]MCPTool // prefixed_name -> tool
	toolToServer  map[string]string  // prefixed_name -> server_id
	toolSeparator string
}

// NewManager creates a new MCPServerManager.
func NewManager() *MCPServerManager {
	return &MCPServerManager{
		servers:       make(map[string]MCPServerEntry),
		sessions:      make(map[string]*mcp.ClientSession),
		tools:         make(map[string]MCPTool),
		toolToServer:  make(map[string]string),
		toolSeparator: ToolSeparator,
	}
}

// LoadFromConfig registers upstream MCP servers from config and connects to them.
func (m *MCPServerManager) LoadFromConfig(ctx context.Context, entries map[string]MCPServerEntry) error {
	for id, entry := range entries {
		entry.ServerID = id
		if entry.Alias == "" {
			entry.Alias = id
		}
		m.servers[id] = entry

		if err := m.connectServer(ctx, entry); err != nil {
			log.Printf("WARN: failed to connect MCP server %s: %v", id, err)
		}
	}
	return nil
}

// connectServer establishes a connection to an upstream MCP server.
func (m *MCPServerManager) connectServer(ctx context.Context, entry MCPServerEntry) error {
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "tianjiLLM",
		Version: "v1.0.0",
	}, nil)

	var transport mcp.Transport
	switch entry.Transport {
	case "stdio":
		if entry.Command == "" {
			return fmt.Errorf("MCP server %s: stdio transport requires command", entry.ServerID)
		}
		transport = &mcp.CommandTransport{
			Command: exec.CommandContext(ctx, entry.Command, entry.Args...),
		}
	case "sse":
		if entry.URL == "" {
			return fmt.Errorf("MCP server %s: sse transport requires url", entry.ServerID)
		}
		transport = &mcp.SSEClientTransport{
			Endpoint: entry.URL,
		}
	case "http":
		if entry.URL == "" {
			return fmt.Errorf("MCP server %s: http transport requires url", entry.ServerID)
		}
		transport = &mcp.StreamableClientTransport{
			Endpoint: entry.URL,
		}
	default:
		return fmt.Errorf("MCP server %s: unsupported transport %q", entry.ServerID, entry.Transport)
	}

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("connecting to MCP server %s: %w", entry.ServerID, err)
	}

	m.mu.Lock()
	m.sessions[entry.ServerID] = session
	m.mu.Unlock()

	if err := m.discoverTools(ctx, entry, session); err != nil {
		return fmt.Errorf("discovering tools from %s: %w", entry.ServerID, err)
	}

	return nil
}

// discoverTools lists tools from an upstream server and registers them.
func (m *MCPServerManager) discoverTools(ctx context.Context, entry MCPServerEntry, session *mcp.ClientSession) error {
	result, err := session.ListTools(ctx, nil)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	allowed := toSet(entry.AllowedTools)
	disallowed := toSet(entry.DisallowedTools)

	for _, t := range result.Tools {
		if len(allowed) > 0 && !allowed[t.Name] {
			continue
		}
		if disallowed[t.Name] {
			continue
		}

		prefixed := entry.Alias + m.toolSeparator + t.Name
		tool := MCPTool{
			Name:         t.Name,
			PrefixedName: prefixed,
			Description:  t.Description,
			InputSchema:  t.InputSchema,
			ServerID:     entry.ServerID,
		}
		m.tools[prefixed] = tool
		m.toolToServer[prefixed] = entry.ServerID
	}

	return nil
}

// ListTools returns all discovered tools across all connected servers.
func (m *MCPServerManager) ListTools() []MCPTool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tools := make([]MCPTool, 0, len(m.tools))
	for _, t := range m.tools {
		tools = append(tools, t)
	}
	return tools
}

// CallTool invokes a tool on its upstream server and returns the result.
func (m *MCPServerManager) CallTool(ctx context.Context, prefixedName string, arguments map[string]any) (*mcp.CallToolResult, error) {
	m.mu.RLock()
	tool, ok := m.tools[prefixedName]
	if !ok {
		m.mu.RUnlock()
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Tool not found: %s", prefixedName)}},
			IsError: true,
		}, nil
	}
	serverID := m.toolToServer[prefixedName]
	session, hasSession := m.sessions[serverID]
	m.mu.RUnlock()

	if !hasSession {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("MCP server unavailable: %s", serverID)}},
			IsError: true,
		}, nil
	}

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      tool.Name,
		Arguments: arguments,
	})
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Tool call failed: %v", err)}},
			IsError: true,
		}, nil
	}

	return result, nil
}

// GetServer returns the entry for a given server ID.
func (m *MCPServerManager) GetServer(id string) (MCPServerEntry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.servers[id]
	return e, ok
}

func toSet(items []string) map[string]bool {
	if len(items) == 0 {
		return nil
	}
	s := make(map[string]bool, len(items))
	for _, item := range items {
		s[strings.TrimSpace(item)] = true
	}
	return s
}
