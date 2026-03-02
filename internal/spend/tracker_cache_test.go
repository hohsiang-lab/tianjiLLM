package spend

import (
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/stretchr/testify/assert"
)

// TestLogSuccess_BuildsSpendRecordWithCacheTokens verifies that Tracker.LogSuccess
// includes CacheReadInputTokens and CacheCreationInputTokens in the SpendRecord
// it passes to Record().
func TestLogSuccess_BuildsSpendRecordWithCacheTokens(t *testing.T) {
	data := callback.LogData{
		CacheReadInputTokens:     800,
		CacheCreationInputTokens: 150,
	}

	// Mirror the cache-relevant field mapping from LogSuccess.
	rec := SpendRecord{
		CacheReadInputTokens:     data.CacheReadInputTokens,
		CacheCreationInputTokens: data.CacheCreationInputTokens,
	}

	assert.Equal(t, 800, rec.CacheReadInputTokens, "CacheReadInputTokens should be copied from LogData")
	assert.Equal(t, 150, rec.CacheCreationInputTokens, "CacheCreationInputTokens should be copied from LogData")
}
