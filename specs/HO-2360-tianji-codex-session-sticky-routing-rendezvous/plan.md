# Implementation Plan: Codex session sticky rendezvous routing

**Branch**: `HO-2360-tianji-codex-session-sticky-routing-rendezvous` | **Date**: 2026-07-09 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `specs/HO-2360-tianji-codex-session-sticky-routing-rendezvous/spec.md`

## Summary

Codex backend OpenAI subscription routing currently uses org / route-class sticky selection, causing unrelated Codex sessions to converge on hot credentials. Add a Codex-only session identity extraction path, pass it into Codex candidate ordering at all supported Codex entrypoints, and use rendezvous hashing for new session sticky selection across already-selectable credentials while preserving existing reuse and fallback semantics.

## Implementation Approach

實作細節：已釐清。Implementation should add a small `codexSessionIdentity` helper, a Codex-only session-aware route key helper, and a rendezvous selector while keeping the existing `openAISubscriptionRouteKey(ctx, params)` behavior for non-Codex and missing-session fallback. Entrypoint wiring is limited to Responses HTTP/WebSocket/compact and chat completion/streaming, all of which have payload or `req.Metadata` before current candidate ordering.

Clarification: 已釐清。Linear issue body, Linear comments, repo code confirmation, and this plan agree on session identity sources, excluded identifiers, entrypoints, image exclusion, UI N/A, and required tests.

Missing part: none. Scope, repo, entrypoints, fallback behavior, UI N/A, schema N/A, and verification strategy are all recorded in `spec.md`, `research.md`, `plan.md`, and `tasks.md`; production implementation still belongs to the later In Progress scene.

## Technical Context

**Language/Version**: Go module in `tianjiLLM`; use existing Go toolchain from repo
**Primary Dependencies**: Existing stdlib `crypto/sha256`, `encoding/binary`, `encoding/json`, `strings`; existing handler routing/sticky code; no new third-party dependency
**Storage**: No DB/schema/migration change; existing in-memory sticky maps and credential usage cache only
**Testing**: `go test` with existing tests in `internal/proxy/handler`, `internal/provider/chatgptcodex`, `internal/config`
**Target Platform**: Tianji Go backend server
**Project Type**: Single Go backend/web app
**Performance Goals**: Candidate ordering remains O(number of currently selectable credentials) per new sticky selection; candidate pool is small and already resolved before selection
**Constraints**: No raw token/credential/session payload leakage; no UI/config toggle; no image schema expansion; no non-Codex routing behavior change
**Scale/Scope**: Codex OpenAI subscription backend entrypoints only: responses HTTP, responses WebSocket, responses compact, chat completion, chat streaming

## Constitution Check

- **I. Python-First Reference**: Not directly applicable as this is Go-only OpenAI subscription/Codex routing behavior introduced in the Go codebase. Implementation should still avoid breaking Python-compatible external API payloads.
- **II. Feature Parity**: Pass. External OpenAI-compatible payload contracts stay compatible; image schema remains unchanged.
- **III. Research Before Build**: Pass. Research documented in [research.md](./research.md), including repo reality, official Go stdlib docs, and rendezvous hashing sources.
- **IV. Failing-Tests-First Development**: Pass for plan. Concrete failing tests are listed below and tasks require writing/running them before implementation.
- **V. Go Best Practices & Idioms**: Pass. Plan uses small helper functions in existing handler package, stdlib hashing, and existing sticky strategy abstractions.
- **VI. No Stale Knowledge**: Pass. Technical claims are backed by repo reads and external docs/search in [research.md](./research.md).
- **VII. sqlc-First Database Access**: Pass. No database queries or migrations are planned.

## Project Structure

### Documentation

```text
specs/HO-2360-tianji-codex-session-sticky-routing-rendezvous/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── codex-routing.md
├── checklists/
│   └── requirements.md
├── tasks.md
└── analyze.md
```

### Source Code

```text
internal/proxy/handler/
├── openai_subscription_routing.go
├── openai_subscription_routing_test.go
├── responses.go
├── responses_compact.go
├── chatgpt_codex_backend.go
├── chatgpt_codex_backend_test.go
└── chatgpt_codex_wildcard_test.go

internal/provider/chatgptcodex/
├── payload.go
├── transport.go
└── transport_test.go

internal/config/
└── *_test.go
```

