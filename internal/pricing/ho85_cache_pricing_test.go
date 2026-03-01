// HO-85: ModelPricing sync 未包含 cache token 費率，導致 cache 計費為 0
//
// Root Cause:
//   - upstreamModelEntry struct 缺少 cache_read_input_token_cost 等欄位
//   - ReloadFromDB 只 map input/output cost，未 map cache 費率
//   - DB schema (ModelPricing table) 缺少 cache 費率欄位
//   - 當 DB 有資料時 Calculator lookup 不會 fallback 到 embedded，
//     導致 cache cost = $0
//
// 這些 tests 在 fix 前應全部 FAIL。
package pricing

import (
	"encoding/json"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

// ─── Test 1: upstreamModelEntry 解析 cache 費率後不應丟失 ────────────────────
//
// 驗證 upstreamModelEntry struct 能保留 cache_read_input_token_cost 欄位的值。
// 用 round-trip 方式：Unmarshal → 再讀 raw JSON 中的 cache 欄位。
// 如果 struct 沒有對應欄位，Unmarshal 後 re-marshal 會遺失這些值。
// 目前 FAIL：struct 沒有該欄位，round-trip 後 cache 費率 = 0。
func TestHO85_UpstreamModelEntry_ParsesCacheFields(t *testing.T) {
	input := map[string]interface{}{
		"input_cost_per_token":            3e-06,
		"output_cost_per_token":           1.5e-05,
		"cache_read_input_token_cost":     3e-07,
		"cache_creation_input_token_cost": 3.75e-06,
		"max_input_tokens":                200000,
		"max_output_tokens":               16000,
		"max_tokens":                      216000,
		"mode":                            "chat",
		"litellm_provider":                "anthropic",
	}
	raw, _ := json.Marshal(input)

	// Parse into upstreamModelEntry (this is what sync.go does)
	var entry upstreamModelEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	// Re-marshal to check what fields were captured
	roundTripped, _ := json.Marshal(entry)
	var result map[string]interface{}
	if err := json.Unmarshal(roundTripped, &result); err != nil {
		t.Fatalf("re-marshal failed: %v", err)
	}

	// HO-85: these fields should be captured by upstreamModelEntry but aren't
	// If the struct has no cache fields, these will be missing (zero) after round-trip
	cacheRead, _ := result["cache_read_input_token_cost"].(float64)
	cacheCreation, _ := result["cache_creation_input_token_cost"].(float64)

	const wantCacheRead = 3e-07
	const wantCacheCreation = 3.75e-06

	if cacheRead != wantCacheRead {
		t.Errorf("upstreamModelEntry lost cache_read_input_token_cost after round-trip: got %v, want %v\n"+
			"(field missing in upstreamModelEntry struct -> sync writes 0 to DB)",
			cacheRead, wantCacheRead)
	}
	if cacheCreation != wantCacheCreation {
		t.Errorf("upstreamModelEntry lost cache_creation_input_token_cost after round-trip: got %v, want %v\n"+
			"(field missing in upstreamModelEntry struct -> sync writes 0 to DB)",
			cacheCreation, wantCacheCreation)
	}
}

// ─── Test 2: ReloadFromDB 後 ModelInfo 保留 cache 費率 ──────────────────────
//
// 模擬 DB 有 cache 費率欄位，ReloadFromDB 後 Calculator 的 models layer
// 應包含 CacheReadCostPerToken > 0。
// 目前 FAIL：DB schema + ReloadFromDB 都沒有 cache 欄位。
func TestHO85_ReloadFromDB_PreservesCachePricing(t *testing.T) {
	calc := &Calculator{
		embedded:  make(map[string]ModelInfo),
		models:    make(map[string]ModelInfo),
		overrides: make(map[string]ModelInfo),
	}

	// 模擬從 DB 取出的 ModelPricing row（目前 db.ModelPricing 沒有 cache 欄位）
	dbEntries := []db.ModelPricing{
		{
			ModelName:          "claude-sonnet-4-5",
			InputCostPerToken:  3e-06,
			OutputCostPerToken: 1.5e-05,
			// HO-85: 這兩個欄位目前不存在於 db.ModelPricing:
			// CacheReadInputTokenCost:     3e-07,
			// CacheCreationInputTokenCost: 3.75e-06,
		},
	}

	calc.ReloadFromDB(dbEntries)

	info := calc.GetModelInfo("claude-sonnet-4-5")
	if info == nil {
		t.Fatal("GetModelInfo returned nil for claude-sonnet-4-5 after ReloadFromDB")
	}

	// HO-85: 這兩個值目前 = 0，因為 DB 沒有欄位、ReloadFromDB 沒有 map
	if info.CacheReadCostPerToken == 0 {
		t.Errorf("CacheReadCostPerToken = 0 after ReloadFromDB, want > 0 (DB column missing, ReloadFromDB does not map cache fields)")
	}
	if info.CacheCreationCostPerToken == 0 {
		t.Errorf("CacheCreationCostPerToken = 0 after ReloadFromDB, want > 0 (DB column missing, ReloadFromDB does not map cache fields)")
	}
}

// ─── Test 3: DB 有資料時 cache token cost 不應為 $0 ─────────────────────────
//
// 當 models layer（DB sync 後）有 claude-sonnet-4-5 但 cache 費率 = 0，
// lookup 不會 fallback 到 embedded（embedded 有正確的 cache 費率）。
// 結果：有 cache tokens 的 request cost = $0 for cache portion。
// 目前 FAIL：cost 是 0，但應接近 embedded pricing 的計算結果。
func TestHO85_CacheCostNotZeroWhenDBHasModel(t *testing.T) {
	calc := &Calculator{
		embedded:  make(map[string]ModelInfo),
		models:    make(map[string]ModelInfo),
		overrides: make(map[string]ModelInfo),
	}

	// embedded layer：有正確的 cache 費率（來自 model_prices.json）
	calc.embedded["claude-sonnet-4-5"] = ModelInfo{
		InputCostPerToken:         3e-06,
		OutputCostPerToken:        1.5e-05,
		CacheReadCostPerToken:     3e-07,
		CacheCreationCostPerToken: 3.75e-06,
	}

	// DB sync 後，models layer 有 claude-sonnet-4-5 但 cache 費率 = 0
	// （這是 HO-85 的現狀：DB 沒有 cache 欄位，sync 沒有解析）
	calc.models["claude-sonnet-4-5"] = ModelInfo{
		InputCostPerToken:         3e-06,
		OutputCostPerToken:        1.5e-05,
		CacheReadCostPerToken:     0, // BUG: should be 3e-07
		CacheCreationCostPerToken: 0, // BUG: should be 3.75e-06
	}

	usage := TokenUsage{
		PromptTokens:             1000,
		CompletionTokens:         500,
		CacheReadInputTokens:     5000,
		CacheCreationInputTokens: 2000,
	}

	// 用 embedded 計算「正確」cost（作為預期值）
	embeddedInfo := calc.embedded["claude-sonnet-4-5"]
	expectedCacheReadCost := float64(usage.CacheReadInputTokens) * embeddedInfo.CacheReadCostPerToken
	expectedCacheCreationCost := float64(usage.CacheCreationInputTokens) * embeddedInfo.CacheCreationCostPerToken

	promptCost, _ := calc.Cost("claude-sonnet-4-5", usage)

	// regular input only (no cache): 1000 * 3e-06 = 0.003
	regularInputCost := float64(usage.PromptTokens) * embeddedInfo.InputCostPerToken
	// expected with cache: 0.003 + 5000*3e-07 + 2000*3.75e-06 = 0.003 + 0.0015 + 0.0075 = 0.012
	expectedPromptCost := regularInputCost + expectedCacheReadCost + expectedCacheCreationCost

	// HO-85: 目前 promptCost 只包含 regularInputCost（0.003），因為 cache rates = 0
	// lookup hits models layer (DB) which has rate=0, never falls back to embedded
	if promptCost <= regularInputCost {
		t.Errorf(
			"cache tokens cost = 0: promptCost=%.6f, regularInputCost=%.6f, expectedPromptCost=%.6f\n"+
				"  cache_read:     %d tokens x %.2e = %.6f\n"+
				"  cache_creation: %d tokens x %.2e = %.6f\n"+
				"HO-85: DB model has no cache fields -> lookup returns 0 rates -> $0 for cache tokens",
			promptCost, regularInputCost, expectedPromptCost,
			usage.CacheReadInputTokens, embeddedInfo.CacheReadCostPerToken, expectedCacheReadCost,
			usage.CacheCreationInputTokens, embeddedInfo.CacheCreationCostPerToken, expectedCacheCreationCost,
		)
	}
}

// ─── Test 4: sync 解析 claude-sonnet-4-5 時應包含 cache 費率欄位 ────────────
//
// 模擬 upstream JSON 包含 claude-sonnet-4-5 的 cache 費率，
// 驗證解析後的 upstreamModelEntry 保留這些值（供後續存入 DB）。
// 用 round-trip 方式驗證 struct 有正確捕捉 cache 欄位。
// 目前 FAIL：upstreamModelEntry 沒有 cache 欄位，值丟失。
func TestHO85_SyncParsing_ClaudeSonnet45_CacheFields(t *testing.T) {
	// 真實 upstream LiteLLM JSON 中 claude-sonnet-4-5 的 pricing 資料
	input := map[string]interface{}{
		"input_cost_per_token":                         3e-06,
		"output_cost_per_token":                        1.5e-05,
		"cache_read_input_token_cost":                  3e-07,
		"cache_creation_input_token_cost":              3.75e-06,
		"input_cost_per_token_above_200k_tokens":       6e-06,
		"output_cost_per_token_above_200k_tokens":      2.25e-05,
		"max_input_tokens":                             200000,
		"max_output_tokens":                            16000,
		"max_tokens":                                   216000,
		"mode":                                         "chat",
		"litellm_provider":                             "anthropic",
	}
	raw, _ := json.Marshal(input)

	// This is what SyncFromUpstream does in Step 3
	var entry upstreamModelEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	// Round-trip to check what got captured
	roundTripped, _ := json.Marshal(entry)
	var result map[string]interface{}
	_ = json.Unmarshal(roundTripped, &result)

	cacheRead, _ := result["cache_read_input_token_cost"].(float64)
	cacheCreation, _ := result["cache_creation_input_token_cost"].(float64)

	// HO-85: these should be captured but struct has no fields for them
	if cacheRead != 3e-07 {
		t.Errorf("sync would write cache_read_input_token_cost=%v to DB, want 3e-07\n"+
			"(field missing in upstreamModelEntry -> lost during unmarshal)",
			cacheRead)
	}
	if cacheCreation != 3.75e-06 {
		t.Errorf("sync would write cache_creation_input_token_cost=%v to DB, want 3.75e-06\n"+
			"(field missing in upstreamModelEntry -> lost during unmarshal)",
			cacheCreation)
	}
}
