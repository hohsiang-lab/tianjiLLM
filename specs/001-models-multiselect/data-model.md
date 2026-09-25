# Data Model: Models Multi-Select for Create API Key

**Feature**: `001-models-multiselect`  
**Date**: 2026-02-24

---

## Entities

### VerificationToken (API Key) — No Schema Change

The `VerificationToken` DB table already stores `models text[]`. This feature
does not change the schema or the semantics of that field.

| Field | Type | Semantics |
|-------|------|-----------|
| `models` | `text[]` / `[]string` | Empty slice → unrestricted ("All Models"). Non-empty → allowed model names. |

**No migration required.**

---

## View-Model Changes (Go Structs)

### `pages.KeysPageData` — add `AvailableModels`

```go
type KeysPageData struct {
    // ... existing fields unchanged ...
    
    // NEW: all model names available in the proxy (DB + config).
    // Used to populate the Models multi-select in the Create Key form.
    AvailableModels []string
}
```

### `pages.KeyDetailData` — add `AvailableModels`

```go
type KeyDetailData struct {
    // ... existing fields unchanged ...
    
    // NEW: all model names available in the proxy.
    // Used to populate the Models multi-select in the Edit Settings form.
    AvailableModels []string
}
```

---

## Data Flow

### Create Key Form (POST `/ui/keys/create`)

```
Browser                          UIHandler.handleKeyCreate
───────                          ─────────────────────────
GET /ui/keys
  ← KeysPageData{AvailableModels: ["gpt-4", "claude-3", ...]}
  
  User opens "Create New Key" dialog
  User checks "gpt-4" and "claude-3" (or checks "All Models")
  
POST /ui/keys/create
  Form body: all_models=0&models=gpt-4&models=claude-3
  (or)       all_models=1
                                  r.ParseForm()
                                  allModels := r.FormValue("all_models")
                                  var models []string
                                  if allModels != "1" {
                                    models = r.Form["models"]
                                  }
                                  // models == ["gpt-4", "claude-3"] or []
                                  
                                  CreateVerificationTokenParams{
                                    Models: models, // unchanged semantics
                                    ...
                                  }
```

### Edit Key Settings (POST `/ui/keys/{token}/update`)

Same pattern: `all_models` sentinel + `r.Form["models"]` slice.

---

## Validation Rules

| Rule | Implementation |
|------|----------------|
| "All Models" takes precedence | If `all_models=1`, set `models = nil` (no individual selections recorded). |
| No model selected (unchecked) | Treated as "All Models" — `models` stays empty, no restriction. |
| Selected models are submitted verbatim | No server-side validation that model names exist; key stores whatever was submitted (matching Python behavior). |
| Model names are unique in selector | Deduplication by model name when building `AvailableModels`. |

---

## State Transitions

```
"All Models" selected
    → hidden field all_models=1
    → individual checkboxes disabled
    → server receives models=[] → unrestricted key

Specific models selected
    → hidden field all_models=0
    → checkboxes for gpt-4, claude-3 checked
    → server receives models=["gpt-4","claude-3"] → restricted key

No selection (empty list)
    → treated same as "All Models"
    → server receives models=[] → unrestricted key
```

---

## No New SQL Queries Required

The `ListProxyModels` sqlc query already exists and returns all `ProxyModelTable`
rows. `UIHandler.loadKeysPageData` will call `h.DB.ListProxyModels(ctx)` and
extract model names, then merge with `h.Config.ModelList` names (deduplicating).

No new `.sql` files or `make generate` run needed for this feature.
