package openai

import (
	"bytes"
	"encoding/json"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

var doneMarker = []byte("[DONE]")

// ParseStreamChunk parses a single SSE data line from OpenAI's streaming API.
// Returns (chunk, isDone, error).
func ParseStreamChunk(data []byte) (*model.StreamChunk, bool, error) {
	data = bytes.TrimSpace(data)

	if bytes.Equal(data, doneMarker) {
		return nil, true, nil
	}

	var chunk model.StreamChunk
	if err := json.Unmarshal(data, &chunk); err != nil {
		return nil, false, err
	}

	return &chunk, false, nil
}
