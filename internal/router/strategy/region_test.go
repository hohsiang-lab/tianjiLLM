package strategy

import (
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/router"
	"github.com/stretchr/testify/assert"
)

func TestFilterByRegion_PrefixMatch(t *testing.T) {
	deps := []*router.Deployment{
		{ID: "us1", Region: "us-east1"},
		{ID: "eu1", Region: "eu-west1"},
		{ID: "eu2", Region: "eu-central1"},
	}

	result := FilterByRegion(deps, "eu")
	assert.Len(t, result, 2)
	assert.Equal(t, "eu1", result[0].ID)
	assert.Equal(t, "eu2", result[1].ID)
}

func TestFilterByRegion_NoMatch_ReturnsAll(t *testing.T) {
	deps := []*router.Deployment{
		{ID: "us1", Region: "us-east1"},
	}

	result := FilterByRegion(deps, "ap")
	assert.Len(t, result, 1)
	assert.Equal(t, "us1", result[0].ID)
}

func TestFilterByRegion_EmptyFilter_ReturnsAll(t *testing.T) {
	deps := []*router.Deployment{
		{ID: "us1", Region: "us-east1"},
		{ID: "eu1", Region: "eu-west1"},
	}

	result := FilterByRegion(deps, "")
	assert.Len(t, result, 2)
}
