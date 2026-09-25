# Requirements Checklist: HO-1288

**Purpose**: Validate SpecKit artifacts before implementation.

## Completeness

- [x] Linear endpoint requirement is captured: `https://chatgpt.com/backend-api/codex/responses`.
- [x] Subscription credential resolution is captured.
- [x] `Authorization` bearer header is captured.
- [x] `ChatGPT-Account-Id` conditional header is captured.
- [x] `originator: codex_cli_rs` default plus override is captured.
- [x] API-key OpenAI provider no-regression scope is captured.
- [x] No owner input is required for scope.

## Testability

- [x] RED URL test is listed.
- [x] RED no-Platform-call test is listed.
- [x] RED header test is listed.
- [x] API-key provider regression test is listed.
- [x] Credential failure/no-fallback tests are listed.
- [x] Tests are mocked and offline.

## Scope Boundaries

- [x] No production code is changed in Todo phase.
- [x] Codex app-server login is out of scope.
- [x] OAuth connect/refresh persistence is out of scope.
- [x] Existing direct Platform `/v1` behavior remains compatible unless explicit backend transport is selected.
