package mcp

import (
	"net/http"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewSSEHandler creates an SSE transport handler for the MCP server.
// Returns an http.Handler suitable for mounting on a chi router.
func NewSSEHandler(server *sdkmcp.Server) http.Handler {
	return sdkmcp.NewSSEHandler(func(r *http.Request) *sdkmcp.Server {
		return server
	}, nil)
}

// NewStreamableHTTPHandler creates a Streamable HTTP transport handler.
// Returns an http.Handler suitable for mounting on a chi router.
func NewStreamableHTTPHandler(server *sdkmcp.Server) http.Handler {
	return sdkmcp.NewStreamableHTTPHandler(func(r *http.Request) *sdkmcp.Server {
		return server
	}, nil)
}
