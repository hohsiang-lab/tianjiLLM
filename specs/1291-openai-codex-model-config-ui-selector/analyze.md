# Analyze: HO-1291 scope and consistency

## Artifact Consistency

| Check | Status | Evidence |
|---|---:|---|
| Linear scope covered | PASS | Spec covers config/model parameter, Models UI explicit transport choice, API-key routing preservation, and invalid mix prevention. |
| Repo reality aligned | PASS | Plan was refreshed after `origin/main` advanced to `3fa617a`; it now treats typed config, runtime decode, and backend route hook as main baseline, with UI selector and validation hardening remaining in scope. |
| HO-1288 boundary preserved | PASS | This issue only persists/selects transport; backend HTTP transport remains HO-1288. |
| HO-1290 boundary preserved | PASS | Response/error normalization remains out of scope. |
| Tests-first | PASS | Tasks start with RED config/UI/E2E tests. |
| Security | PASS | DOM/token leak constraints inherited from HO-1172 and restated. |
| Mockscreen scope review | PASS | Waiting recovery generated and posted desktop/mobile mockscreens to Discord thread `1502886173721493515`; screenshots show Models table summary, Add Model dialog, subscription credentials selector, explicit transport selector, and custom API base conflict warning. |

## Explicit Issue Behavior Map

| Linear requirement | Spec/plan mapping |
|---|---|
| Add config/model parameter or provider mode for subscription Codex transport | FR-001 to FR-005, plan D1-D3, data-model field; latest main already supplies the typed field/runtime decode, and HO-1291 completes UI plus stricter validation. |
| Update Models UI to make transport choice explicit when OpenAI subscription credentials are selected | FR-007 to FR-010, contract route/form behavior, tasks T016-T019. |
| Keep existing `openai/*` API-key routing valid | User Story 4, FR-013, tasks T006. |
| UI/config tests cover creating or editing subscription-backed wildcard with Codex transport | SC-001 to SC-004, tasks T001-T005. |
| Model cannot accidentally combine subscription credentials with wrong Platform API transport | FR-004 to FR-006, invalid combinations contract. |

## Fatal / Critical / Owner Input

- Fatal: 0
- Critical: 0
- Owner input required before Waiting: 0

## Waiting Artifact-Review Recovery: Mockscreen Gate

### Root Cause

Original Todo close-out moved HO-1291 to `Waiting` with docs-only PR #164 but treated the issue primarily as config/spec planning. Evidence: the Discord thread had no attachments, this `analyze.md` had no mockscreen/scope alignment section, and the Linear evidence comment listed docs/diff gates only. That violated the current `speckit-workflow` UI mockscreen gate because HO-1291 explicitly changes a Models UI selector.

### Mockscreen Evidence

- Desktop mockscreen: posted to Discord thread `1502886173721493515` as message `1502900111691223150`.
- Mobile mockscreen: posted to Discord thread `1502886173721493515` as message `1502900131148595263`.
- Local source files are outside git under `/Users/n0rmanc/.openclaw/media/ho-1291-mockscreen.html`, `/Users/n0rmanc/.openclaw/media/ho-1291-desktop.png`, and `/Users/n0rmanc/.openclaw/media/ho-1291-mobile.png`.
- Visual sanity check passed: required selector structure is visible, desktop and mobile text is readable, no important UI overlaps, and images are not committed to the PR branch.

### Scope Alignment

The mockscreen matches the SpecKit scope:

- Create Model dialog includes `OpenAI Subscription Credentials`.
- New `OpenAI Subscription Transport` selector exposes `Platform direct HTTP` and `ChatGPT Codex backend`.
- Saved table summary shows subscription credential count and transport label.
- Custom API Base conflict warning remains visible for selected subscription credentials.
- API-key model row remains represented without subscription transport requirement.

## Notes For Implementation

The only compatibility decision to record during implementation is legacy handling for existing subscription rows without transport. New UI/config saves must be explicit either way.

## Waiting Artifact-Review Recovery: Main Drift

Norman asked whether the spec needed an update after main changed. Live check found PR #164 remains mergeable and docs-only, but the artifact content was stale because `origin/main` now includes HO-1288/HO-1290 baseline code:

- `internal/config/config.go` already has `OpenAISubscriptionTransport`.
- `internal/proxy/handler/runtime_model_source.go` already decodes `openai_subscription_transport`.
- `internal/proxy/handler/openai_subscription_resolution.go` already knows `chatgpt_codex_backend`.
- `internal/ui/handler_models.go` and `internal/ui/pages/models.templ` still lack Create/Edit parsing, selector, prefill, removal, and safe summary.
- `internal/config/validate.go` still lacks missing/unknown transport validation for subscription-backed rows.

This revision updates SpecKit docs only so future implementation starts from current main reality.

## Waiting CI Recovery: Result-Screen Gate

Latest current-head CI run `25629676219` passed on PR head `c518d106573a41864db27d195590c28bfcf58cd4`: `lint`, `test`, `e2e`, and `build` succeeded; `docker` was skipped by workflow condition.

Implementation result-screen evidence was captured from the actual Playwright-backed Models UI before moving to `Waiting Merge`:

- Desktop result screen: Discord thread message `1503027635394056203`; local file `/Users/n0rmanc/.openclaw/media/ho-1291-result-desktop.png`.
- Mobile result screen: Discord thread message `1503027639906992262`; local file `/Users/n0rmanc/.openclaw/media/ho-1291-result-mobile.png`.
- Sanity check passed: desktop Add Model dialog shows selected OpenAI subscription credential and `OpenAI Subscription Transport = ChatGPT Codex backend`; mobile Edit Model dialog shows the prefilled selected credential and transport selector without important clipping.
