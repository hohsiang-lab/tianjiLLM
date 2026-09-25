# Analyze: HO-1167 OpenAI Subscription Config and Credential Resolution

**Date**: 2026-05-07
**Scope**: `spec.md`, `plan.md`, `research.md`, `data-model.md`, `contracts/openai-subscription-config.md`, `quickstart.md`, `tasks.md`

## Summary

- Fatal: 0
- Critical: 0
- Owner scope questions: 0
- Governance blocker: none after owner authorized formal branch/draft PR creation

## Cross-Artifact Checks

| Check | Result | Evidence |
|-------|--------|----------|
| Config field appears in spec, data model, contract, and tasks | PASS | `openai_subscription_credential_ids` is consistently named. |
| Existing API-key fallback is preserved when IDs are omitted | PASS | Spec FR-006, plan data flow, quickstart expected behavior. |
| No API-key fallback after configured subscription ID failure | PASS | Spec FR-011, contract runtime failure rule, resolver tests. |
| Custom `api_base` with subscription IDs is rejected | PASS | Spec FR-004, contract invalid YAML, config tests. |
| Custom OpenAI-compatible providers remain API-key based | PASS | Spec FR-005, research alternative B, validation tasks. |
| Direct HTTP and Codex app-server paths are separate | PASS | Spec US3, data model resolver outputs, contract sections. |
| Local stdio env stripping is specified | PASS | Spec FR-015, data model env contract, app-server tests. |
| WebSocket app-server connection auth is separate | PASS | Spec FR-016, data model connection auth, app-server tests. |
| No production implementation in Todo | PASS | tasks Phase 0 blocks implementation until In Progress. |

## Consistency Notes

- The docs are ready for the normal Todo gate: draft PR evidence plus Linear Waiting transition.
- `chatgptAuthTokens` is version-sensitive and marked experimental/internal in OpenAI Codex source. The plan therefore requires implementation-time verification against the checked Codex app-server version before coding the helper.
- Direct OpenAI HTTP bearer injection is modeled as a resolver output boundary because Linear says later routing/injection tickets will apply bearer auth.

## Blockers

None.
