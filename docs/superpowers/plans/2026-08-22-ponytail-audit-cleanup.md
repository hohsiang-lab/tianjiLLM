# Ponytail Audit Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove all evidence-backed Ponytail over-engineering findings from tianjiLLM while preserving active runtime behavior and config overflow compatibility.

**Architecture:** Delete only packages, generated assets, aliases, dependencies, and typed config fields proven unused by production imports or selectors. Keep active handlers, dynamic icon data, and unknown configuration keys. Work in six independently testable slices with a fresh audit after each slice.

**Tech Stack:** Go 1.26, templ/templUI generated Go, Go modules, `ccc`, Graphify, `go test`, `golangci-lint`, Tailwind build.

**Spec:** `docs/superpowers/specs/2026-08-22-ponytail-audit-cleanup-design.md`

## Global Constraints

- Do not change active request routing, provider behavior, UI handlers, or database prompt-management code.
- Do not remove `internal/ui/components/icon/icon_data.go`, `Icon()`, or dynamic `icon.Icon(name)`.
- Preserve unknown YAML keys through each config struct's existing `Overflow` map.
- Use `git rm` for repository deletions; do not use broad filesystem deletion.
- Run focused validation and `git diff --check` before each slice commit.
- Refresh CCC after each slice and keep generated Graphify output untracked.

---

### Task 1: Remove unused generated UI components and assets

**Files:**
- Delete: `internal/ui/components/aspectratio/`
- Delete: `internal/ui/components/avatar/`
- Delete: `internal/ui/components/dropdown/`
- Delete: `internal/ui/components/separator/`
- Delete: `internal/ui/components/skeleton/`
- Delete: `internal/ui/assets/js/avatar.min.js`
- Delete: `internal/ui/assets/js/dropdown.min.js`
- Test: existing templ generation and `go test ./internal/ui/...`

**Interfaces:**
- Consumes: active templ imports and `templ generate ./internal/ui/...`.
- Produces: the same active UI packages without dormant component packages or scripts.

- [ ] **Step 1: Verify the deletion set is still unused**

```bash
rtk rg -n 'components/(aspectratio|avatar|dropdown|separator|skeleton)|avatar\.min\.js|dropdown\.min\.js' internal cmd
rtk go list -deps ./cmd/tianji | rtk rg 'internal/ui/components/(aspectratio|avatar|dropdown|separator|skeleton)'
```

Expected: no active source import or production dependency output.

- [ ] **Step 2: Delete only the verified paths**

```bash
git rm -r internal/ui/components/aspectratio \
  internal/ui/components/avatar \
  internal/ui/components/dropdown \
  internal/ui/components/separator \
  internal/ui/components/skeleton \
  internal/ui/assets/js/avatar.min.js \
  internal/ui/assets/js/dropdown.min.js
```

- [ ] **Step 3: Regenerate templ output and verify no files are recreated**

```bash
rtk templ generate ./internal/ui/...
git status --short
```

Expected: no deleted component returns and no unrelated generated diff.

- [ ] **Step 4: Run focused checks**

```bash
rtk go test ./internal/ui/... -count=1
rtk git diff --check
rtk ccc index
```

- [ ] **Step 5: Commit**

```bash
git add -u internal/ui
git commit -m "chore: remove unused UI components"
```

### Task 2: Prune unreferenced Lucide aliases

**Files:**
- Modify: `internal/ui/components/icon/icon_defs.go`
- Preserve: `internal/ui/components/icon/icon.go`
- Preserve: `internal/ui/components/icon/icon_data.go`
- Test: active UI package compilation and alias reference scan

**Interfaces:**
- Consumes: the 26 aliases referenced by active `.templ` and generated UI code.
- Produces: the same aliases and dynamic lookup behavior without unused variables.

- [ ] **Step 1: Capture the active alias set**

```bash
rtk rg -o 'icon\.[A-Z][A-Za-z0-9]*' internal/ui --glob '*.templ' --glob '*.go' \
  | rtk sort -u
```

Use this output plus direct package references to retain only the currently
used aliases; do not infer aliases from `icon_data.go` names.

- [ ] **Step 2: Remove only unused variable declarations**

