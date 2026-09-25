package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModelResponse_ImagesUnmarshal(t *testing.T) {
	raw := `{"id":"gen-1","object":"chat.completion","created":1,"model":"test","choices":[{"index":0,"message":{"role":"assistant","content":"","images":[{"image_url":{"url":"data:image/png;base64,abc"}}]},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`

	var resp ModelResponse
	require.NoError(t, json.Unmarshal([]byte(raw), &resp))
	msg := resp.Choices[0].Message
	assert.Equal(t, "", msg.Content)
	assert.Len(t, msg.Images, 1, "images should unmarshal")
	if len(msg.Images) > 0 {
		assert.Equal(t, "data:image/png;base64,abc", msg.Images[0].ImageURL.URL)
	}
}
