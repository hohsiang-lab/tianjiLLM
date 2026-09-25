# Requirements Checklist: HO-1175

**Created**: 2026-05-07
**Feature**: OpenAI OAuth connect/callback state lifecycle

## Spec Quality

- [x] No implementation code changes in Todo planning artifacts.
- [x] Functional requirements are measurable.
- [x] User stories are independently testable.
- [x] Edge cases cover auth, state TTL, replay, provider errors, token exchange failures, and org trust boundary.
- [x] Out of scope is explicit.

## Security Requirements

- [x] Connect requires authenticated UI session.
- [x] Callback relies on server-side state, not UI cookies or API keys.
- [x] Callback ignores query/body organization fields.
- [x] State is short-lived and consume-once.
- [x] Error pages forbid token, code, verifier, encrypted credential, and raw upstream response leakage.

## Data Requirements

- [x] Server-side state fields are defined.
- [x] Existing cache backend was checked for TTL/delete support.
- [x] Existing public base URL config and OpenAI OAuth helpers were checked.
- [x] No migration is planned unless implementation finds a real schema gap.

## Test Requirements

- [x] Failing tests are defined before implementation.
- [x] Each acceptance scenario maps to at least one test.
- [x] Offline mock and no-real-OpenAI guard requirements are explicit.
- [x] Replay and conflicting `org_id` tests are explicit.
