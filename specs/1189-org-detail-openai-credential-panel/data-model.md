# Data Model: Org detail OpenAI credential panel

No schema migration is planned.

## Existing Tables

### `OrganizationTable`

Used by existing Org detail page.

Relevant fields:

- `organization_id`
- `organization_alias`
- `spend`
- `max_budget`
- `models`
- `tpm_limit`
- `rpm_limit`
- `metadata`
- `created_at`

### `CredentialTable`

Existing credential persistence.

Relevant safe fields:

- `credential_id`
- `credential_name`
- `credential_type`
- `credential_info`
- `organization_id`
- `created_at`
- `updated_at`

Explicitly forbidden display field:

- `credential_value`

## Existing Query

```sql
-- name: ListCredentialsByOrg :many
SELECT *
FROM "CredentialTable"
WHERE organization_id = $1
ORDER BY created_at DESC;
```

Implementation must pass the current org ID as `&orgID` and then keep only rows with:

```text
credential_type == "openai_subscription"
```

## View Models

### `OrgDetailData`

Add one field:

```go
OpenAICredentials []CredentialRow
```

Using `pages.CredentialRow` is preferred because it already represents safe list-level credential data. If implementation extracts a narrower row type, it must preserve the same safety and formatting semantics.

### `CredentialRow`

Already exists in `internal/ui/pages/credentials.templ`.

Fields relevant to org panel:

- `ID`
- `Name`
- `Email`
- `CredentialStatus`
- `CredentialVariant`
- `QuotaStatus`
- `QuotaStatusVariant`
- `QuotaSummary`
- `LastRefresh`
- `LastError`

`OrganizationID` may be retained but does not need prominent display inside an org-scoped panel.

## Relationships

```text
OrganizationTable.organization_id
  -> CredentialTable.organization_id
       filtered by credential_type = "openai_subscription"
```

Credentials with `organization_id = NULL` or a different org ID are not part of this panel.

## Validation Rules

- Current org ID comes from route param and must match loaded organization。
- Credential row must be org-scoped by DB query and OpenAI subscription type by filter。
- Unknown safe metadata renders fallback text。
- Secret material is excluded before template rendering。
