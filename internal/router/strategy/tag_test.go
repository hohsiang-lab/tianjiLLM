package strategy

import (
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/router"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeTagDeployments() []*router.Deployment {
	return []*router.Deployment{
		{ID: "d1", Config: &config.ModelConfig{Tags: []string{"tier:premium", "region:us"}}},
		{ID: "d2", Config: &config.ModelConfig{Tags: []string{"tier:standard", "region:eu"}}},
		{ID: "d3", Config: &config.ModelConfig{Tags: []string{"tier:premium", "region:eu"}}},
		{ID: "d4", Config: &config.ModelConfig{}}, // no tags
	}
}

func TestTagBased_PickWithTags_MatchAll(t *testing.T) {
	s := NewTagBased(NewShuffle())
	deps := makeTagDeployments()

	// Match all: tier:premium AND region:eu → only d3
	picked := s.PickWithTags(deps, []string{"tier:premium", "region:eu"}, false)
	require.NotNil(t, picked)
	assert.Equal(t, "d3", picked.ID)
}

func TestTagBased_PickWithTags_MatchAny(t *testing.T) {
	s := NewTagBased(NewShuffle())
	deps := makeTagDeployments()

	// Match any: tier:premium OR region:eu → d1, d2, d3
	picked := s.PickWithTags(deps, []string{"tier:premium", "region:eu"}, true)
	require.NotNil(t, picked)
	assert.Contains(t, []string{"d1", "d2", "d3"}, picked.ID)
}

func TestTagBased_PickWithTags_NoMatch_FallsBack(t *testing.T) {
	s := NewTagBased(NewShuffle())
	deps := makeTagDeployments()

	// No matching tags → falls back to all deployments
	picked := s.PickWithTags(deps, []string{"nonexistent"}, false)
	require.NotNil(t, picked)
}

func TestTagBased_PickWithTags_EmptyTags(t *testing.T) {
	s := NewTagBased(NewShuffle())
	deps := makeTagDeployments()

	// Empty tags → picks from all via inner strategy
	picked := s.PickWithTags(deps, nil, false)
	require.NotNil(t, picked)
}

func TestHasAllTags(t *testing.T) {
	assert.True(t, hasAllTags([]string{"a", "b", "c"}, []string{"a", "b"}))
	assert.True(t, hasAllTags([]string{"a", "b"}, []string{}))
	assert.False(t, hasAllTags([]string{"a"}, []string{"a", "b"}))
	assert.False(t, hasAllTags(nil, []string{"a"}))
}

func TestHasAnyTag(t *testing.T) {
	assert.True(t, hasAnyTag([]string{"a", "b", "c"}, []string{"b", "z"}))
	assert.True(t, hasAnyTag([]string{"a"}, []string{"a"}))
	assert.True(t, hasAnyTag([]string{"a", "b"}, []string{})) // empty want → true
	assert.False(t, hasAnyTag([]string{"a"}, []string{"b", "c"}))
	assert.False(t, hasAnyTag(nil, []string{"a"}))
}
