package spend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewCalculatorEmpty(t *testing.T) {
	c, err := NewCalculator("")
	if err != nil {
		t.Fatalf("NewCalculator: %v", err)
	}
	// No pricing → cost should be 0
	cost := c.Calculate("gpt-4o", 100, 50)
	if cost != 0 {
		t.Fatalf("expected 0 cost with no pricing, got %f", cost)
	}
}

func TestNewCalculatorWithFile(t *testing.T) {
	prices := map[string]ModelPricing{
		"gpt-4o": {InputCostPerToken: 0.00001, OutputCostPerToken: 0.00003},
	}
	data, _ := json.Marshal(prices)
	path := filepath.Join(t.TempDir(), "prices.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	c, err := NewCalculator(path)
	if err != nil {
		t.Fatalf("NewCalculator: %v", err)
	}

	cost := c.Calculate("gpt-4o", 1000, 500)
	expected := 1000*0.00001 + 500*0.00003
	if cost != expected {
		t.Fatalf("cost = %f, want %f", cost, expected)
	}
}

func TestCalculateUnknownModel(t *testing.T) {
	c, _ := NewCalculator("")
	cost := c.Calculate("unknown", 100, 50)
	if cost != 0 {
		t.Fatalf("unknown model cost = %f, want 0", cost)
	}
}

func TestNewCalculatorMissingFile(t *testing.T) {
	c, err := NewCalculator("/nonexistent/path.json")
	if err != nil {
		t.Fatalf("should not error: %v", err)
	}
	if c == nil {
		t.Fatal("calculator should not be nil")
	}
}

func TestNewCalculatorInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("not json"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	c, err := NewCalculator(path)
	if err != nil {
		t.Fatalf("should not error: %v", err)
	}
	if c == nil {
		t.Fatal("calculator should not be nil")
	}
}
