# Requirements Checklist: HO-1259

**Purpose**: Validate SpecKit artifacts before implementation.

## Completeness

- [x] Linear root cause and product direction captured.
- [x] `public_base_url` removal is explicit, including `proxy.yml` / proxy YAML config artifacts.
- [x] Default localhost redirect URI is explicit.
- [x] Stored exact redirect URI requirement is explicit.
- [x] `/ui/credentials` paste modal and protected submit endpoint requirement is explicit.
- [x] Direct callback preservation is explicit.
- [x] Secret redaction requirement covers pasted URL, code, state, verifier, and tokens.
- [x] No owner input is required for scope.

## Testability

- [x] RED-first tests are listed before implementation tasks.
- [x] Redirect URI identity has direct tests.
- [x] Pasted URL parser has direct tests.
- [x] Replay/one-time state behavior has direct tests.
- [x] Secret leakage has sentinel tests.
- [x] E2E covers the user-visible paste flow.

## Scope Boundaries

- [x] Hosted redirect whitelist is out of scope.
- [x] Raw token/auth cache paste/import is out of scope.
- [x] Credential CRUD/quota/refresh/spend features remain out of scope.
- [x] No production code is changed in Todo phase.
