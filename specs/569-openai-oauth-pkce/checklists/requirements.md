# Requirements Checklist: OpenAI OAuth PKCE Primitives and Provider Config

**Feature**: HO-1174 OpenAI OAuth PKCE primitives and provider config  
**Date**: 2026-05-06

## Spec Quality

- [x] User stories are independently testable.
- [x] Acceptance scenarios cover PKCE/state, config defaults/overrides, token exchange shape, and redirect safety.
- [x] Out-of-scope items are explicit: callback, persistence, UI, refresh, routing.
- [x] No production OpenAI network dependency is required for tests.
- [x] Security-sensitive invariants are explicit: no `client_secret`, HTTPS production redirect, secure randomness.

## Planning Quality

- [x] Failing tests are listed with exact function names and file paths.
- [x] Existing API-key OpenAI path has a regression test.
- [x] Research sources are documented.
- [x] DB/sqlc is marked not applicable for this issue.
- [x] Scope questions are captured before implementation.
