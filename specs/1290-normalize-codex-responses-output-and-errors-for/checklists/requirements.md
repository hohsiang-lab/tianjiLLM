# Requirements Checklist: HO-1290

## Completeness

- [x] Scope covers successful non-streaming Codex Responses output normalization.
- [x] Scope covers 401 auth diagnostics.
- [x] Scope covers 403 missing-scope diagnostics.
- [x] Scope covers 429 quota diagnostics.
- [x] Scope excludes streaming normalization from this issue.
- [x] Scope excludes OAuth/login/credential UI changes.
- [x] Scope preserves Platform OpenAI provider behavior.

## Testability

- [x] Success mapping has an independent mocked unit test.
- [x] 401/403/429 error mapping has mocked tests.
- [x] Token leakage has sentinel tests.
- [x] Existing OpenAI provider regression is required.
- [x] No real OpenAI/ChatGPT network calls are needed.

## Consistency

- [x] `spec.md` requirements map to `plan.md` decisions.
- [x] `tasks.md` starts with RED tests before implementation tasks.
- [x] `contracts/` defines success and error JSON shapes.
- [x] `research.md` records repo and official OpenAI evidence.
- [x] Todo phase is docs-only and implementation is gated on Linear `In Progress`.
