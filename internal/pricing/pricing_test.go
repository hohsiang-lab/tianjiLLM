package pricing

import (
	"sync"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

func newTestCalculator() *Calculator {
	return &Calculator{
		embedded: map[string]ModelInfo{
			"gpt-4": {InputCostPerToken: 0.00003, OutputCostPerToken: 0.00006, Mode: "chat", Provider: "openai"},
		},
		models:    make(map[string]ModelInfo),
		overrides: make(map[string]ModelInfo),
	}
}

func TestDefault_NonNil(t *testing.T) {
	c := Default()
	if c == nil {
		t.Fatal("Default() returned nil")
	}
}

func TestDefault_EmbeddedAndModelsMatch(t *testing.T) {
	c := Default()
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.embedded) == 0 {
		t.Fatal("embedded map is empty after Default()")
	}
	// embedded and models should have same contents initially
	if len(c.embedded) != len(c.models) {
		t.Errorf("embedded len=%d != models len=%d", len(c.embedded), len(c.models))
	}
	for k, ev := range c.embedded {
		mv, ok := c.models[k]
		if !ok {
			t.Errorf("embedded key %q not found in models", k)
			continue
		}
		if ev.InputCostPerToken != mv.InputCostPerToken {
			t.Errorf("key %q: embedded.InputCost=%v != models.InputCost=%v", k, ev.InputCostPerToken, mv.InputCostPerToken)
		}
	}
}

// TestLookupLayerPriority: overrides > models > embedded
func TestLookupLayerPriority(t *testing.T) {
	c := &Calculator{
		embedded:  map[string]ModelInfo{"m": {InputCostPerToken: 1}},
		models:    map[string]ModelInfo{"m": {InputCostPerToken: 2}},
		overrides: map[string]ModelInfo{"m": {InputCostPerToken: 3}},
	}

	// All three defined: should get overrides value
	info := c.lookup("m")
	if info == nil || info.InputCostPerToken != 3 {
		t.Errorf("expected override (3), got %v", info)
	}

	// Remove override → should get models value
	c.overrides = make(map[string]ModelInfo)
	info = c.lookup("m")
	if info == nil || info.InputCostPerToken != 2 {
		t.Errorf("expected models (2), got %v", info)
	}

	// Remove models → should get embedded value
	c.models = make(map[string]ModelInfo)
	info = c.lookup("m")
	if info == nil || info.InputCostPerToken != 1 {
		t.Errorf("expected embedded (1), got %v", info)
	}

	// Remove all → nil
	c.embedded = make(map[string]ModelInfo)
	info = c.lookup("m")
	if info != nil {
		t.Errorf("expected nil, got %v", info)
	}
}

// TestLookupStripPrefix: "provider/model" → strips prefix in each layer
func TestLookupStripPrefix(t *testing.T) {
	c := &Calculator{
		embedded:  map[string]ModelInfo{"claude-3": {InputCostPerToken: 0.001}},
		models:    make(map[string]ModelInfo),
		overrides: make(map[string]ModelInfo),
	}

	info := c.lookup("anthropic/claude-3")
	if info == nil {
		t.Fatal("expected to find via prefix strip in embedded")
	}
	if info.InputCostPerToken != 0.001 {
		t.Errorf("wrong cost: %v", info.InputCostPerToken)
	}
}

// TestLookupStripPrefixPriority: strip prefix also respects layer order
func TestLookupStripPrefixPriority(t *testing.T) {
	c := &Calculator{
		embedded:  map[string]ModelInfo{"m": {InputCostPerToken: 1}},
		models:    map[string]ModelInfo{"m": {InputCostPerToken: 2}},
		overrides: make(map[string]ModelInfo),
	}

	// models layer should win over embedded via prefix strip
	info := c.lookup("openai/m")
	if info == nil || info.InputCostPerToken != 2 {
		t.Errorf("expected models (2) via prefix strip, got %v", info)
	}
}

