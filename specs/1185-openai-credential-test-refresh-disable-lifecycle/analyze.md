# Analyze: HO-1185 OpenAI Credential Test/Refresh/Disable Lifecycle APIs

## Fatal

0

## Critical

0

## Owner Input Required

0

## Evidence

- Linear HO-1185 is In Progress and asks for OpenAI credential test/refresh/disable lifecycle APIs。
- Existing repo already has OpenAI subscription credential CRUD、refresh、disable metadata、audit helper、mock upstream `/v1/models` support。
- Official OpenAI docs confirm Bearer auth and `GET /v1/models` model-list endpoint。
- Earlier critical gaps were fixed: delete idempotency is explicit, `plan.md` lists concrete failing tests, and plan-review evidence is recorded。
- Implementation now adds handler-level lifecycle tests plus route/handler implementation; no owner input remains。

## Scope Confirmation

Proceed with the scoped lifecycle API contract:

- test: refresh if needed, then call mockable `GET /v1/models`。
- refresh: force-refresh selected credential through existing refresh helper。
- disable: local safe metadata update only, idempotent for already-disabled credentials。
- all responses/audit/metadata redacted。
- no owner questions remain。

## Implementation Gate Evidence

- Failing test gate first failed because `OpenAISubscriptionCredentialTest` / `Refresh` / `Disable` handlers and upstream mock hooks did not exist。
- Targeted lifecycle tests now cover fresh/stale test credential, upstream failure redaction, explicit refresh success/failure, local-only idempotent disable, wrong type/missing, delete compatibility, disabled resolution exclusion, no-real-OpenAI guarded client, and audit redaction。
