package spend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCalculatorEmptyPath(t *testing.T) {
	c, err := NewCalculator("")
	if err != nil {
		t.Fatal(err)
	}
	cost := c.Calculate("gpt-4", 100, 50)
	if cost != 0 {
		t.Fatalf("expected 0, got %f", cost)
	}
}

func TestCalculatorWithPricing(t *testing.T) {
	dir := t.TempDir()
	pricing := map[string]ModelPricing{
		"gpt-4": {
			InputCostPerToken:  0.00003,
			OutputCostPerToken: 0.00006,
		},
	}
	data, _ := json.Marshal(pricing)
	path := filepath.Join(dir, "pricing.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	c, err := NewCalculator(path)
	if err != nil {
		t.Fatal(err)
	}

	cost := c.Calculate("gpt-4", 1000, 500)
	if cost < 0.05 || cost > 0.07 {
		t.Fatalf("cost out of expected range: %f", cost)
	}

	// Unknown model
	cost = c.Calculate("unknown", 100, 50)
	if cost != 0 {
		t.Fatalf("expected 0 for unknown model, got %f", cost)
	}
}

func TestCalculatorInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("{invalid"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := NewCalculator(path)
	if err != nil {
		t.Fatal(err)
	}
	// Should still work, just no pricing loaded
	cost := c.Calculate("gpt-4", 100, 50)
	if cost != 0 {
		t.Fatalf("expected 0, got %f", cost)
	}
}

func TestCalculatorNonexistentFile(t *testing.T) {
	c, err := NewCalculator("/nonexistent/pricing.json")
	if err != nil {
		t.Fatal(err)
	}
	cost := c.Calculate("gpt-4", 100, 50)
	if cost != 0 {
		t.Fatalf("expected 0, got %f", cost)
	}
}
