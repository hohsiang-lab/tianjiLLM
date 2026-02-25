# Research: Models Multi-Select for Create API Key

**Feature**: `001-models-multiselect`  
**Date**: 2026-02-24  
**Status**: Complete — all unknowns resolved

---

## Decision Log

### 1. Multi-Select UI Component Strategy

**Decision**: Implement a custom inline checkbox-inside-popover component in `keys.templ` (and `key_detail.templ`) rather than adding a new shared UI component.

**Rationale**:
- The multi-select is used in exactly two places (Create Key form, Edit Key settings form). Premature extraction to a shared component adds boilerplate.
- The existing codebase pattern puts form-specific template helpers as `templ` helper functions within the page file (e.g., `createKeyForm`, `keyRow` in `keys.templ`).
- The existing `popover` + checkbox pattern is achievable with the already-imported templUI components and plain `<input type="checkbox">` elements.
- A native `<select multiple>` is technically simpler but requires Ctrl/Cmd+click — not discoverable or usable per SC-002 (under 30 seconds, up to 50 models).

**Alternatives considered**:
- **Native `<select multiple>`**: Rejected — non-obvious multi-selection UX (requires Ctrl+click); fails usability bar for admins unfamiliar with HTML multi-selects.
- **Extract to `internal/ui/components/multiselect/`**: Rejected for now — two callsites do not justify an abstraction. Can be refactored in a future iteration.
- **Third-party JS widget (e.g., Choices.js, Tom Select)**: Rejected — introduces external JS dependency; the project already avoids external JS beyond what is vendored in `assets/js/`.

**Source**: Codebase review of `internal/ui/components/`, `internal/ui/pages/keys.templ`, `internal/ui/assets/js/`.

---

### 2. Model Name Source for the Selector

**Decision**: Aggregate model names from **both** the DB (`ProxyModelTable`) and the YAML config (`Config.ModelList`) — the same logic that `loadModelsPageData` already uses — and deduplicate by model name.

**Rationale**:
- The proxy considers both DB-managed models and YAML-config models as valid model names for key restrictions (matching how `handler_models.go` exposes them in the Models management UI).
- Using only DB models would miss YAML-only deployments; using only config would miss DB-added models.
- No DB schema change required — `ListProxyModels` already exists.
- This keeps the model list consistent with what the Models page shows.

**Alternatives considered**:
- **DB-only**: Rejected — breaks deployments where models are purely YAML-configured.
- **New dedicated endpoint `/ui/api/models`**: Rejected — not needed since model names are loaded server-side at page render time (the spec says no dynamic refresh after form open is needed).

**Source**: `internal/ui/handler_models.go` (`loadModelsPageData`), `internal/db/queries/proxy_model.sql` (`ListProxyModels`), `internal/config/config.go` (`ProxyConfig.ModelList`).

---

### 3. Form Submission Encoding

**Decision**: Use repeated form values (`<input type="checkbox" name="models" value="...">`) plus a sentinel `<input type="hidden" name="all_models" value="0|1">`.

**Rationale**:
- HTML checkbox groups with `name="models"` naturally submit multiple `models=X` entries in the POST body.
- Go's `net/http.Request.Form["models"]` returns `[]string` of all submitted values — directly usable without further parsing.
- A sentinel `all_models=1` hidden field makes "All Models" selection unambiguous on the server side without relying on an empty slice (which could also mean "no checkbox checked by accident").
- JS toggles: when "All Models" is checked → set `all_models=1`, disable individual checkboxes; when unchecked → set `all_models=0`, re-enable individual checkboxes.

**Alternatives considered**:
- **CSV text field preserved**: Rejected — FR-009 explicitly requires the free-text field to be removed.
- **JSON array in single hidden field**: Rejected — adds unnecessary JS serialization complexity; repeated-name form values are idiomatic HTML.

**Source**: HTML spec, Go `net/http` docs, existing `parseCSV` usage in `handler_keys.go`.

---

### 4. Scope — Create Form Only vs. Edit Form Too

**Decision**: Update **both** the Create Key form (`createKeyForm` in `keys.templ`) and the Edit Settings form (`EditSettingsForm`/`SettingsTab` in `key_detail.templ`).

**Rationale**:
- The spec (FR-009) says the free-text input MUST be removed. The edit form also exposes a `models` text input via `modelsStr := r.FormValue("models")`.
- Consistency: having the create form use multi-select while the edit form uses free text is confusing and a regression risk (SC-005).
- `KeyDetailData` will receive the same `AvailableModels []string` field.

**Source**: `internal/ui/handler_keys.go` (`handleKeyUpdate`), `internal/ui/pages/key_detail.templ`.

---

### 5. Backend `handleKeyCreate` Change

**Decision**: Replace `parseCSV(r.FormValue("models"))` with direct `r.Form["models"]` slice (nil-safe), gated on `all_models` sentinel.

**Rationale**:
- After `r.ParseForm()`, `r.Form["models"]` returns all submitted `models=X` values as `[]string`.
- If `all_models=1` → send `nil`/empty slice (unrestricted key, per spec FR-003 / Assumption "All Models = empty array").
- If `all_models=0` and `r.Form["models"]` is non-empty → use that slice directly (no CSV splitting needed).
- If `all_models=0` and `r.Form["models"]` is empty (no checkboxes ticked) → treat as unrestricted (FR-008: default is unrestricted).

**Source**: Go `net/http` docs; `internal/ui/handler_keys.go`.

---

## Resolved Unknowns

| Unknown | Resolution |
|---------|------------|
| How does the proxy know which models are valid? | `ProxyModelTable` (DB) + `ModelList` (YAML config) — both already loaded in `UIHandler`. |
| Is a JS dependency needed? | No — small inline `<script>` block in the templ file for "All Models" toggle is sufficient. |
| Does the backend API need changing? | No — `CreateVerificationTokenParams.Models []string` already accepts a slice; only form parsing logic changes. |
| DB schema change needed? | No — `models` column is already `text[]`. |
| How to handle empty model list (FR-007)? | Show only "All Models" option; form still works; submit with `all_models=1`. |
