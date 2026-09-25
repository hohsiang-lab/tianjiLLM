package pages

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestModelWithReasoningEffort(t *testing.T) {
	assert.Equal(t, "gpt-5.6(low)", modelWithReasoningEffort("gpt-5.6", "low"))
	assert.Equal(t, "gpt-5.6", modelWithReasoningEffort("gpt-5.6", ""))
	assert.Equal(
		t,
		"hf.co/awhiteside/CodeRankEmbed-Q8_0-GGUF:Q8_0",
		modelWithReasoningEffort("hf.co/awhiteside/CodeRankEmbed-Q8_0-GGUF:Q8_0", ""),
	)
}
