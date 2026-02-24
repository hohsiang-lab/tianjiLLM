# Quickstart: Implementing Models Multi-Select

**Branch**: `001-models-multiselect` | **Date**: 2026-02-24

This guide walks an engineer through implementing this feature end-to-end. Follow the steps in order.

## Prerequisites

```bash
cd /Users/n0rmanc/src/hohsiang-lab/tianjiLLM
git checkout 001-models-multiselect
make build          # Verify baseline builds
make test           # Verify baseline tests pass
```

## Step 1: Add `AvailableModels` to View Models

### `internal/ui/pages/keys.templ`

In `KeysPageData`, add:
```go
AvailableModels []string  // populated from DB + config; used in createKeyForm
```

### `internal/ui/pages/key_detail.templ`

In `KeyDetailData`, add:
```go
AvailableModels []string  // populated from DB + config; used in EditSettingsForm
```

**Verify**: `make ui` (templ generate) should pass with no errors after this change.

## Step 2: Add Helper Functions

### `internal/ui/handler_keys.go`

Add the `sort` import, then add:

```go
// collectModelNames returns a sorted, deduplicated list of all configured model names.
func (h *UIHandler) collectModelNames(ctx context.Context) []string {
    seen := map[string]bool{}
    var names []string
    if h.DB != nil {
        rows, _ := h.DB.ListProxyModels(ctx)
        for _, r := range rows {
            if !seen[r.ModelName] {
                seen[r.ModelName] = true
                names = append(names, r.ModelName)
            }
        }
    }
    for _, m := range h.Config.ModelList {
        if !seen[m.ModelName] {
            seen[m.ModelName] = true
            names = append(names, m.ModelName)
        }
    }
    sort.Strings(names)
    return names
}

// parseModelSelection converts multi-select form values to a model list.
// Empty or containing "" → []string{} (All Models / unrestricted).
func parseModelSelection(values []string) []string {
    var result []string
    for _, v := range values {
        v = strings.TrimSpace(v)
        if v == "" {
            return []string{}
        }
        result = append(result, v)
    }
    return result
}
```

## Step 3: Populate `AvailableModels` in Handlers

### `loadKeysPageData` (handler_keys.go)

At the end of the function, before `return data`, add:
```go
data.AvailableModels = h.collectModelNames(r.Context())
```

### `handleKeyDetail` and `handleKeyEdit` (handler_keys.go)

After building `data := buildKeyDetailData(vt)`, add:
```go
data.AvailableModels = h.collectModelNames(r.Context())
```

## Step 4: Fix Form Submission Parsing

### `handleKeyCreate` (handler_keys.go)

Replace:
```go
modelsStr := r.FormValue("models")
// ...
models := parseCSV(modelsStr)
```

With:
```go
models := parseModelSelection(r.Form["models"])
```

Note: `r.ParseForm()` is already called earlier in this handler — `r.Form["models"]` is available.

### `handleKeyUpdate` (handler_keys.go)

Replace:
```go
if modelsStr != "" {
    params.Models = parseCSV(modelsStr)
}
```

With:
```go
// Always apply the multi-select result (empty slice = All Models)
params.Models = parseModelSelection(r.Form["models"])
```

Note: For the edit form, we always set models (even to empty), so the key can be changed from restricted back to unrestricted. Remove the `if modelsStr != ""` guard.

## Step 5: Replace UI Inputs with Multi-Select

### `internal/ui/pages/keys.templ` — `createKeyForm`

**Add helper function** (outside the templ block):
```go
func isModelSelected(model string, selected []string) bool {
    for _, s := range selected {
        if s == model {
            return true
        }
    }
    return false
}
```

**Replace** the existing Models text input:
```templ
<div class="space-y-2">
    <label for="models" class="text-sm font-medium">Models (comma-separated)</label>
    @input.Input(input.Props{
        ID: "models", Name: "models",
        Placeholder: "gpt-4, claude-sonnet-4-5-20250929",
    })
</div>
```

**With** a multi-select:
```templ
<div class="space-y-2">
    <label for="models" class="text-sm font-medium">Models</label>
    <select
        id="models"
        name="models"
        multiple
        size={ fmt.Sprintf("%d", min(6, len(data.AvailableModels)+1)) }
        class="flex w-full rounded-md border border-input bg-background px-3 py-2 text-sm min-h-[80px]"
    >
        <option value="" selected>All Models</option>
        for _, m := range data.AvailableModels {
            <option value={ m }>{ m }</option>
        }
    </select>
    <p class="text-xs text-muted-foreground">
        Hold Ctrl/Cmd to select multiple. Select "All Models" for unrestricted access.
    </p>
</div>
```

### `internal/ui/pages/key_detail.templ` — `EditSettingsForm`

**Replace** the existing Models text input (around line 440):
```templ
<div class="space-y-2">
    <label for="edit_models" class="text-sm font-medium">Models (comma-separated)</label>
    @input.Input(input.Props{
        ID: "edit_models", Name: "models",
        Attributes: templ.Attributes{
            "value": joinModels(data.Models),
        },
    })
</div>
```

**With**:
```templ
<div class="space-y-2">
    <label for="edit_models" class="text-sm font-medium">Models</label>
    <select
        id="edit_models"
        name="models"
        multiple
        size={ fmt.Sprintf("%d", min(6, len(data.AvailableModels)+1)) }
        class="flex w-full rounded-md border border-input bg-background px-3 py-2 text-sm min-h-[80px]"
    >
        <option value=""
            if len(data.Models) == 0 { selected }
        >All Models</option>
        for _, m := range data.AvailableModels {
            <option value={ m }
                if isModelSelected(m, data.Models) { selected }
            >{ m }</option>
        }
    </select>
    <p class="text-xs text-muted-foreground">
        Hold Ctrl/Cmd to select multiple. Select "All Models" for unrestricted access.
    </p>
</div>
```

Add `isModelSelected` helper to `key_detail.templ` as well (or move to a shared package).

## Step 6: Regenerate and Build

```bash
make ui    # templ generate + tailwind build
make build # full build
```

## Step 7: Tests

### Add unit tests to `test/contract/ui_test.go`

```go
func TestUI_KeyCreate_MultiSelectModels(t *testing.T) {
    // Test: two models selected → key has those models
}

func TestUI_KeyCreate_AllModels(t *testing.T) {
    // Test: empty value submitted → key is unrestricted (empty models)
}

func TestUI_KeyCreate_NoModelsField(t *testing.T) {
    // Test: no models field at all → key is unrestricted
}
```

### Run tests

```bash
make test
# or targeted:
go test ./internal/ui/... -v
go test ./test/contract/... -run TestUI_Key -v
```

## Step 8: E2E Test (optional, requires PostgreSQL)

```bash
make e2e  # Playwright tests
```

Verify the create key dialog shows the model multi-select with options populated.

## Edge Cases to Verify

- [ ] No models in DB or config → only "All Models" shown, form works
- [ ] 50+ models → `<select>` scrolls correctly, all selections preserved on submit
- [ ] Re-opening edit form → pre-selected models shown correctly
- [ ] Changing from specific models → All Models (edit) → DB updated to `[]`
- [ ] `make build` succeeds (no unused imports — `joinModels` may become unused if removed from key_detail.templ)
