# Data Model: HO-1343 shared sticky routing strategy

## StickyCandidate

Provider-neutral view of one selectable candidate.

- `ID`: stable non-secret ID. Claude uses a token cache key, OpenAI subscription uses credential ID.
- `OriginalIndex`: deterministic fallback order from the provider adapter.
- `Metadata`: provider-owned opaque data, only visible to the adapter.

Rules:

- Must not contain raw API keys, bearer tokens, refresh tokens, encrypted credential values, or raw upstream payloads.
- Must remain stable across one selection call.

## StickyTrack

Shared sticky state entry.

- `TrackKey`: provider adapter route key.
- `SelectedID`: stable candidate ID selected last time.
- `SelectionMetadata`: provider-owned snapshot of state at selection time.

Rules:

- Claude metadata includes 5h reset timestamp needed to detect reset-cycle re-evaluation.
- Codex metadata may include cached primary/weekly reset/status fingerprints when fresh snapshots are used.
- Missing metadata must not panic; it means reuse can fall back to provider policy defaults.

## StickyPolicy

Provider adapter contract used by the shared core.

- `TrackKey(now, request)`: returns route-specific track key.
- `CandidateID(candidate)`: returns stable non-secret ID.
- `CanReuse(selected, candidates, metadata, now)`: decides whether current sticky selection can be reused.
- `ShouldReselect(selected, metadata, now)`: can force re-evaluation when provider state changes.
- `Score(candidate, now)`: returns comparable provider-specific score when known.
- `Fallback(candidates)`: returns deterministic order when scores are unknown or tied.

## Claude Policy State

Inputs:

- `RateLimitStore.Get(RateLimitCacheKey(apiKey))`
- optional bounded `RateLimitDB` backfill
- Anthropic 5h / 7d / 7d_sonnet utilization and reset timestamps
- model class from `isSonnetModel(modelName)`

Selection metadata:

- selected candidate token cache key
- selected 5h reset timestamp

Rules:

- reuse while candidate remains available and 5h reset timestamp has not changed.
- reselect when 5h reset differs.
- score known candidates by earliest 7d reset.
- unknown reset loses to known reset; all unknown falls back deterministically.

## Codex Policy State

Inputs:

- OpenAI subscription candidate credential IDs
- existing `OpenAIQuotaState` gate from response headers
- `OpenAISubscriptionCodexUsageCache.getFresh(credentialID, now)` or equivalent safe read method
- current org/model route key

Selection metadata:

- selected credential ID
- optional primary 5h reset/status fingerprint
- optional weekly reset/status fingerprint

Rules:

- reuse while candidate remains available and policy allows reuse.
- when fresh cached snapshots exist, prefer lower primary 5h utilization; use weekly utilization/reset as tie-break or longer-window pressure signal.
- re-evaluate when selected fresh snapshot status/window indicates exhaustion or a reset fingerprint changes.
- missing/stale/backoff/unavailable snapshots are unknown, not failures.
- unknown snapshot path falls back to existing sticky/round-robin deterministic behavior.

## Hot-path No-Fetch Guard

The proxy request path may read cache state but must not invoke:

- `chatgptcodex.UsageFetcher.Fetch`
- `chatgptcodex.UsageClient.Fetch`
- direct `GET /backend-api/wham/usage`

UI/manual/background refresh remains allowed to call those APIs outside proxy routing.
