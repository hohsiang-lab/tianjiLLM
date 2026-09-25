# Codex Responses Lite Contract Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Normalize Tianji's prepared ChatGPT Codex Responses Lite payloads to the
upstream Codex wire contract while preserving non-Lite behavior.

**Architecture:** Keep the conversion at `PrepareResponsesPayload` and
`PrepareCompactResponsesPayload`, so HTTP, WebSocket, image, compact, and
credential-retry callers share one immutable normalized body. Build the Lite
`additional_tools` input prefix from the already decoded generic Responses
payload, omit unsupported hosted tools, and leave routing/header selection
unchanged.

**Tech Stack:** Go 1.26, `encoding/json`, existing `testify` tests, `go test`,
`golangci-lint`, Makefile CI commands.

**Spec:** `docs/superpowers/specs/2026-08-22-codex-responses-lite-contract.md`

## Global Constraints

- Do not change non-Lite wire payloads.
- Do not add dependencies or a new abstraction layer.
- Preserve caller payload ownership and retry body identity.
- Do not modify or delete the untracked `podslog` incident log.
- Do not deploy or mutate Kubernetes as part of this source fix.

---

### Task 1: Lock the Lite wire contract with failing tests

**Files:**
- Modify: `internal/provider/chatgptcodex/transport_test.go`

- [ ] **Step 1: Add a normal Responses Lite contract test**

Assert that a payload with instructions, flat function/custom tools, an
existing namespace, and a hosted tool emits an `additional_tools` prefix,
developer instructions input, one merged `functions` namespace, no top-level
`instructions`/`tools`, `tool_choice=auto`, `parallel_tool_calls=false`, and
`reasoning.context=all_turns`.

- [ ] **Step 2: Add compact and non-Lite regression assertions**

Assert compact omits `store`/`stream` after the same Lite conversion, and a
non-Lite request preserves top-level instructions/tools and caller reasoning.

- [ ] **Step 3: Run the targeted tests and confirm RED**

Run:

```bash
go test ./internal/provider/chatgptcodex -run 'Test(ResponsesLite|BuildCompactRequest|BuildResponsesRequest)' -count=1
```

Expected: the new Lite contract assertions fail because the current body still
contains top-level `instructions`/`tools` and lacks `additional_tools`.

### Task 2: Implement the minimal shared Lite normalization

**Files:**
- Modify: `internal/provider/chatgptcodex/responses_payload.go`

- [ ] **Step 1: Normalize the input array and prefix**

Clone the prepared payload, convert string input through the existing helper,
prepend `additional_tools`, then prepend a developer message when instructions
are non-empty.

- [ ] **Step 2: Normalize tools**

Convert flat `function` and `custom` tools into a single `functions` namespace,
merge an existing `functions` namespace, preserve other namespaces, and omit
hosted tools. Treat client-executed `tool_search` as valid only when its
execution field is `client`.

- [ ] **Step 3: Force Lite-only fields and remove incompatible fields**

Set `tool_choice=auto`, `parallel_tool_calls=false`, and
`reasoning.context=all_turns`; delete top-level `instructions` and `tools`.
Keep the existing normal Responses `store`/`stream` defaults and let compact
remove them afterward.

- [ ] **Step 4: Run targeted tests and confirm GREEN**

Run the same targeted command from Task 1, then run the full provider package:

```bash
go test ./internal/provider/chatgptcodex -count=1
```

### Task 3: Review, validate, and publish

**Files:**
- No additional source files unless tests expose a regression.

- [ ] **Step 1: Run formatting, lint, tests, and build**

```bash
gofmt -w internal/provider/chatgptcodex/responses_payload.go internal/provider/chatgptcodex/transport_test.go
make check
```

- [ ] **Step 2: Run ponytail-review on the diff**

Remove only unnecessary complexity identified by the review; correctness and
contract validation stay in scope for the normal review.

- [ ] **Step 3: Commit and push the intended files**

Stage only the plan/spec, payload implementation, and tests. Keep `podslog`
untracked. Push branch `codex/fix-codex-responses-lite-contract`.

- [ ] **Step 4: Create the pull request and monitor CI**

Create a PR against `main`, inspect all required checks, and fix any CI failure
on the branch before merging.

- [ ] **Step 5: Merge and verify remote state**

Merge only after required checks are green, then verify the PR is merged and
the remote `main` contains the merge commit.