// TestReloadFromDB replaces models layer
func TestReloadFromDB(t *testing.T) {
	c := newTestCalculator()
	// Initially lookup uses embedded
	info := c.lookup("gpt-4")
	if info == nil || info.InputCostPerToken != 0.00003 {
		t.Fatalf("before reload: unexpected %v", info)
	}

	entries := []db.ModelPricing{
		{
			ModelName:          "gpt-4",
			InputCostPerToken:  0.99,
			OutputCostPerToken: 1.99,
			MaxInputTokens:     128000,
		},
		{
			ModelName:         "gemini-pro",
			InputCostPerToken: 0.0005,
		},
	}
	c.ReloadFromDB(entries)

	// gpt-4 should now use DB value
	info = c.lookup("gpt-4")
	if info == nil || info.InputCostPerToken != 0.99 {
		t.Errorf("after reload: expected 0.99, got %v", info)
	}

	// gemini-pro should now be found
	info = c.lookup("gemini-pro")
	if info == nil {
		t.Fatal("gemini-pro not found after reload")
	}

	// embedded must not change
	c.mu.RLock()
	embeddedInfo, ok := c.embedded["gpt-4"]
	c.mu.RUnlock()
	if !ok || embeddedInfo.InputCostPerToken != 0.00003 {
		t.Errorf("embedded was modified by ReloadFromDB")
	}
}

// TestReloadFromDB_EmptyEntries clears models layer
func TestReloadFromDB_EmptyEntries(t *testing.T) {
	c := newTestCalculator()
	c.models["gpt-4"] = ModelInfo{InputCostPerToken: 0.99}

	c.ReloadFromDB(nil)

	c.mu.RLock()
	_, ok := c.models["gpt-4"]
	c.mu.RUnlock()
	if ok {
		t.Error("expected models to be cleared after ReloadFromDB(nil)")
	}
}

// TestModelCostMap_MergeLayers: all three layers merged, correct priority
func TestModelCostMap_MergeLayers(t *testing.T) {
	// Use a fresh instance (not Default singleton) for isolation
	c := &Calculator{
		embedded:  map[string]ModelInfo{"a": {InputCostPerToken: 1}, "b": {InputCostPerToken: 2}},
		models:    map[string]ModelInfo{"b": {InputCostPerToken: 20}, "c": {InputCostPerToken: 30}},
		overrides: map[string]ModelInfo{"c": {InputCostPerToken: 300}},
	}

	m := c.modelCostMapInternal()

	if m["a"].InputCostPerToken != 1 {
		t.Errorf("a: expected 1, got %v", m["a"])
	}
	if m["b"].InputCostPerToken != 20 {
		t.Errorf("b: expected models(20) to override embedded(2), got %v", m["b"])
	}
	if m["c"].InputCostPerToken != 300 {
		t.Errorf("c: expected overrides(300) to win, got %v", m["c"])
	}
}

// TestModelCostMap_ReturnsCopy verifies mutation doesn't affect internal state
func TestModelCostMap_ReturnsCopy(t *testing.T) {
	c := Default()
	m := ModelCostMap()
	m["__sentinel__"] = ModelInfo{InputCostPerToken: 999}

	c.mu.RLock()
	_, found := c.models["__sentinel__"]
	c.mu.RUnlock()
	if found {
		t.Error("ModelCostMap returned a reference instead of a copy")
	}
}

// TestConcurrentReadWrite: race detector should not flag this
func TestConcurrentReadWrite(t *testing.T) {
	c := newTestCalculator()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				c.lookup("gpt-4")
			}
		}()
	}
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			entries := []db.ModelPricing{
				{ModelName: "gpt-4", InputCostPerToken: 0.01},
			}
			c.ReloadFromDB(entries)
		}()
	}
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			c.SetCustomPricing("custom-model", ModelInfo{InputCostPerToken: float64(n)})
		}(i)
	}
	wg.Wait()
}

// modelCostMapInternal is a test helper to call the merge on any Calculator (not just Default).
func (c *Calculator) modelCostMapInternal() map[string]ModelInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	merged := make(map[string]ModelInfo, len(c.embedded)+len(c.models)+len(c.overrides))
	for k, v := range c.embedded {
		merged[k] = v
	}
	for k, v := range c.models {
		merged[k] = v
	}
	for k, v := range c.overrides {
		merged[k] = v
	}
	return merged
}
