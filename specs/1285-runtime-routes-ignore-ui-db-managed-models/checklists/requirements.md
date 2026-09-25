# Requirements Checklist: Runtime routes consume UI DB-managed models

## Completeness

- [x] Scope identifies affected runtime listing, routing, provider resolution, discovery, and refresh paths.
- [x] Scope lists every issue-named affected route family.
- [x] Functional requirements are testable.
- [x] Duplicate YAML/DB merge rule is defined.
- [x] Wildcard behavior requires an explicit implementation decision and tests.
- [x] DB outage and YAML-only fallback are covered.
- [x] Create/update/delete refresh behavior is covered.
- [x] No production code is planned in Todo state.

## Testability

- [x] RED tests are first implementation tasks.
- [x] `/v1/models` DB inclusion has a test.
- [x] `/v1/chat/completions` DB routing has a test.
- [x] One non-chat route has a test.
- [x] `/model_group/info` DB discovery has a test.
- [x] Duplicate precedence has a test.
- [x] Refresh/restart-required behavior has a test.
- [x] Route audit evidence is required for every issue-listed path family.

## Security

- [x] Tests must use mocked upstreams.
- [x] Error contract forbids leaking API keys, bearer tokens, encrypted credential values, or raw JSON.
- [x] Existing OpenAI subscription credential behavior is preserved.

## Governance

- [x] Linear `Todo` blocks production implementation.
- [x] Draft PR is docs-only.
- [x] `Waiting Merge` is blocked until `In Progress` implementation, review, and CI gates complete.
