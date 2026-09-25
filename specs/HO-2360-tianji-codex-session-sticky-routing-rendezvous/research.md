# Research: Codex session sticky rendezvous routing

## Decision: Codex-only session identity extraction from metadata

**Decision**: Extract `session_id` from `client_metadata.session_id` first, then parse `client_metadata["x-codex-turn-metadata"]` as JSON and read `session_id`. Trim whitespace; blank means missing. Do not substitute `thread_id`, `turn_id`, or `window_id`.

**Rationale**: Linear comments and memory bridge evidence for HO-2360 show recent live `gpt-5.5` / `responses` requests carry session id in both direct and nested forms. Repo reality confirms HTTP responses, WebSocket create frames, compact responses, and chat completion metadata are available before current Codex candidate ordering calls.

**Alternatives considered**:

- Use `thread_id` / `turn_id` fallback: rejected because issue contract explicitly forbids using them as sticky keys.
- Add image request metadata: rejected because current image payload validation rejects unknown extra params and image request structs have no session metadata contract.

## Decision: Session-aware track key only for Codex backend

**Decision**: Keep existing `openAISubscriptionRouteKey(ctx, params)` for current behavior. Add a Codex-specific session-aware key/helper used only by Codex subscription ordering. With session id, use `openai-subscription:<org>:codex-session:<session_id>:<route_class>`; without session id, keep `openai-subscription:<org>:<route_class>`.

**Rationale**: `internal/proxy/handler/openai_subscription_routing.go` currently routes sticky selection with org / route-class only. Changing global key behavior would risk non-Codex OpenAI subscription and native upstream routing, which are explicitly out of scope.

**Alternatives considered**:

- Replace `openAISubscriptionRouteKey` globally: rejected due non-Codex blast radius.
- Add a config toggle: rejected by issue scope; no UI/config setting is required.

## Decision: Rendezvous hashing for new session sticky selection

**Decision**: For a new session sticky entry, compute deterministic scores from `session_id + "\x00" + credential_id` for all currently selectable credentials and choose the highest score.

**Rationale**: Rendezvous / highest random weight hashing preserves deterministic key-to-backend assignment and minimizes remapping when backend membership changes. HO-2360 live distribution simulation showed 500 sessions across 7 configured credentials would spread roughly 68-79 sessions per credential, while current observed routing concentrated across only 3 tokens.

**External verification**:

- Brave web search result for rendezvous hashing says remapping is required only for objects mapped to the failed site and disruption is minimal.
- IETF weighted HRW draft search result describes HRW as mapping objects to servers uniformly while being minimally affected by server-set changes.
- Public GitHub examples for HRW/rendezvous hashing exist in routing/load-balancing contexts, including prior HO-2360 comments.

**Alternatives considered**:

- Current org-level sticky: rejected because it creates hot credentials for unrelated sessions.
- Per-request round-robin: rejected because it breaks session affinity.
- `hash(session_id) % n`: rejected because adding/removing credentials remaps many sessions.

## Decision: Use Go stdlib hashing and byte conversion

**Decision**: Use `crypto/sha256.Sum256` and compare a fixed prefix of the digest as unsigned big-endian using `encoding/binary`.

**Rationale**: Official Go docs state `crypto/sha256` implements SHA224/SHA256 and exposes `Sum256(data []byte) [Size]byte`. Official `encoding/binary` docs expose `BigEndian` and byte-order utilities for fixed-size values. No third-party dependency is needed.

**Alternatives considered**:

- Add a rendezvous hashing library: rejected because candidate scoring is simple and adding dependency is unnecessary.
- Use non-cryptographic hash: possible, but sha256 stdlib is deterministic, stable, and already sufficient for tiny candidate sets.

## Decision: Keep existing usage gate and reuse semantics

**Decision**: Keep existing `codexStickyCanReuse...`, usage cache hydration, rate-limit filtering, disabled/auth-failed filtering, and `filterOpenAISubscriptionCodexUsageSelectable` behavior. Rendezvous selection runs only over currently selectable credentials for new or invalidated sticky entries.

**Rationale**: Existing code already encodes reset and gate behavior. The issue requires only affected sessions to move when their sticky credential becomes unavailable or over gate.

**Alternatives considered**:

- Recompute rendezvous on every request: rejected because it would bypass intended sticky reuse semantics.
- Ignore unknown usage: keep current fallback behavior only where existing policy already allows it.

## Repo Reality Evidence

- `internal/proxy/handler/openai_subscription_routing.go` currently defines `orderOpenAISubscriptionCodexCandidates(ctx, params, resolved)` with no session parameter.
- `stickyOpenAISubscriptionSelect(ctx, params, candidates)` currently uses `openAISubscriptionRouteKey(ctx, params)` and `selectLowestStickyScore`.
- `internal/proxy/handler/responses.go` decodes HTTP and WebSocket payload before ordering candidates.
- `internal/proxy/handler/responses_compact.go` decodes compact payload before ordering candidates.
- `internal/proxy/handler/chatgpt_codex_backend.go` receives `req *model.ChatCompletionRequest`; `internal/provider/chatgptcodex/payload.go` preserves `req.Metadata`.
- `internal/provider/chatgptcodex/transport.go` image generation/edit payload builders have no session metadata and reject unknown extra params.

## UI / Schema Evidence

- HO-2360 issue explicitly says no required UI change and forbids `codex_session_sticky` toggle, model edit form setting, credential setting, or new routing page.
- Existing `/ui/usage` and credential pages are observability alignment surfaces only if a later issue asks for UI observability.
- No DB table, migration, or persisted session id is required.
