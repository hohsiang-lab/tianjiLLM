package strategy

import (
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/router"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tpmDeployments() []*router.Deployment {
	tpm1 := int64(1000)
	tpm2 := int64(2000)
	return []*router.Deployment{
		{ID: "d1", Config: &config.ModelConfig{TianjiParams: config.TianjiParams{TPM: &tpm1}}},
		{ID: "d2", Config: &config.ModelConfig{TianjiParams: config.TianjiParams{TPM: &tpm2}}},
	}
}

func TestLowestTPMRPM_PicksLowestUtilization(t *testing.T) {
	s := NewLowestTPMRPM(NewShuffle())
	deps := tpmDeployments()

	// d1: 500/1000 = 50%, d2: 400/2000 = 20% → pick d2
	s.RecordUsage("d1", 500)
	s.RecordUsage("d2", 400)

	picked := s.Pick(deps)
	require.NotNil(t, picked)
	assert.Equal(t, "d2", picked.ID)
}

func TestLowestTPMRPM_PicksZeroUsageFirst(t *testing.T) {
	s := NewLowestTPMRPM(NewShuffle())
	deps := tpmDeployments()

	// d1 has usage, d2 has none → pick d2
	s.RecordUsage("d1", 500)

	picked := s.Pick(deps)
	require.NotNil(t, picked)
	assert.Equal(t, "d2", picked.ID)
}

func TestLowestTPMRPM_ResetClearsUsage(t *testing.T) {
	s := NewLowestTPMRPM(NewShuffle())
	deps := tpmDeployments()

	s.RecordUsage("d1", 999)
	s.ResetUsage()

	// After reset both are at 0, d1 has lower limit so either is fine
	picked := s.Pick(deps)
	require.NotNil(t, picked)
}

func TestLowestTPMRPM_NoLimits_FallsBack(t *testing.T) {
	s := NewLowestTPMRPM(NewShuffle())
	deps := []*router.Deployment{
		{ID: "d1", Config: &config.ModelConfig{}},
		{ID: "d2", Config: &config.ModelConfig{}},
	}

	// No TPM limits → falls back to inner strategy
	picked := s.Pick(deps)
	require.NotNil(t, picked)
}
