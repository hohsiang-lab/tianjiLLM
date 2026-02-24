# Contracts: Models Multi-Select for Create API Key

**Branch**: `001-models-multiselect` | **Date**: 2026-02-24

## No New API Endpoints

This feature is a **pure UI-layer change**. No new REST or HTMX endpoints are introduced.

## Existing Endpoints (Modified Behavior)

The following existing endpoints receive the multi-select form data. Their URL, method, and response format are **unchanged**; only the handling of the `models` field changes.

---

### `POST /ui/keys/create`

**Current behavior**: `models` form field is a single comma-separated string (e.g. `"gpt-4, claude-3"`), parsed by `parseCSV()`.

**New behavior**: `models` form field is a multi-select — zero or more repeated fields with the same name. Go receives `r.Form["models"]` as `[]string`.

| Scenario | Form Submission | Stored in DB |
|----------|----------------|-------------|
| All Models (unrestricted) | `models=` (empty string) or no `models` field | `models = []` |
| One model selected | `models=gpt-4` | `models = ["gpt-4"]` |
| Two models selected | `models=gpt-4&models=claude-3` | `models = ["gpt-4", "claude-3"]` |

**Response**: unchanged — `KeysTableWithKeyReveal` HTML partial (on success) or `KeysTableWithToast` with error.

---

### `POST /ui/keys/{token}/settings`

**Current behavior**: `models` form field is a single comma-separated string, parsed by `parseCSV()`.

**New behavior**: `models` form field is a multi-select — zero or more repeated fields.

| Scenario | Form Submission | Stored in DB |
|----------|----------------|-------------|
| All Models | `models=` or no `models` field | `models = []` |
| Specific models | `models=gpt-4&models=claude-3` | `models = ["gpt-4", "claude-3"]` |

**Response**: unchanged — `SettingsTabWithToast` HTML partial.

---

## HTMX Endpoints (Unchanged)

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `GET /ui/keys` | GET | Render Keys page (loads `AvailableModels` for create form) |
| `GET /ui/keys/table` | GET | Render table partial (loads `AvailableModels` for create form) |
| `GET /ui/keys/{token}` | GET | Render Key Detail page (loads `AvailableModels` for edit form) |
| `GET /ui/keys/{token}/edit` | GET | Render edit settings form (loads `AvailableModels`) |
