# Analyze: HO-1190 OpenAI OAuth Mock Harness and Endpoint Override Coverage

## Consistency Check

| Check | Result | Evidence |
|-------|--------|----------|
| Spec scope avoids production behavior | PASS | Scope limits work to test helpers, endpoint override coverage, and guard tests. |
| Linear parent alignment | PASS | HO-1173 requires CI no real OpenAI, `httptest.NewServer`, endpoint overrides, and QA matrix coverage. |
| Repo reality alignment | PASS | Existing OAuth exchange accepts injected client and endpoint override config; OpenAI provider exposes `NewWithBaseURL`. |
| WireMock excluded | PASS | FR-010 and plan explicitly prohibit WireMock/external mock services. |
| Reusable by backend and UI E2E | PASS | `internal/testutil/openaitest` can be imported by backend tests and `test/e2e` because both are inside module parent. |
| Tests-first | PASS | tasks.md T001-T005 are failing tests before helper implementation. |

## Fatal Findings

None.

## Critical Findings

None.

## Minor / Watch Items

- Guard coverage depends on code paths accepting injected `*http.Client` or base URLs. If future refresh/failover code hard-codes `http.DefaultClient`, implementation must add an injection seam before claiming no-real-network coverage.
- `/v1/models` production implementation may be owned by a sibling issue; HO-1190 only needs mock harness and representative tests unless the missing seam blocks endpoint override coverage.

## Owner Scope Questions

None. Sima decision: implement a test-only `internal/testutil/openaitest` harness, keep v1 in-process with `httptest.NewServer`, and use injected clients/base URLs for no-real-OpenAI guard coverage.
