# Requirements Checklist: OpenAI connect and callback UI flow

**Feature**: HO-1187 OpenAI connect and callback UI flow
**Date**: 2026-05-08

## Completeness

- [x] Linear Goal is represented in `spec.md`.
- [x] Scope includes OpenAI Connect entry point.
- [x] Scope includes authenticated UI-session connect start.
- [x] Scope includes callback success state.
- [x] Scope includes callback readable failure/error page.
- [x] Tests include UI E2E success with mocked OAuth/token upstream.
- [x] Tests include UI E2E callback failure/error state.
- [x] Tests include unauthenticated connect blocked/redirected behavior.
- [x] Out-of-scope sibling responsibilities are explicit.

## Safety

- [x] No production code is planned in Todo state.
- [x] No real OpenAI network or credentials are allowed in tests.
- [x] Callback UI redaction requirements are explicit.
- [x] Existing HO-1175 server-side state and stored `org_id` boundary is preserved.
- [x] Callback route remains public but state-authenticated.

## Testability

- [x] Each P1 user story has an independent test.
- [x] Verification commands are listed.
- [x] Mock OAuth upstream path is documented.
- [x] DOM secret assertions are required.

## Owner Input

- [x] Owner input required: 0.
