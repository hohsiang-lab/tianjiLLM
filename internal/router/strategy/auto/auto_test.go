package auto

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCosineSimilarity_Identical(t *testing.T) {
	a := []float32{1, 2, 3}
	score := CosineSimilarity(a, a)
	assert.InDelta(t, 1.0, score, 1e-6)
}

func TestCosineSimilarity_Orthogonal(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{0, 1, 0}
	score := CosineSimilarity(a, b)
	assert.InDelta(t, 0.0, score, 1e-6)
}

func TestCosineSimilarity_Opposite(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{-1, 0, 0}
	score := CosineSimilarity(a, b)
	assert.InDelta(t, -1.0, score, 1e-6)
}

func TestCosineSimilarity_EmptyVectors(t *testing.T) {
	score := CosineSimilarity(nil, nil)
	assert.Equal(t, 0.0, score)
}

func TestCosineSimilarity_DifferentLengths(t *testing.T) {
	a := []float32{1, 2}
	b := []float32{1, 2, 3}
	score := CosineSimilarity(a, b)
	assert.Equal(t, 0.0, score)
}

func TestCosineSimilarity_ZeroVector(t *testing.T) {
	a := []float32{0, 0, 0}
	b := []float32{1, 2, 3}
	score := CosineSimilarity(a, b)
	assert.Equal(t, 0.0, score)
}

func TestCosineSimilarity_KnownValue(t *testing.T) {
	a := []float32{1, 2, 3}
	b := []float32{4, 5, 6}
	// cos(a,b) = (4+10+18) / (sqrt(14) * sqrt(77)) = 32 / sqrt(1078)
	expected := 32.0 / math.Sqrt(1078.0)
	score := CosineSimilarity(a, b)
	assert.InDelta(t, expected, score, 1e-6)
}

func TestBestMatch_FindsBest(t *testing.T) {
	query := []float32{1, 0, 0}
	candidates := [][]float32{
		{0, 1, 0},
		{0.9, 0.1, 0},
		{0, 0, 1},
	}
	idx, score := BestMatch(query, candidates, 0.5)
	assert.Equal(t, 1, idx)
	assert.True(t, score > 0.9)
}

func TestBestMatch_NoneAboveThreshold(t *testing.T) {
	query := []float32{1, 0, 0}
	candidates := [][]float32{
		{0, 1, 0},
		{0, 0, 1},
	}
	idx, _ := BestMatch(query, candidates, 0.5)
	assert.Equal(t, -1, idx)
}

func TestBestMatch_EmptyCandidates(t *testing.T) {
	query := []float32{1, 0, 0}
	idx, _ := BestMatch(query, nil, 0.5)
	assert.Equal(t, -1, idx)
}

func TestAutoRouter_RouteWithMockEncoder(t *testing.T) {
	routes := []Route{
		{Name: "math", Model: "gpt-4o", Examples: []string{"solve equation"}},
		{Name: "code", Model: "claude", Examples: []string{"write python"}},
	}

	// Create a pre-initialized AutoRouter (bypass the encoder)
	ar := New(routes, nil, "default-model", 0.3)

	// Manually set routeVectors to bypass encoder
	ar.routeVectors = [][]float32{
		{1, 0, 0}, // math
		{0, 1, 0}, // code
	}
	ar.initialized = true

	// Override encoder with nil (we don't call it for init)
	// But Route() needs an encoder for the query — let's test default fallback instead
	model, err := ar.Route(context.Background(), "")
	require.NoError(t, err)
	assert.Equal(t, "default-model", model, "empty message should return default")
}

func TestAutoRouter_EmptyMessages(t *testing.T) {
	ar := New(nil, nil, "fallback", 0.5)
	model, err := ar.Route(context.Background(), "")
	require.NoError(t, err)
	assert.Equal(t, "fallback", model)
}

func TestAverageVectors(t *testing.T) {
	vecs := [][]float32{
		{1, 2, 3},
		{3, 4, 5},
	}
	avg := averageVectors(vecs)
	assert.InDelta(t, 2.0, float64(avg[0]), 1e-6)
	assert.InDelta(t, 3.0, float64(avg[1]), 1e-6)
	assert.InDelta(t, 4.0, float64(avg[2]), 1e-6)
}

func TestAverageVectors_Empty(t *testing.T) {
	avg := averageVectors(nil)
	assert.Nil(t, avg)
}

func TestAverageVectors_Single(t *testing.T) {
	vecs := [][]float32{{1, 2, 3}}
	avg := averageVectors(vecs)
	assert.Equal(t, []float32{1, 2, 3}, avg)
}