Delete unreferenced `var Name = Icon("name")` declarations from
`icon_defs.go`; leave the file header and all retained declarations unchanged.

- [ ] **Step 3: Verify active icon paths**

```bash
rtk go test ./internal/ui/... -count=1
rtk rg -n 'Icon\(|icon_data|icon_defs' internal/ui/components/icon
rtk git diff --check
```

Expected: dynamic `Icon()` and `icon_data.go` remain present, and the UI
packages compile with every active alias.

- [ ] **Step 4: Refresh the code index and commit**

```bash
rtk ccc index
git add internal/ui/components/icon/icon_defs.go
git commit -m "chore: prune unused icon aliases"
```

### Task 3: Delete dormant prompt and RAG skeletons

**Files:**
- Delete: `internal/prompt/`
- Delete: `internal/rag/`
- Preserve: `internal/proxy/handler/rag.go`
- Test: remaining proxy handler packages and full Go compilation

**Interfaces:**
- Consumes: current proxy RAG passthrough endpoints and active DB prompt-management code.
- Produces: unchanged passthrough routes without dormant implementation packages.

- [ ] **Step 1: Prove active boundaries before deletion**

```bash
rtk rg -n 'internal/(prompt|rag)|PromptManagement|PromptManagementConfig' --glob '*.go'
rtk go list -deps ./cmd/tianji | rtk rg 'internal/(prompt|rag)' || true
```

Expected: no production importer of either package; prompt references are
limited to dormant config/package code or active DB handlers that remain.

- [ ] **Step 2: Delete dormant packages**

```bash
git rm -r internal/prompt internal/rag
```

- [ ] **Step 3: Verify the passthrough boundary and tests**

```bash
rtk rg -n 'func .*RAG|/rag|handle.*RAG' internal/proxy/handler/rag.go
rtk go test ./internal/proxy/... ./internal/config/... -count=1
rtk git diff --check
```

- [ ] **Step 4: Refresh CCC and commit**

```bash
rtk ccc index
git add -u internal
git commit -m "chore: remove dormant prompt and RAG packages"
```

### Task 4: Remove dormant SCIM package and proxy hook

**Files:**
- Delete: `internal/scim/`
- Modify: `internal/proxy/server.go`
- Modify: `go.mod`
- Modify: `go.sum`
- Delete: SCIM-only integration tests identified by the caller scan
- Test: `go test ./internal/proxy/... ./internal/scim/...` before deletion, then remaining proxy tests

**Interfaces:**
- Consumes: the current proxy server construction, which never initializes the optional SCIM hook.
- Produces: the same proxy routes without a nil SCIM route field or dormant dependency.

- [ ] **Step 1: Confirm the route hook is never initialized**

```bash
rtk rg -n 'NewSCIMServer|SCIM|scim|scimRouter' internal cmd test --glob '*.go'
rtk rg -n 'github.com/elimity-com/scim' go.mod go.sum
```

Expected: only dormant package/tests and the uninitialized proxy hook remain.

- [ ] **Step 2: Delete the package, tests, and optional hook**

Remove the SCIM-only files and the corresponding field/route registration in
`internal/proxy/server.go`; do not alter unrelated proxy routes.

- [ ] **Step 3: Remove the unused module**

```bash
rtk go mod tidy
```

Confirm the diff removes only SCIM's unused module graph entries.

- [ ] **Step 4: Run focused checks and commit**

```bash
rtk go test ./internal/proxy/... -count=1
rtk git diff --check
rtk ccc index
git add internal/proxy/server.go go.mod go.sum
git add -u internal/scim test
git commit -m "chore: remove unused SCIM integration"
```

### Task 5: Remove dormant secret manager and provider modules

**Files:**
- Delete: `internal/secretmanager/`
- Modify: `internal/config/` only if `SecretResolver` and `LoadWithSecrets` have no caller
- Modify: `go.mod`
- Modify: `go.sum`
- Delete: secret-manager-only tests
- Test: config loading and full package compilation

**Interfaces:**
- Consumes: `cmd/tianji/main.go`'s existing `config.Load()` path.
- Produces: identical default configuration loading without unused cloud/Vault provider modules.

