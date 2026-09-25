# Contract: Org detail OpenAI credential panel

## Route

`GET /ui/orgs/{org_id}`

Protected by existing `sessionAuth` group in `internal/ui/routes.go`.

## Input

- `org_id`: route param for `OrganizationTable.organization_id`

## Data Reads

Required reads:

- `GetOrganization(ctx, orgID)`
- `ListOrgMembers(ctx, orgID)`
- `ListTeamsByOrganization(ctx, &orgID)`
- `ListCredentialsByOrg(ctx, &orgID)`
- `RateLimitStore.GetOpenAIQuotaState(credentialID, now)` when available

## Filtering

Panel rows must satisfy:

```text
credential.organization_id == orgID
credential.credential_type == "openai_subscription"
```

## Response / Rendered UI

Org detail page includes:

- Org header and existing overview content。
- `OpenAI credentials` card。
- Empty state when no rows exist。
- Table rows when rows exist。
- Detail links to `/ui/credentials/{credential_id}`。

## Forbidden Output

The HTML response must not contain:

- `credential_value`
- raw `access_token`
- raw `refresh_token`
- raw `id_token`
- JWT-like seeded fixture text
- `Bearer `
- encrypted credential blob
- raw fallback API key
- arbitrary raw OpenAI account payload

## Error Behavior

- Missing org keeps existing redirect/safe behavior。
- Credential load failure must not leak DB/internal secret details into page。
- Missing quota state renders unknown/not recorded。