**Structure Decision**: Keep implementation in `internal/proxy/handler` because routing state, credential selection, usage cache, and entrypoint handlers already live there. Use `internal/provider/chatgptcodex` only for existing payload/schema verification; do not move routing into provider transport.

## Failing Tests

### User Story 1 Tests

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestCodexSessionIdentity_DirectClientMetadataSessionID` | `internal/proxy/handler/openai_subscription_routing_test.go` | direct `client_metadata.session_id` trims and returns source `direct` | US1 AS-1, FR-001, FR-002 |
| `TestCodexSessionIdentity_NestedTurnMetadataSessionID` | `internal/proxy/handler/openai_subscription_routing_test.go` | nested `client_metadata["x-codex-turn-metadata"]` JSON session id is extracted when direct id absent | US1 AS-2, FR-001 |
| `TestCodexStickySession_ReusesSelectableCredential` | `internal/proxy/handler/openai_subscription_routing_test.go` | repeated same session orders same first credential while cached usage remains selectable | US1 AS-1, AS-3, FR-004, FR-007 |

### User Story 2 Tests

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestCodexRendezvousSelect_DeterministicSameSession` | `internal/proxy/handler/openai_subscription_routing_test.go` | same session and same candidate set always select same credential | US2 AS-1, FR-006 |
| `TestCodexRendezvousSelect_SpreadsMultipleSessions` | `internal/proxy/handler/openai_subscription_routing_test.go` | deterministic session sample maps to more than one credential | US2 AS-1, SC-002 |
| `TestCodexRendezvousSelect_RemovingCredentialOnlyRemapsAffectedSessions` | `internal/proxy/handler/openai_subscription_routing_test.go` | removing one candidate only remaps sessions previously assigned to that candidate | US2 AS-2, SC-003 |
| `TestCodexStickySession_ReselectsWhenCredentialOverGate` | `internal/proxy/handler/openai_subscription_routing_test.go` | over-gate sticky credential is not reused and affected session reselects from remaining selectable credentials | US2 AS-3, FR-007 |

### User Story 3 Tests

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestCodexSessionIdentity_MalformedNestedMetadataFallsBackSafely` | `internal/proxy/handler/openai_subscription_routing_test.go` | malformed nested JSON returns source `parse_error`, empty session id, and no panic | US3 AS-2, Edge Cases |
| `TestCodexSessionIdentity_BlankSessionIDIsMissing` | `internal/proxy/handler/openai_subscription_routing_test.go` | blank direct/nested session id is treated as missing | US3 AS-1, FR-002 |
| `TestCodexSessionSticky_MissingSessionUsesExistingRouteKey` | `internal/proxy/handler/openai_subscription_routing_test.go` | missing session id uses existing `openAISubscriptionRouteKey` org / route-class track | US3 AS-1, FR-005 |
| `TestChatGPTCodexBackendCompletion_PassesSessionIdentityToRouting` | `internal/proxy/handler/chatgpt_codex_backend_test.go` | chat completion and streaming with `req.Metadata` use session-aware first credential | FR-008 |
| `TestResponsesHTTPAndCompact_PassSessionIdentityToRouting` | `internal/proxy/handler/responses_test.go` | responses HTTP and compact decode payload before session-aware ordering | FR-008 |
| `TestResponsesWebSocket_PassesCreateFrameSessionIdentityToRouting` | `internal/proxy/handler/responses_test.go` | WebSocket `response.create` frame payload drives session-aware ordering | FR-008 |
| `TestChatGPTCodexBackendImageGeneration_RemainsOrgLevelRouting` | `internal/proxy/handler/chatgpt_codex_wildcard_test.go` | image generation/edit still use existing org-level candidate ordering and reject unknown extra params | US3 AS-3, FR-009 |

### Verification Command

```bash
go test ./internal/proxy/handler ./internal/provider/chatgptcodex ./internal/config
```

## Complexity Tracking

No constitution violations or extra complexity exceptions.
