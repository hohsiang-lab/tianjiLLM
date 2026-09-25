package pricing

// HO-83: Tests for inserting embedded model_prices.json entries that are missing from DB.
//
// Feature: After SyncFromUpstream completes, any model present in the embedded
// model_prices.json that does NOT already exist in the DB should be auto-inserted.
// Models that already exist in DB must NOT be overwritten (insert-only semantics).
//
// These are TDD-style failing tests — the feature is not yet implemented.

import (
	"encoding/json"
	"testing"
)

// ---- helpers ----

// newCalcWithEmbedded creates a fresh Calculator with the embedded JSON pre-loaded.
func newCalcWithEmbedded() *Calculator {
	c := &Calculator{
		embedded:  make(map[string]ModelInfo),
		models:    make(map[string]ModelInfo),
		overrides: make(map[string]ModelInfo),
	}
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(modelPricesJSON, &raw)
	for name, data := range raw {
		if name == "sample_spec" {
			continue
		}
		var info ModelInfo
		if err := json.Unmarshal(data, &info); err == nil {
			c.embedded[name] = info
		}
	}
	return c
}

// embeddedModelNames returns all model names present in the embedded JSON.
func embeddedModelNamesHO83() map[string]struct{} {
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(modelPricesJSON, &raw)
	names := make(map[string]struct{}, len(raw))
	for name := range raw {
		if name == "sample_spec" {
			continue
		}
		names[name] = struct{}{}
	}
	return names
}

// ---- Test 1: Embedded-only models are inserted into DB after sync ----
//
// Given: embedded JSON contains at least one model
// And: the DB is initially empty (existingDB is empty set)
// When: selectEmbeddedToInsert is called (the new pure-logic helper HO-83 must add)
// Then: ALL embedded models should appear in the returned insert list
//
// AC: Sync 後，embedded JSON 有但 upstream 沒有的 model → 應該被 insert 進 DB

func TestHO83_EmbeddedOnlyModelIsInsertedAfterSync(t *testing.T) {
	embeddedNames := embeddedModelNamesHO83()
	if len(embeddedNames) == 0 {
		t.Skip("embedded model_prices.json is empty — cannot run test")
	}

	// Pick the first embedded model name for targeted assertion.
	var targetModel string
	for name := range embeddedNames {
		targetModel = name
		break
	}

	calc := newCalcWithEmbedded()

	// DB is empty — no existing models.
	existingDB := map[string]struct{}{}

	// selectEmbeddedToInsert is the new function HO-83 must add.
	// It takes the embedded model map and the set of model names already in DB,
	// and returns the names of embedded models that should be inserted.
	toInsert := selectEmbeddedToInsert(calc.embedded, existingDB)

	toInsertSet := make(map[string]struct{}, len(toInsert))
	for _, name := range toInsert {
		toInsertSet[name] = struct{}{}
	}

	if _, found := toInsertSet[targetModel]; !found {
		t.Errorf("HO-83 Test1: embedded model %q should be in insert list when DB is empty, got %d items total",
			targetModel, len(toInsert))
	}

	// All embedded models must be candidates when DB is empty.
	for name := range embeddedNames {
		if _, found := toInsertSet[name]; !found {
			t.Errorf("HO-83 Test1: embedded model %q missing from insert candidates (DB empty)", name)
		}
	}

}

// ---- Test 2: DB-existing model is NOT overwritten (insert-only) ----
//
// Given: embedded JSON contains model X
// And: DB already has model X (admin-customized pricing)
// When: selectEmbeddedToInsert is called
// Then: model X must NOT appear in the insert list (insert-only, no overwrite)
//
// AC: Sync 後，DB 已有某 model（admin 手改）→ 不應被覆蓋（insert-only）

