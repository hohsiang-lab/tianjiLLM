package pricing

import (
	"testing"
)

func TestCostKnownModel(t *testing.T) {
	c := Default()
	// gpt-4o should exist in embedded pricing
	p, comp := c.Cost("gpt-4o", 1000, 500)
	if p == 0 && comp == 0 {
		t.Skip("gpt-4o not in embedded pricing")
	}
	if p < 0 || comp < 0 {
		t.Fatalf("negative cost: prompt=%f completion=%f", p, comp)
	}
}

func TestCostUnknownModel(t *testing.T) {
	c := Default()
	p, comp := c.Cost("nonexistent-model-xyz", 1000, 500)
	if p != 0 || comp != 0 {
		t.Fatalf("unknown model should return 0 cost, got %f %f", p, comp)
	}
}

func TestTotalCost(t *testing.T) {
	c := Default()
	total := c.TotalCost("gpt-4o", 1000, 500)
	p, comp := c.Cost("gpt-4o", 1000, 500)
	if total != p+comp {
		t.Fatalf("TotalCost=%f != prompt(%f)+completion(%f)", total, p, comp)
	}
}

func TestGetModelInfoKnown(t *testing.T) {
	c := Default()
	info := c.GetModelInfo("gpt-4o")
	if info == nil {
		t.Skip("gpt-4o not in embedded pricing")
	}
	if info.InputCostPerToken <= 0 {
		t.Fatalf("InputCostPerToken=%f", info.InputCostPerToken)
	}
}

func TestGetModelInfoUnknown(t *testing.T) {
	c := Default()
	info := c.GetModelInfo("nonexistent-model-xyz")
	if info != nil {
		t.Fatal("should return nil for unknown model")
	}
}

func TestCostWithCustomPricing(t *testing.T) {
	c := Default()
	c.SetCustomPricing("my-custom-model", ModelInfo{
		InputCostPerToken:  0.001,
		OutputCostPerToken: 0.002,
	})
	p, comp := c.Cost("my-custom-model", 100, 200)
	if p != 0.1 {
		t.Fatalf("prompt cost = %f, want 0.1", p)
	}
	if comp != 0.4 {
		t.Fatalf("completion cost = %f, want 0.4", comp)
	}
	// Cleanup
	c.mu.Lock()
	delete(c.overrides, "my-custom-model")
	c.mu.Unlock()
}
