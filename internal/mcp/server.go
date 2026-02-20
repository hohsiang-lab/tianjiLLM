package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewMCPServer creates a new MCP protocol server backed by the given manager.
// Tools are dynamically registered on the server from the manager's tool list.
func NewMCPServer(manager *MCPServerManager) *sdkmcp.Server {
	server := sdkmcp.NewServer(&sdkmcp.Implementation{
		Name:    "tianjiLLM",
		Version: "v1.0.0",
	}, nil)

	return server
}

// SyncTools registers all tools from the manager onto the MCP server.
// Call this after LoadFromConfig to populate the tool list.
func SyncTools(server *sdkmcp.Server, manager *MCPServerManager) {
	tools := manager.ListTools()
	for _, t := range tools {
		tool := t // capture
		inputSchema := tool.InputSchema
		if inputSchema == nil {
			inputSchema = json.RawMessage(`{"type":"object"}`)
		}
		server.AddTool(
			&sdkmcp.Tool{
				Name:        tool.PrefixedName,
				Description: tool.Description,
				InputSchema: inputSchema,
			},
			func(ctx context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
				var args map[string]any
				if req.Params.Arguments != nil {
					if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
						return &sdkmcp.CallToolResult{
							Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: fmt.Sprintf("invalid arguments: %v", err)}},
							IsError: true,
						}, nil
					}
				}
				return manager.CallTool(ctx, tool.PrefixedName, args)
			},
		)
	}
}
