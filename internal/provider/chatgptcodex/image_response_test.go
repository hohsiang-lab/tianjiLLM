package chatgptcodex

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransformImageGenerationStream_ExtractsImagesAndUsage(t *testing.T) {
	body := strings.NewReader(strings.Join([]string{
		`data: {"type":"response.output_item.done","item":{"type":"image_generation_call","result":"aW1hZ2U=","revised_prompt":"revised"}}`,
		``,
		`data: {"type":"response.completed","response":{"usage":{"input_tokens":1674,"output_tokens":66,"total_tokens":1740}}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n"))

	got, err := TransformImageGenerationStream(body)

	require.NoError(t, err)
	require.Len(t, got.Images, 1)
	assert.Equal(t, "aW1hZ2U=", got.Images[0].B64JSON)
	assert.Equal(t, "revised", got.Images[0].RevisedPrompt)
	assert.Equal(t, 1674, got.Usage.PromptTokens)
	assert.Equal(t, 66, got.Usage.CompletionTokens)
	assert.Equal(t, 1740, got.Usage.TotalTokens)
}
