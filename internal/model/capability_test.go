package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCapabilityMatrixLookupIsExactAndFailsClosed(t *testing.T) {
	matrix := CapabilityMatrix{
		{Backend: BackendDirectOpenAIHTTP, Model: "gpt-4o"}: {
			SupportsStream: true,
		},
	}

	record, ok := matrix.Lookup(BackendDirectOpenAIHTTP, "gpt-4o")
	assert.True(t, ok)
	assert.True(t, record.SupportsStream)

	record, ok = matrix.Lookup(BackendDirectOpenAIHTTP, "gpt-4o-mini")
	assert.False(t, ok)
	assert.Equal(t, CapabilityRecord{}, record)

	_, ok = matrix.Lookup(BackendChatGPTCodex, "gpt-4o")
	assert.False(t, ok)
}
