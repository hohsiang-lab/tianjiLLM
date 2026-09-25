# Quickstart: Validate Codex session sticky rendezvous routing

## Prerequisites

- Work from this issue branch.
- Do not implement production code until failing tests are written and confirmed to fail.

## Failing-Test Pass

```bash
go test ./internal/proxy/handler ./internal/provider/chatgptcodex ./internal/config
```

Expected before implementation: newly added HO-2360 tests compile and fail because session-aware extraction/routing is not implemented yet.

## Implementation Validation

After implementation:

```bash
go test ./internal/proxy/handler ./internal/provider/chatgptcodex ./internal/config
```

Expected after implementation:

- Same session repeatedly selects the same first credential while selectable.
- Different sessions distribute across multiple credentials.
- Removing one credential only remaps affected sessions in deterministic tests.
- Missing or malformed session metadata falls back to org-level sticky key.
- Responses HTTP, Responses WebSocket, compact, chat completion, and chat streaming pass session identity into routing.
- Image generation/edit behavior remains unchanged.

## Manual Diff Sanity

Before handoff, verify final diff does not include:

- UI setting/toggle/page changes.
- DB schema or migration changes.
- Image request schema expansion.
- Non-Codex routing behavior changes.
- Raw token/session payload logging.