- [ ] **Step 1: Prove resolver and provider callers**

```bash
rtk rg -n 'internal/secretmanager|LoadWithSecrets|SecretResolver|Load\(' internal cmd test --glob '*.go'
rtk rg -n 'cloud.google.com/go/(auth|secretmanager)|google.golang.org/api|aws-sdk-go-v2/service/secretsmanager|azsecrets|hashicorp/vault' go.mod internal
```

Expected: no production caller of the dormant resolver/provider path.

- [ ] **Step 2: Delete dormant implementation and its tests**

```bash
git rm -r internal/secretmanager
```

Remove the generic resolver seam only when Step 1 proves it is unused; keep
`config.Load()` and active config types unchanged.

- [ ] **Step 3: Prune provider dependencies**

```bash
rtk go mod tidy
```

Review the module diff and retain any dependency still imported by another
active package.

- [ ] **Step 4: Run config checks**

```bash
rtk go test ./internal/config/... ./cmd/tianji/... -count=1
rtk git diff --check
rtk ccc index
```

- [ ] **Step 5: Commit**

```bash
git add internal/config go.mod go.sum
git add -u internal/secretmanager test
git commit -m "chore: remove unused secret manager providers"
```

### Task 6: Remove ignored typed config knobs with overflow coverage

**Files:**
- Modify: `internal/config/config.go`
- Modify/create: the existing config test file selected by repository conventions
- Test: config load, unknown-key preservation, and active selector behavior

**Interfaces:**
- Consumes: YAML loading and the existing `Overflow` maps.
- Produces: the same active runtime configuration with fewer inert typed fields.

- [ ] **Step 1: Write regression coverage before field removal**

Add tests that load a config containing:

```yaml
unknown_future_key: preserved
```

and assert that the key remains in the relevant `Overflow` map, while an
active selector already used by production still loads into its typed field.

- [ ] **Step 2: Run the new tests**

```bash
rtk go test ./internal/config/... -run 'Test.*(Overflow|Load)' -count=1
```

The tests must pass against the current implementation before removing fields.

- [ ] **Step 3: Remove only fields with no non-test selector**

Use `rg` and `go list` to remove the verified inert fields listed in the
approved design, including dead callback/cache/router/alerting/UI/RBAC
compatibility knobs. Retain fields with live selectors such as active
callback, cache, OAuth/SSO, audit, spend-log, and wired router settings.

- [ ] **Step 4: Verify config compatibility**

```bash
rtk go test ./internal/config/... ./internal/proxy/... ./cmd/tianji/... -count=1
rtk go vet ./internal/config/...
rtk git diff --check
```

- [ ] **Step 5: Refresh CCC and commit**

```bash
rtk ccc index
git add internal/config
git commit -m "chore: remove unused config knobs"
```

### Task 7: Full audit and completion verification

**Files:**
- Modify: none unless the audit finds a remaining evidence-backed issue.
- Test: repository-wide test, lint, build, source scans, and Ponytail audit.

- [ ] **Step 1: Re-run source/dependency scans**

```bash
rtk go list -deps ./cmd/tianji
rtk rg -n 'internal/(prompt|rag|scim|secretmanager)|PromptManagementConfig|NewSCIMServer|LoadWithSecrets' internal cmd test --glob '*.go'
```

Expected: no dormant production references.

- [ ] **Step 2: Run the full verification suite**

```bash
rtk go test ./... -count=1
rtk golangci-lint run
rtk make build
```

If `make build` is blocked by the missing Tailwind binary, run the repository's
documented tool bootstrap, rerun the build, and report any remaining
environment-only blocker separately.

- [ ] **Step 3: Refresh indexes and run the audit**

```bash
rtk ccc index
```

Run the same whole-repository Ponytail audit procedure used for the baseline.
For every new finding, prove whether it is active; fix only evidence-backed
over-engineering and repeat the relevant task validation.

- [ ] **Step 4: Final diff and status review**

```bash
rtk git diff --check
git status --short --branch
git log --oneline origin/main..HEAD
```

Only after fresh output proves all gates pass and the audit returns
`Lean already. Ship.` may the branch enter the finishing workflow.
