# Scope Confirmation: HO-2091

## Six Todo Scope Questions

1. **要改哪個 repo？**
   `hohsiang-lab/tianjiLLM` only.

2. **現在 Todo 可以改 code 嗎？**
   不行。這輪只產生 SpecKit docs-only artifacts。

3. **實作層在哪裡？**
   Backend scheduler + OpenAI subscription refresh/routing + existing credential/model operator UI status surfacing.

4. **資料形狀釐清了嗎？**
   已釐清。使用既有 `CredentialTable`、encrypted `credential_value`、safe JSONB `credential_info`，新增 safe metadata fields for failure/reconnect state；DB access 走 sqlc query。

5. **相依與邊界釐清了嗎？**
   已釐清。重用 `internal/scheduler`、existing refresh helper、`singleflight`、optional Redis `redsync` lock、existing lifecycle/UI redaction；不改 API-key route、不改 5-minute request buffer、不做 real OpenAI tests。

6. **驗證方式釐清了嗎？**
   已釐清。先寫 failing Go tests，使用 mock OpenAI OAuth server / guarded clients，覆蓋 proactive refresh、skip、lock、invalidated metadata、stale route references、redaction。

7. **UI 畫面知道要對齊哪邊嗎？**
   已釐清。沒有新 screen / layout / form / dialog；對齊既有 Tianji credentials/models admin surfaces，實作只新增 operator-facing status wording / stale-reference diagnostics。In Progress 需用 handler/template tests 驗證 `OpenAI session ended; reconnect required` 與 stale route references，不提交 Todo mockscreen。

## Missing Part

None for Todo scope. UI alignment source is existing credentials/models operator surfaces plus the exact issue-specified reconnect wording; no new layout mockscreen is required.

## Implementation Stop Conditions

- Do not start implementation until Linear moves to `In Progress`.
- Stop if cross-pod lock cannot be provided by existing Redis `redsync` wrapper or a tested DB claim path.
- Stop if any metadata/log/UI path would expose token material.
- Stop if stale-route cleanup would silently mutate routes without operator-visible evidence.

## PR Scope

Docs-only under:

```text
specs/HO-2091-proactively-refresh-idle-openai-subscription/**
```

No production Go, generated code, config, workflow, UI template, or test source changes in Todo.
