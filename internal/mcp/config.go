package mcp

// ToolSeparator is the default separator between server alias and tool name.
const ToolSeparator = "-"

// MCPServerEntry is the parsed representation of an MCP server from config.
type MCPServerEntry struct {
	ServerID        string
	Alias           string
	Transport       string // "stdio", "sse", "http"
	URL             string
	Command         string
	Args            []string
	AuthType        string
	AuthToken       string
	StaticHeaders   map[string]string
	AllowedTools    []string
	DisallowedTools []string
}

// MCPTool represents a callable tool discovered from an upstream MCP server.
type MCPTool struct {
	Name         string `json:"name"`
	PrefixedName string `json:"prefixed_name"`
	Description  string `json:"description"`
	InputSchema  any    `json:"inputSchema"`
	ServerID     string `json:"server_id"`
}
