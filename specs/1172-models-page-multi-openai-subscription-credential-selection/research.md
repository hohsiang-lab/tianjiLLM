# Research: Models page multi OpenAI subscription credential selection

## Repo Reality

### Models page current behavior

- `internal/ui/handler_models.go` loads Models page data from DB and YAML config fallback。
- Create Model builds `tianji_params` from `model`, optional `api_base`, optional `api_key`, `tpm`, `rpm`。
- Edit Model parses existing `tianji_params` into a map, updates known fields, and preserves unknown fields。
- `internal/ui/pages/models.templ` renders Create/Edit dialogs with `api_base` and `api_key` inputs but no subscription credential selector。
- Existing E2E in `test/e2e/models_create_test.go` and `models_edit_test.go` covers create/edit, API-key preservation, custom `api_base`, and unknown-field preservation。

### Credential data and safe metadata

- `CredentialTable` has `credential_id`, `credential_name`, `credential_type`, `credential_value`, `credential_info`, optional `organization_id`。
- `internal/ui/handler_credentials.go` filters OpenAI subscription rows with `credential_type == "openai_subscription"`。
- That handler parses a narrow metadata struct (`email`, `status`, `last_refresh_at`, `last_error`, `disabled_reason`) and redacts strings before rendering。
- This issue should reuse that safe-display pattern instead of rendering arbitrary `credential_info` JSON。

### Backend config contract

- `config.TianjiParams` already contains `OpenAISubscriptionCredentialIDs []string` with YAML key `openai_subscription_credential_ids`。
- Config validation rejects subscription credential IDs with custom `api_base` and non-OpenAI/default-base providers。
- UI should block the same invalid combination earlier, but backend validation remains authoritative。

## Decisions

### Decision 1 - Use explicit repeated checkbox values

Use repeated form fields:

```text
openai_subscription_credential_ids=cred-a&openai_subscription_credential_ids=cred-b
```

**Rationale**: This matches existing Go/HTMX form submission style and avoids JSON parsing in HTML forms. It preserves order and is easy to test with `r.Form["openai_subscription_credential_ids"]`。

**Rejected**: comma-separated input, because the Linear goal is explicit UI selection and typos must be avoided。

### Decision 2 - Load options from `CredentialTable`, filtered in UI adapter

Use existing `ListCredentials` initially and filter `credential_type == "openai_subscription"` in the adapter. Add a sqlc query only if implementation reveals performance or clarity requires it。

**Rationale**: This follows existing HO-1186 implementation and avoids unnecessary DB surface during a narrow FE change。

### Decision 3 - Preserve missing configured IDs visibly

If a model already references a deleted or wrong-type credential ID, the edit dialog should show a missing badge instead of silently dropping the ID on read。

**Rationale**: Silent read-time mutation would hide broken config and could change runtime routing unintentionally。

### Decision 4 - Server-side conflict validation is mandatory

Client-side disable/help text can improve UX, but `handleModelCreate` and `handleModelUpdate` must reject selected IDs plus custom `api_base`。

**Rationale**: HTMX/client JS can be bypassed; backend config validation already makes this combination invalid。

### Decision 5 - No real OpenAI calls

The UI does not test or refresh credentials. It only displays safe metadata from DB and saves selected IDs。

**Rationale**: Lifecycle and upstream validation belong to HO-1185 and backend credential resolution tickets。

## External / Library Evidence

- Context7 `/a-h/templ`: dynamic attributes and spread `templ.Attributes` maps support the exact existing pattern used by this repo for HTMX attributes, ARIA labels, and IDs。
- Context7 `/bigskysoftware/htmx`: `hx-post` form submission sends form data and swaps the response into an `hx-target`, matching current Models create/update table-swap behavior。
- grep.app: public examples for `templ.Attributes` confirm common Go `templ` projects pass dynamic input/button attributes through `templ.Attributes`; exact Tianji-like HTMX prior art was not useful, so repo-local patterns are stronger evidence。

## Open Questions

None for Todo planning. Implementation must still verify selector accessibility and generated templ compile behavior.
