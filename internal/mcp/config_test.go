package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToolSeparator(t *testing.T) {
	assert.Equal(t, "-", ToolSeparator)
}

func TestMCPServerEntry_Fields(t *testing.T) {
	entry := MCPServerEntry{
		ServerID:  "test-server",
		Alias:     "test",
		Transport: "stdio",
		Command:   "/usr/bin/test",
		Args:      []string{"--flag"},
	}
	assert.Equal(t, "test-server", entry.ServerID)
	assert.Equal(t, "test", entry.Alias)
	assert.Equal(t, "stdio", entry.Transport)
}

func TestMCPTool_Fields(t *testing.T) {
	tool := MCPTool{
		Name:         "read_file",
		PrefixedName: "fs-read_file",
		Description:  "Read a file",
		ServerID:     "fs",
	}
	assert.Equal(t, "read_file", tool.Name)
	assert.Equal(t, "fs-read_file", tool.PrefixedName)
}

func TestNewManager(t *testing.T) {
	m := NewManager()
	assert.NotNil(t, m)
	assert.NotNil(t, m.servers)
	assert.NotNil(t, m.tools)
	assert.Equal(t, ToolSeparator, m.toolSeparator)
}
