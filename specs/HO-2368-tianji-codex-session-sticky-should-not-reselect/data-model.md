# Data Model: HO-2368 Codex session sticky reuse across primary reset

No persistent data model or database schema changes are required.

## Runtime Entities

### OpenAI Subscription Route Key

- **Shape, session-aware**: `openai-subscription:<org-scope>:codex-session:<session-id>:<route-class>`
- **Shape, fallback**: `openai-subscription:<org-scope>:<route-class>`
- **Identity rule**: A key containing `:codex-session:` is governed by session-affinity reuse semantics.
- **Privacy rule**: Raw `session-id` must not be emitted in logs; logged keys must use the existing `codex-session:sha256:<digest>` redaction shape.

### Sticky Strategy Entry

- **SelectedID**: Previously selected credential id.
- **Metadata**: Previous `codexStickyMetadata`, including the previous `PrimaryResetAt`.
- **Lifecycle**: Created after selection; reused only if the selected credential is still in the current candidate pool and policy permits reuse.

### Sticky Strategy Candidate

- **ID**: Current credential id.
- **Metadata**: Current `codexStickyMetadata`.
- **Reuse rule**: For session-aware keys, known and selectable current metadata is enough to reuse even when `PrimaryResetAt` differs.

### Codex Sticky Metadata

- **Known**: Usage snapshot has enough data to evaluate current usage.
- **Available**: Snapshot status says the credential is available for Codex.
- **Selectable**: Credential is under primary and secondary/weekly gates and is safe for selection.
- **PrimaryResetAt**: Primary window reset fingerprint; no longer a reuse requirement for session-aware tracks.
- **SecondaryResetScore**: Weekly/secondary reset score used for candidate ordering and tie-breaking.
- **Score**: Existing selection score.

## State Transitions

```text
session-aware sticky + selected candidate present
  -> current metadata unknown
      -> keep existing conservative unknown behavior
  -> current metadata known and selectable
      -> reuse selected credential even if PrimaryResetAt changed
  -> current metadata known and not selectable
      -> reselect from remaining selectable candidates

fallback sticky + selected candidate present
  -> current metadata known
      -> preserve existing PrimaryResetAt equality requirement
```

## Out Of Scope

- No database table, sqlc query, migration, or persistent cache schema change.
- No route-key shape change.
- No credential model/config shape change.
