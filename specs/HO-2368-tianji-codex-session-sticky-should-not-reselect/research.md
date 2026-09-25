# Research: HO-2368 Codex session sticky reuse across primary reset

## Decision 1: Detect session-aware sticky tracks by route-key marker

**Decision**: Treat OpenAI subscription route keys containing `:codex-session:` as session-aware Codex sticky tracks.

**Rationale**: Repo code builds session route keys with `openAISubscriptionCodexSessionRouteKey()` as `openai-subscription:<org>:codex-session:<session_id>:<route_class>`. Existing log redaction also keys off the same marker in `safeOpenAISubscriptionTrackLogValue()`, so the marker is already canonical inside this file.

**Alternatives considered**:

- Add new metadata to sticky entries: rejected for this bug because the route key already carries the required track class and metadata migration would widen scope.
- Change all Codex sticky reuse behavior: rejected because HO-2368 explicitly preserves non-session fallback behavior unless separately decided.

## Decision 2: Preserve unknown-current-metadata behavior

**Decision**: Keep current conservative handling for `!current.Known`; do not broaden session reuse when usage metadata is unknown.

**Rationale**: HO-2368 asks to reuse when usage metadata is known enough and the candidate remains selectable. Existing tests already cover missing-current-snapshot re-evaluation and missing-snapshot fallback behavior. Changing unknown behavior would be a separate risk because usage gates cannot be evaluated.

**Alternatives considered**:

- Always reuse session stickiness when current metadata is unknown: rejected because it can keep a session on a credential whose Codex usage gate cannot be evaluated.

## Decision 3: Preserve candidate filtering and usage gates

**Decision**: Only change the primary-reset equality requirement for session-aware route keys after the current candidate is known and selectable. Keep existing upstream credential filtering, `Selectable=false` handling, primary/weekly thresholds, and unavailable/exhausted snapshot behavior.

**Rationale**: Repo code filters disabled/missing credentials before sticky selection, and `filterOpenAISubscriptionCodexUsageSelectable()` removes known non-selectable candidates. The reuse helper also checks `!current.Selectable`; this must remain the hard release valve for sessions.

**Alternatives considered**:

- Add special-case bypass in candidate filtering: rejected because that would risk routing to disabled, removed, or usage-gated credentials.

## Decision 4: No new dependency

**Decision**: Use existing Go standard library string matching for route-key classification.

**Rationale**: `internal/proxy/handler/openai_subscription_routing.go` already imports `strings`, and official Go docs for `strings.Contains` define the needed substring predicate. No library selection is required.

**Source**: `https://pkg.go.dev/strings#Contains`

## Decision 5: SpecKit script limitation

**Decision**: Keep Sima's required `HO-2368-...` branch/worktree identity and manually fill SpecKit templates instead of renaming the branch to the script's `NNN-feature` convention.

**Rationale**: `.specify/scripts/bash/setup-plan.sh --json` rejected the live issue branch with `ERROR: Not on a feature branch`. Sima workspace identity gate requires worktree basename and branch to start with `HO-2368`, so the branch cannot be renamed for script compatibility.

**Alternatives considered**:

- Rename branch to `2368-...`: rejected because SpecKit script requires exactly three leading digits and Sima requires `HO-2368`.
- Patch SpecKit scripts in this planning PR: rejected as unrelated tooling change.
