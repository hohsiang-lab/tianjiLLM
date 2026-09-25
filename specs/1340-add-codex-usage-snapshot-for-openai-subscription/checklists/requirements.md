# Requirements Checklist: HO-1340 Codex usage snapshot

## Clarity

- [x] Scope is limited to Codex usage snapshot visibility for OpenAI subscription credentials.
- [x] The unofficial endpoint is explicitly best-effort and not billing truth.
- [x] Non-goals exclude proxy routing, payload behavior, and per-request fetching.
- [x] Open questions are empty for Todo scope.

## Safety

- [x] Existing credential resolution/refresh reuse is required.
- [x] Separate token decrypt path is forbidden.
- [x] Token/raw body/cookie/header persistence and rendering are forbidden.
- [x] Redaction tests are required across DB, JSON, HTML, logs, and audit metadata.

## Testability

- [x] Provider helper request/header tests are specified.
- [x] Parser tests are specified.
- [x] Cache/backoff tests are specified.
- [x] 401 refresh-once tests are specified.
- [x] UI credential detail tests are specified.
- [x] `/ui/usage` Codex tab tests are specified.
- [x] No-proxy-request-fetch regression is specified.

## Scope Confirmation

- [x] Todo phase remains docs-only.
- [x] Implementation tasks are blocked until Linear leaves Todo.
- [x] No owner-input blocker remains.
