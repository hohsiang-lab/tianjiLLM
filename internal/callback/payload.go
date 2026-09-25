package callback

import (
	"log"
	"os"
	"strconv"
	"sync"
)

const (
	defaultMaxStringLength = 2048
	truncationMarker       = "\n...[TRUNCATED]...\n"
)

var (
	maxStringOnce  sync.Once
	maxStringValue int
)

// MaxStringLengthPromptInDB returns the configured max string length for payloads,
// read from MAX_STRING_LENGTH_PROMPT_IN_DB env var (default 2048).
// The value is read once at first call and cached for the process lifetime.
func MaxStringLengthPromptInDB() int {
	maxStringOnce.Do(func() {
		maxStringValue = defaultMaxStringLength
		if v := os.Getenv("MAX_STRING_LENGTH_PROMPT_IN_DB"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n <= 0 {
				log.Printf("warn: invalid MAX_STRING_LENGTH_PROMPT_IN_DB=%q, using default %d", v, defaultMaxStringLength)
			} else {
				maxStringValue = n
			}
		}
	})
	return maxStringValue
}

// TruncatePayload truncates content to maxChars runes using a 35% front + 65% back split.
// The total output length (including the marker) never exceeds maxChars runes.
// Returns the original string if it's within the limit.
func TruncatePayload(content string, maxChars int) string {
	runes := []rune(content)
	if len(runes) <= maxChars {
		return content
	}
	markerRunes := []rune(truncationMarker)
	available := maxChars - len(markerRunes)
	if available <= 0 {
		return string(runes[:maxChars])
	}
	startChars := int(float64(available) * 0.35)
	endChars := available - startChars
	return string(runes[:startChars]) + truncationMarker + string(runes[len(runes)-endChars:])
}