func TestHO83_ExistingDBModelIsNotOverwritten(t *testing.T) {
	embeddedNames := embeddedModelNamesHO83()
	if len(embeddedNames) == 0 {
		t.Skip("embedded model_prices.json is empty — cannot run test")
	}

	// Pick a model that IS in embedded JSON to simulate admin ownership.
	var adminModel string
	for name := range embeddedNames {
		adminModel = name
		break
	}

	calc := newCalcWithEmbedded()

	// Simulate DB already containing the model (admin has customized it).
	existingDB := map[string]struct{}{
		adminModel: {},
	}

	toInsert := selectEmbeddedToInsert(calc.embedded, existingDB)

	// The admin model MUST NOT be in the insert list.
	for _, name := range toInsert {
		if name == adminModel {
			t.Errorf("HO-83 Test2: model %q already in DB must NOT be in insert list (insert-only violated)", name)
		}
	}

	// All OTHER embedded models (not in DB) SHOULD be in insert list.
	toInsertSet := make(map[string]struct{}, len(toInsert))
	for _, name := range toInsert {
		toInsertSet[name] = struct{}{}
	}
	for name := range embeddedNames {
		if name == adminModel {
			continue // correctly excluded
		}
		if _, found := toInsertSet[name]; !found {
			t.Errorf("HO-83 Test2: non-DB embedded model %q should be in insert list but is missing", name)
		}
	}
}

// ---- Test 3: Upstream models continue to be synced normally ----
//
// Given: upstream contains 60+ models (normal sync)
// And: embedded JSON has models that are NOT in upstream
// When: selectEmbeddedToInsert is called with only upstream models in existingDB
// Then: embedded-only models should be identified as needing insert
//       (normal upstream upsert behavior is not affected by this logic)
//
// AC: Sync 後，upstream 有的 model → 正常行為不變

func TestHO83_UpstreamModelsSyncedNormally(t *testing.T) {
	embeddedNames := embeddedModelNamesHO83()
	calc := newCalcWithEmbedded()

	// Simulate: DB contains only upstream models (not any embedded models).
	// These upstream model names don't overlap with embedded JSON model names.
	upstreamInDB := map[string]struct{}{
		"upstream-only-a0": {},
		"upstream-only-b0": {},
		"upstream-only-c0": {},
	}

	toInsert := selectEmbeddedToInsert(calc.embedded, upstreamInDB)

	// ALL embedded models should be candidates since none are in DB.
	if len(toInsert) != len(embeddedNames) {
		t.Errorf("HO-83 Test3: expected %d embedded models for insert (upstream models in DB don't overlap embedded), got %d",
			len(embeddedNames), len(toInsert))
	}

	// Upstream models should NOT appear in insert list
	// (they're not in embedded JSON, so selectEmbeddedToInsert ignores them).
	toInsertSet := make(map[string]struct{}, len(toInsert))
	for _, name := range toInsert {
		toInsertSet[name] = struct{}{}
	}
	for name := range upstreamInDB {
		if _, found := toInsertSet[name]; found {
			t.Errorf("HO-83 Test3: upstream model %q should not be in embedded insert list", name)
		}
	}
}

// ---- Edge case: all embedded models already in DB ----

func TestHO83_NothingInsertedWhenAllEmbeddedAlreadyInDB(t *testing.T) {
	embeddedNames := embeddedModelNamesHO83()
	calc := newCalcWithEmbedded()

	existingDB := make(map[string]struct{}, len(embeddedNames))
	for name := range embeddedNames {
		existingDB[name] = struct{}{}
	}

	toInsert := selectEmbeddedToInsert(calc.embedded, existingDB)

	if len(toInsert) != 0 {
		t.Errorf("HO-83: expected empty insert list when all embedded models already in DB, got %d", len(toInsert))
	}
}

// ---- Edge case: nil existingDB ----

func TestHO83_SelectEmbeddedToInsert_NilExistingDB(t *testing.T) {
	embedded := map[string]ModelInfo{
		"model-a": {InputCostPerToken: 0.001},
		"model-b": {InputCostPerToken: 0.002},
	}
	toInsert := selectEmbeddedToInsert(embedded, nil)
	if len(toInsert) != 2 {
		t.Errorf("HO-83: expected 2 models with nil existingDB, got %d", len(toInsert))
	}
}

// ---- Edge case: empty embedded map ----

func TestHO83_SelectEmbeddedToInsert_EmptyEmbedded(t *testing.T) {
	toInsert := selectEmbeddedToInsert(map[string]ModelInfo{}, map[string]struct{}{})
	if len(toInsert) != 0 {
		t.Errorf("HO-83: expected empty result for empty embedded map, got %d", len(toInsert))
	}
}
