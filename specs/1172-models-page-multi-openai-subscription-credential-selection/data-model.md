# Data Model: Models page OpenAI subscription credential selection

## Existing DB Tables

### `ProxyModelTable`

No schema change.

Relevant field:

| Field | Type | Semantics |
| --- | --- | --- |
| `tianji_params` | JSONB / `[]byte` in sqlc row | Stores model provider config, including `openai_subscription_credential_ids`。 |

### `CredentialTable`

No schema change.

Relevant fields:

| Field | Type | UI use |
| --- | --- | --- |
| `credential_id` | text | Submitted value and saved ID。 |
| `credential_name` | text | Safe display label。 |
| `credential_type` | text | Must equal `openai_subscription` to be selectable。 |
| `credential_value` | text | Never displayed or passed to templates。 |
| `credential_info` | JSONB | Parsed through safe metadata struct only。 |
| `organization_id` | nullable text | Optional context display if needed。 |

## `tianji_params` Shape

```json
{
  "model": "openai/gpt-4o",
  "openai_subscription_credential_ids": ["cred-primary", "cred-backup"],
  "tpm": 50000,
  "rpm": 500
}
```

Rules:

- Empty selection means the field is omitted or removed。
- Non-empty selection is invalid with non-empty `api_base`。
- Existing `api_key`, `api_base`, `api_version`, `tpm`, `rpm`, and unknown fields keep current semantics。

## View Models

### `ModelCredentialOption`

```go
type ModelCredentialOption struct {
    ID     string
    Name   string
    Email  string
    Status string
}
```

Source: filtered `CredentialTable` rows. The struct must not include `CredentialValue` or raw metadata maps。

### `ModelCredentialSummary`

```go
type ModelCredentialSummary struct {
    ID      string
    Name    string
    Email   string
    Status  string
    Missing bool
}
```

Source: selected IDs plus options lookup. Missing IDs remain visible until admin removes them。

### `ModelRow` additions

```go
OpenAISubscriptionCredentialIDs []string
OpenAISubscriptionCredentials   []ModelCredentialSummary
```

### `ModelsPageData` additions

```go
OpenAISubscriptionCredentialOptions []ModelCredentialOption
```

## Data Flow

### Create

```text
GET /ui/models
  -> load models + load openai_subscription credential options
  -> render Add Model selector

POST /ui/models/create
  -> r.ParseForm()
  -> ids := normalized repeated openai_subscription_credential_ids
  -> if len(ids)>0 && api_base!="": reject without closing dialog
  -> build tianji_params
  -> persist ProxyModelTable row
```

### Edit

```text
GET /ui/models/edit?model_id=...
  -> load model row
  -> parse existing tianji_params.openai_subscription_credential_ids
  -> load credential options
  -> render selector with checked IDs and missing badges

POST /ui/models/update
  -> parse existing tianji_params
  -> parse selected IDs
  -> validate api_base conflict
  -> set or delete openai_subscription_credential_ids
  -> preserve api_key when empty and preserve unknown fields
```

## Validation

| Rule | Behavior |
| --- | --- |
| selected IDs empty | Omit/delete `openai_subscription_credential_ids`; API-key path unchanged。 |
| selected IDs non-empty + `api_base` non-empty | Reject create/update, keep dialog open。 |
| duplicate IDs submitted | Deduplicate preserving first occurrence。 |
| blank IDs submitted | Drop blanks。 |
| submitted ID not in current options | Preserve only if already configured and shown as missing; new unknown IDs from form should be rejected or ignored with error。 |
| wrong credential type | Not shown in options and not accepted as a new selection。 |

## No Schema Migration

This issue only writes an existing JSON config field and reads existing credential rows. No DB migration is required.
