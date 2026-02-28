# HO-66: CI/CD Test Coverage Gate + Coverage Report

## Summary

在 GitHub Actions CI pipeline 加入 coverage gate，PR 合併前強制檢查測試覆蓋率不低於 threshold（10%），並將 coverage report 呈現於 PR summary，防止覆蓋率退化。

## Background

- 目前 codebase 覆蓋率約 6.5%
- 既有 `ci.yml` 的 `test` job 已跑 `go test -race -cover ./...`，但無 gate、無 report
- `internal/ui/components/` 和 `internal/ui/pages/` 為 templ 生成碼，不應計入覆蓋率
- CI 使用 `ubuntu-latest` runner，已有 PostgreSQL service container

## Functional Requirements

### FR-1: Coverage Profile 產生

- 修改 `test` job，加 `-coverprofile=coverage.out` 產生 coverage profile
- 排除 `internal/ui/components` 和 `internal/ui/pages` 路徑（透過 `grep -v` 過濾 coverage.out 或使用 `-coverpkg` 指定 package 清單）

### FR-2: Coverage Gate

- 解析 `coverage.out`，計算總覆蓋率百分比
- 若低於 **10%** threshold → step 失敗，PR check 為 ❌
- Threshold 值以 workflow 層級的 `env` 變數定義（`COVERAGE_THRESHOLD: 10`），方便日後調整

### FR-3: Coverage Report 呈現

- 使用 `go tool cover -func=coverage.out` 產生 function-level report
- 將摘要（總覆蓋率 + top 10 lowest-coverage packages）寫入 **GitHub Actions Job Summary**（`$GITHUB_STEP_SUMMARY`）
- 不使用第三方 action 或外部服務，純 shell script 實作

### FR-4: Coverage Diff（Nice to Have）

- 在 PR 觸發時，取 `main` branch 的 coverage 作為 baseline（透過 artifact 或 cache）
- 計算 diff 並顯示於 Job Summary（e.g., `Coverage: 8.2% (+1.7%)`）
- 若實作成本過高可跳過，標註 `<!-- TODO: coverage diff -->`

## Non-Functional Requirements

- **不引入第三方 coverage action**：使用 `go tool cover` + shell，減少供應鏈依賴
- **不影響既有 CI 速度**：coverage 分析在同一 job 內完成，不額外開 job
- **Backward compatible**：不改變 lint / build / docker job 行為

## Implementation Notes

### 排除 templ 生成碼

```bash
# 過濾 coverage.out 中的 templ 生成路徑
grep -v -E 'internal/ui/(components|pages)/' coverage.out > coverage-filtered.out
```

### Coverage Gate Script

```bash
TOTAL=$(go tool cover -func=coverage-filtered.out | grep '^total:' | awk '{print $NF}' | tr -d '%')
echo "Total coverage: ${TOTAL}%"
if (( $(echo "$TOTAL < $COVERAGE_THRESHOLD" | bc -l) )); then
  echo "::error::Coverage ${TOTAL}% is below threshold ${COVERAGE_THRESHOLD}%"
  exit 1
fi
```

### Job Summary

```bash
{
  echo "## 🧪 Test Coverage Report"
  echo ""
  echo "**Total: ${TOTAL}%** (threshold: ${COVERAGE_THRESHOLD}%)"
  echo ""
  echo '```'
  go tool cover -func=coverage-filtered.out | tail -20
  echo '```'
} >> "$GITHUB_STEP_SUMMARY"
```

## Success Criteria

| ID | Criterion | Verification |
|----|-----------|--------------|
| SC-1 | PR 觸發 CI 時產生 `coverage.out` | 檢查 workflow log |
| SC-2 | `internal/ui/components` 和 `internal/ui/pages` 不計入覆蓋率 | 檢查 filtered coverage 無相關路徑 |
| SC-3 | 覆蓋率 ≥ 10% 時 check pass | 用現有 codebase（6.5%）測試會 fail；加足夠 test 後 pass |
| SC-4 | 覆蓋率 < 10% 時 check fail，error message 清楚顯示實際值與 threshold | 故意移除 test 驗證 |
| SC-5 | Job Summary 顯示覆蓋率數字與 function-level breakdown | 檢查 PR 的 Actions summary tab |
| SC-6 | Threshold 可透過修改 env 變數調整，不需改 script | 修改 `COVERAGE_THRESHOLD` 值驗證 |

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| 目前 6.5% 低於 10% threshold，gate 會立即 fail | PR 無法合併 | 本 PR 同時需補足 test 達到 10%，或先設 threshold = 5% 再逐步提高 |
| `go tool cover` 精度受 build tag 影響 | 覆蓋率數字不準 | 確保 CI 與本地 build tag 一致 |

## Out of Scope

- Codecov / Coveralls 等第三方平台整合
- Per-file coverage annotation
- Branch coverage（Go 原生不支援）
- Self-hosted ARC runner 遷移（現有 `ubuntu-latest` 足夠）

## References

- 既有 CI: `.github/workflows/ci.yml`
- Linear issue: HO-66
