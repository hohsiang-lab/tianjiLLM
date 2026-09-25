# Data Model: Codex session sticky rendezvous routing

No persisted database model is introduced.

## Runtime Concepts

### CodexSessionIdentity

Represents a safe, normalized routing identity extracted from request metadata.

Fields:

- `SessionID`: trimmed Codex session id. Empty means no usable session identity.
- `Source`: one of `direct`, `x-codex-turn-metadata`, `missing`, `parse_error`.

Validation:

- Trim whitespace before use.
- Do not accept `thread_id`, `turn_id`, or `window_id` as substitutes.
- Do not log raw metadata payload.

### CodexSessionRouteKey

Represents the sticky track key for Codex subscription routing.

Rules:

- With session id: `openai-subscription:<org>:codex-session:<session_id>:<route_class>`.
- Without session id: existing `openai-subscription:<org>:<route_class>`.
- Existing non-Codex route keys remain unchanged.

### SelectableCredentialPool

Represents the already-filtered list of credentials eligible for selection.

Rules:

- Must exclude disabled, malformed, missing, rate-limited, auth-failed, and over-gate credentials according to existing logic.
- Rendezvous hashing runs only after this pool is resolved.

### RendezvousSelection

Represents the deterministic first-choice credential for a new session sticky entry.

Rules:

- Input key: normalized `session_id`.
- Candidate key: credential id, never raw token or credential value.
- Score input: `session_id + "\x00" + credential_id`.
- Selection: highest unsigned deterministic score wins.

## State Transitions

```text
missing sticky entry + session id + selectable pool
  -> rendezvous select credential
  -> save sticky entry under session-aware track

existing sticky entry + selected credential still reusable
  -> reuse selected credential

existing sticky entry + selected credential no longer reusable/selectable
  -> reselect from remaining selectable pool

missing/invalid session id
  -> use existing org-level sticky key and existing selection behavior
```
