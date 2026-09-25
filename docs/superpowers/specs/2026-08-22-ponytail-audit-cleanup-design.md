# Ponytail Audit Cleanup Design

**Date:** 2026-08-22

**Status:** Approved in chat as option A

## Goal

Remove the evidence-backed over-engineering findings from tianjiLLM without
changing active request routing, UI behavior, or supported runtime semantics,
then rerun the repository audit until it reports no findings.

## Scope

The cleanup covers only code that has no production importer, selector, route,
or generated-output consumer:

1. Unused templUI component source/generated files and unused component assets.
2. Unreferenced Lucide alias variables while preserving dynamic icon lookup and
   every alias used by active templates.
3. Dormant `internal/prompt` and `internal/rag` skeleton packages. Existing
   RAG passthrough handlers remain.
4. Dormant `internal/scim`, its uninitialized proxy hook, tests, and dependency.
5. Dormant `internal/secretmanager`, its provider-only dependencies, and any
   generic resolver seam proven to have no caller.
6. Typed config fields with no non-test runtime selector.

The cleanup does not remove active database prompt-management handlers,
provider registry packages, dynamic icon data, RAG passthrough endpoints, or
configuration values that are read by production code.

## Compatibility policy

`internal/config` keeps its existing `Overflow` maps. Removing an unused typed
field therefore does not reject or discard an unknown YAML key; it remains
available through the existing overflow path. Active typed fields and
`config.Load()` behavior remain unchanged.

All removed packages are under `internal/`, so this is an internal source
cleanup rather than a public Go API change. Removing SCIM and secret-manager
dependencies is safe only after a repository-wide caller check confirms that
the production binary does not initialize either path.

## Implementation slices

Each slice is independently formatted, tested, and reviewed:

1. Generated UI component directories/assets.
2. Lucide aliases.
3. Prompt and RAG skeletons.
4. SCIM package and proxy hook.
5. Secret manager and provider dependencies.
6. Unused typed config fields plus unknown-key regression coverage.

Use `go list -deps ./cmd/tianji`, `rg`, and generated-code checks to prove that
the deleted symbols are not active. Refresh CCC after each slice; keep
Graphify output scoped to the changed source and out of tracked changes.

## Validation

Per slice:

- focused `go test` for affected packages;
- `gofmt` and `git diff --check`;
- repository-wide reference checks;
- `ccc index`.

Final gates:

- `rtk go test ./... -count=1`;
- `rtk golangci-lint run`;
- `make build` after providing the missing `bin/tailwindcss`;
- scoped Graphify refresh;
- the same Ponytail audit query repeated until it returns `Lean already. Ship.`.

No deployment or production behavior change is part of this cleanup. Any
runtime regression found by the gates stops the cleanup and preserves the
branch for diagnosis.
