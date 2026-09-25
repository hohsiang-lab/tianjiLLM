package callback

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTruncatePayload_UnderLimit(t *testing.T) {
	content := "short content"
	result := TruncatePayload(content, 2048)
	assert.Equal(t, content, result)
}

func TestTruncatePayload_ExactLimit(t *testing.T) {
	content := strings.Repeat("a", 2048)
	result := TruncatePayload(content, 2048)
	assert.Equal(t, content, result)
}

func TestTruncatePayload_OverLimit(t *testing.T) {
	content := strings.Repeat("x", 4096)
	maxChars := 2048
	result := TruncatePayload(content, maxChars)

	assert.LessOrEqual(t, len([]rune(result)), maxChars, "truncated output must not exceed maxChars runes")
	assert.Contains(t, result, truncationMarker)

	available := maxChars - len([]rune(truncationMarker))
	startChars := int(float64(available) * 0.35)
	endChars := available - startChars

	assert.True(t, strings.HasPrefix(result, content[:startChars]))
	assert.True(t, strings.HasSuffix(result, content[len(content)-endChars:]))
}

func TestTruncatePayload_UTF8(t *testing.T) {
	// 200 Chinese characters (3 bytes each in UTF-8)
	content := strings.Repeat("中", 200)
	maxChars := 100
	result := TruncatePayload(content, maxChars)

	runes := []rune(result)
	assert.LessOrEqual(t, len(runes), maxChars, "truncated rune count must not exceed maxChars")
	assert.Contains(t, result, truncationMarker)
	// Verify no broken UTF-8 — all runes should be valid
	for _, r := range runes {
		assert.NotEqual(t, '\uFFFD', r, "truncation must not produce replacement characters")
	}
}

func TestTruncatePayload_EmptyContent(t *testing.T) {
	result := TruncatePayload("", 2048)
	assert.Equal(t, "", result)
}

func TestMaxStringLengthPromptInDB_Default(t *testing.T) {
	// Without env var set, should return default
	result := MaxStringLengthPromptInDB()
	assert.Equal(t, defaultMaxStringLength, result)
}
