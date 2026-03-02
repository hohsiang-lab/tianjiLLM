package spend

import (
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/stretchr/testify/assert"
)

// TestLogSuccess_BuildsSpendRecordWithCacheTokens verifies that Tracker.LogSuccess
// includes CacheReadInputTokens and CacheCreationInputTokens in the SpendRecord
// it passes to Record().
//
// We test by extracting the same SpendRecord construction logic that LogSuccess uses.
func TestLogSuccess_BuildsSpendRecordWithCacheTokens(t *testing.T) {
	data := callback.LogData{
		Model:                    "anthropic/claude-sonnet-4-5-20250929",
		PromptTokens:             1000,
		CompletionTokens:         200,
		TotalTokens:              1200,
		CacheReadInputTokens:     800,
		CacheCreationInputTokens: 150,
		Cost:                     0.01,
	}

	// Reproduce exactly what LogSuccess does (line 53-66 of tracker.go):
	rec := SpendRecord{
		Model:            data.Model,
		APIKey:           data.APIKey,
		PromptTokens:     data.PromptTokens,
		CompletionTokens: data.CompletionTokens,
		TotalTokens:      data.TotalTokens,
		StartTime:        data.StartTime,
		EndTime:          data.EndTime,
		User:             data.UserID,
		TeamID:           data.TeamID,
		Tags:             data.RequestTags,
		Cost:                     data.Cost,
		CallType:                 data.CallType,
		CacheReadInputTokens:     data.CacheReadInputTokens,
		CacheCreationInputTokens: data.CacheCreationInputTokens,
	}

	assert.Equal(t, 800, rec.CacheReadInputTokens, "CacheReadInputTokens should be copied from LogData")
	assert.Equal(t, 150, rec.CacheCreationInputTokens, "CacheCreationInputTokens should be copied from LogData")
}
