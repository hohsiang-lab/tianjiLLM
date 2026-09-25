# Contract: Codex Session Sticky Reuse Policy

## Scope

This contract describes internal routing behavior for OpenAI subscription Codex backend sticky selection. It is not a public HTTP API contract.

## Inputs

- `trackKey`: OpenAI subscription sticky route key.
- `entry`: sticky strategy entry for a previously selected credential.
- `candidate`: current candidate matching the previous selected credential.
- `candidate.Metadata`: `codexStickyMetadata` for current usage state.

## Session-Aware Track

A track is session-aware when:

```text
strings.Contains(trackKey, ":codex-session:")
```

For a session-aware track, reuse returns `true` when all are true:

- the previous selected credential is still present as the current candidate;
- current metadata is known;
- current metadata is selectable.

For a session-aware track, reuse returns `false` when:

- current metadata is known and `Selectable=false`;
- existing unknown-metadata policy says reuse is unsafe;
- the selected credential is absent from the current candidate pool, disabled, refresh-failed, removed from config, rate-limited, or otherwise filtered out before candidate reuse.

`PrimaryResetAt` inequality alone MUST NOT make a session-aware track return `false`.

## Fallback Track

A track without `:codex-session:` keeps existing behavior:

- if previous metadata has no primary reset fingerprint, reuse may continue;
- otherwise, `previous.PrimaryResetAt == current.PrimaryResetAt` remains the reuse requirement.

## Observability

- Re-selection caused by `Selectable=false` may continue to log `reason=not_selectable`.
- Fallback tracks may continue to log `reason=primary_reset_changed`.
- Session-aware tracks must not log a primary-reset-only reselect reason when the credential is reused.
- Any logged session-aware `trackKey` must be redacted through the existing session-hash shape.
