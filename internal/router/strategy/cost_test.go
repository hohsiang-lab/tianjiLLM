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

// NewFromConfig coverage
func TestNewFromConfig_AllStrategies(t *testing.T) {
	strategies := []string{
		"", "simple-shuffle", "least-busy", "lowest-latency",
		"lowest-cost", "usage-based", "lowest-tpm-rpm", "priority",
	}
	for _, s := range strategies {
		got, err := NewFromConfig(s)
		if err != nil || got == nil {
			t.Errorf("NewFromConfig(%q) failed: %v", s, err)
		}
	}
	_, err := NewFromConfig("unknown-xyz")
	if err == nil {
		t.Fatal("expected error for unknown strategy")
	}
}

// inputCost coverage: fallback to pricing table
func TestInputCost_FallbackPricing(t *testing.T) {
	d := &router.Deployment{
		ModelName: "gpt-4o",
		Config:    &config.ModelConfig{},
	}
	cost := inputCost(d)
	// Should return a positive value from the pricing table or MaxFloat64
	if cost < 0 {
		t.Errorf("expected non-negative cost, got %f", cost)
	}
}

func TestInputCost_ConfigOverride(t *testing.T) {
	v := 0.005
	d := &router.Deployment{
		ModelName: "gpt-4o",
		Config: &config.ModelConfig{
			ModelInfo: &config.ModelInfo{InputCost: &v},
		},
	}
	if got := inputCost(d); got != v {
		t.Errorf("expected %f, got %f", v, got)
	}
}
