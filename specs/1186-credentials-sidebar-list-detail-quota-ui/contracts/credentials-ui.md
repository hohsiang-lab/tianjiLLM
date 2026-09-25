# Contract: Credentials UI

## GET `/ui/credentials`

Protected by existing UI `sessionAuth`.

### Purpose

Render the OpenAI subscription credentials list page.

### Data Inputs

- `CredentialTable` rows from existing DB query path.
- Only rows where `credential_type = "openai_subscription"` are included.
- Optional quota state from `UIHandler.RateLimitStore`.

### Rendered Output

The page includes:

- Sidebar `Credentials` active nav state.
- A list/table of OpenAI subscription credentials.
- Safe fields only:
  - credential ID as link target
  - credential name
  - account email if known
  - credential status
  - quota summary if known
  - reset time if known
  - last refresh time if known
  - safe last error summary
  - organization ID if present

### Required Empty State

When no OpenAI subscription credentials exist, render an empty state equivalent to existing list pages and do not return HTTP 500.

### Forbidden Output

The response body must not include:

- `credential_value`
- raw/encrypted token bundle
- access/refresh/id token
- JWT/bearer strings
- fallback API keys
- raw OpenAI account payload

## GET `/ui/credentials/{credential_id}`

Protected by existing UI `sessionAuth`.

### Purpose

Render one OpenAI subscription credential detail page.

### Data Inputs

- One `CredentialTable` row by `credential_id`.
- Narrow safe metadata decoded from `credential_info`.
- Optional `OpenAIQuotaState` from `RateLimitStore`.

### Success Output

The page includes:

- Header with credential name and credential health badge.
- Account card: account email, credential ID, organization ID, created/updated timestamps.
- Status card: status, last refresh, last error, disabled reason.
- Quota card: quota status, utilization summary, overall reset when known.
- Quota dimension table: requests and tokens with limit, remaining, utilization, reset.
- Back link to `/ui/credentials`.

### Missing or Wrong Type

If `credential_id` is missing, not found, or not `openai_subscription`, render a safe not-found/error page or existing safe HTTP error behavior. Do not reveal whether secret material exists.

### Unknown Quota

If no quota state exists, render an explicit "no quota data yet" / unknown state. Do not render `0% used` unless utilization is known.

### Forbidden Output

Same forbidden output as list route. The detail page must not include hidden inputs or data attributes carrying secret material.
