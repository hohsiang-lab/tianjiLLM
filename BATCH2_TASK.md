# Batch 2 Task — Guardrail Policy Binding + Test UI

Read CLAUDE.md first. Follow its rules strictly.

## What exists (Batch 1)
- `internal/ui/handler_guardrails.go` — CRUD handlers for guardrails
- `internal/ui/pages/guardrails.templ` — list + CRUD UI  
- Routes in `internal/ui/routes.go` lines 96-101
- DB: `internal/db/guardrail_mgmt.sql.go` + `internal/db/policy.sql.go`
- PolicyTable has `guardrails_add []string` (guardrail names)
- PolicyAttachmentTable has `policy_name, scope, teams, keys`
- UIHandler in `internal/ui/handler.go` has `DB *db.Queries`

## Task 1: Policy Binding UI

Add a "Bindings" button to each guardrail row that opens a dialog showing:
- Which Policies reference this guardrail (filter `ListPolicies` where `guardrails_add` contains the guardrail name)
- Each policy's attachments via `ListPolicyAttachmentsByPolicy`
- "Add to Policy" — select an existing policy, add guardrail name to its `guardrails_add` via `UpdatePolicy`
- "Remove" — remove guardrail name from a policy's `guardrails_add`

New handlers in `handler_guardrails.go`:
- `handleGuardrailBindings` — GET, returns binding info as HTMX partial
- `handleGuardrailBindingAdd` — POST, adds guardrail to a policy's guardrails_add
- `handleGuardrailBindingRemove` — POST, removes guardrail from a policy's guardrails_add

New templ in `guardrails.templ`:
- Binding dialog + list partial

New routes in `routes.go`:
- GET `/guardrails/{id}/bindings`
- POST `/guardrails/{id}/bindings/add`
- POST `/guardrails/{id}/bindings/remove`

## Task 2: Test UI

Add a "Test" button to each guardrail row that opens a dialog:
- textarea for test prompt
- "Run Test" button
- Result area showing passed/blocked + message

Handler: `handleGuardrailTest` — POST
- Read guardrail config from DB
- Look at `internal/guardrail/guardrails_ai.go` to see if you can instantiate and run a test
- If too complex, just do: validate config JSON is valid, return a simulated result or "Test not available for this guardrail type"
- Keep it simple — the key is the UI working end-to-end

New route: POST `/guardrails/{id}/test`

## Rules
- Use existing UI components (badge, button, card, dialog, icon, input, table, toast)
- Match Batch 1 code style exactly
- Run `templ generate` after changing .templ files
- `go build ./...` must pass
- Commit: `feat(ui): add guardrail policy binding + test UI (batch 2)`
- Push to current branch

Do NOT ask questions. Make your own judgments. If multiple approaches exist, pick simplest.
