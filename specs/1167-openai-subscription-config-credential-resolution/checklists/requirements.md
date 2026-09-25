# Requirements Checklist: HO-1167

**Purpose**: Validate the SpecKit artifacts before implementation.
**Created**: 2026-05-07

## Completeness

- [x] Scope includes explicit `openai_subscription_credential_ids`.
- [x] Scope rejects auto-consuming all org credentials.
- [x] Scope preserves existing API-key fallback when IDs are omitted.
- [x] Scope forbids API-key fallback after configured subscription IDs fail.
- [x] Scope rejects subscription IDs with custom `api_base`.
- [x] Scope restricts subscription credentials to official OpenAI/default base.
- [x] Scope splits direct HTTP and Codex app-server credential application paths.
- [x] Scope includes local stdio env stripping for `CODEX_API_KEY` and `OPENAI_API_KEY`.
- [x] Scope includes WebSocket app-server auth separation.

## Testability

- [x] Config load tests are specified.
- [x] Config validation rejection tests are specified.
- [x] Existing API-key regression tests are specified.
- [x] Direct OpenAI HTTP resolver tests are specified.
- [x] Codex app-server login RPC tests are specified.
- [x] Local app-server env stripping tests are specified.
- [x] WebSocket auth separation tests are specified.

## Governance

- [x] No production code is included in Todo artifacts.
- [x] Branch/PR creation is now authorized for SpecKit planning artifacts.
- [x] Linear may move to Waiting after draft PR creation.
- [x] Implementation remains blocked until Linear moves to In Progress.
