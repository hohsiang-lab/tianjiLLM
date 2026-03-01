package pricing

// HO-83: 新增 jina-embeddings-v3 定價至 model_prices.json
// 此 test 在 fix 合併前應 FAIL，fix 後應 PASS。

import "testing"

func TestHO83_JinaEmbeddingsV3_HasPositiveInputCost(t *testing.T) {
	info := Default().GetModelInfo("jina-embeddings-v3")
	if info == nil {
		t.Fatal("jina-embeddings-v3 not found in pricing data (model missing from model_prices.json)")
	}
	if info.InputCostPerToken <= 0 {
		t.Errorf("expected InputCostPerToken > 0, got %v", info.InputCostPerToken)
	}
}
