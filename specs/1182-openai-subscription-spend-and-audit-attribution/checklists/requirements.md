# Requirements Checklist: HO-1182

## Scope

- [x] Spec covers spend attribution for OpenAI subscription traffic.
- [x] Spec covers audit attribution for connect/refresh/delete/test/disable.
- [x] Spec preserves existing API-key behavior.
- [x] Spec excludes UI, billing estimation, and sibling OpenAI subscription implementation slices.

## Security

- [x] Tokens, JWTs, bearer strings, encrypted credential values, and raw account payloads are explicitly forbidden from all sinks.
- [x] Audit payloads use narrow safe fields before redaction.
- [x] Error/callback text must use shared redaction.

## Testing

- [x] Tests start before production implementation.
- [x] Successful subscription attribution is covered.
- [x] Multi-credential failover attribution is covered.
- [x] Failure attribution and lifecycle audit are covered.
- [x] API-key regression is covered.
- [x] No-real-OpenAI rule is explicit.

## Owner Scope

- [x] No owner input required for Todo scope confirmation.
