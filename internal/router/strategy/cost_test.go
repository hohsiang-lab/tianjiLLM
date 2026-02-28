package strategy

import (
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/router"
)

func TestLowestCostPickEmpty(t *testing.T) {
	lc := NewLowestCost()
	if lc.Pick(nil) != nil {
		t.Fatal("should return nil for empty")
	}
}

func TestLowestCostPick(t *testing.T) {
	cheapCost := 0.001
	expensiveCost := 0.01

	deployments := []*router.Deployment{
		{
			ID:        "expensive",
			ModelName: "gpt-4o",
			Config: &config.ModelConfig{
				ModelInfo: &config.ModelInfo{InputCost: &expensiveCost},
			},
		},
		{
			ID:        "cheap",
			ModelName: "gpt-3.5-turbo",
			Config: &config.ModelConfig{
				ModelInfo: &config.ModelInfo{InputCost: &cheapCost},
			},
		},
	}

	lc := NewLowestCost()
	picked := lc.Pick(deployments)
	if picked == nil || picked.ID != "cheap" {
		t.Fatalf("expected cheap, got %v", picked)
	}
}
