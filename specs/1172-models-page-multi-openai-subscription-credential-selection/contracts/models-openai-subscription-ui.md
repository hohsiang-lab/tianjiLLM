# Contract: Models OpenAI subscription credential UI

## Routes

No new route is required.

Existing routes with changed behavior:

| Route | Method | Behavior change |
| --- | --- | --- |
| `/ui/models` | GET | Loads OpenAI subscription credential options for Create form and table summaries。 |
| `/ui/models/table` | GET | Renders selected credential summaries in table rows。 |
| `/ui/models/create` | POST | Accepts repeated `openai_subscription_credential_ids` fields。 |
| `/ui/models/edit?model_id=...` | GET | Renders Edit form with selected credential IDs prefilled。 |
| `/ui/models/update` | POST | Accepts repeated `openai_subscription_credential_ids` fields and updates JSON config。 |

## Form Fields

### Create / Update

```text
model_name=e2e-openai
model=openai/gpt-4o
api_base=
api_key=
openai_subscription_credential_ids=cred-primary
openai_subscription_credential_ids=cred-backup
tpm=50000
rpm=500
```

Expected `tianji_params`:

```json
{
  "model": "openai/gpt-4o",
  "openai_subscription_credential_ids": ["cred-primary", "cred-backup"],
  "tpm": 50000,
  "rpm": 500
}
```

### Empty Selection

```text
model_name=e2e-api-key
model=openai/gpt-4o
api_key=$OPENAI_API_KEY
```

Expected `tianji_params`:

```json
{
  "model": "openai/gpt-4o",
  "api_key": "$OPENAI_API_KEY"
}
```

`openai_subscription_credential_ids` must be omitted or removed。

### Invalid Combination

```text
model_name=e2e-invalid
model=openai/gpt-4o
api_base=https://custom.api.com
openai_subscription_credential_ids=cred-primary
```

Expected behavior:

- No create/update DB mutation。
- Dialog remains open。
- Toast or field-level error mentions that OpenAI subscription credentials cannot be combined with custom `api_base`。

## Safe Display Contract

Allowed display fields:

- `credential_name`
- `credential_id`
- safe/redacted `credential_info.email`
- safe/redacted `credential_info.status`

Forbidden display fields:

- `credential_value`
- raw `access_token`
- raw `refresh_token`
- raw `id_token`
- JWT-looking strings
- bearer strings
- encrypted credential blob
- arbitrary raw `credential_info` map values

## Compatibility Contract

Existing behavior must remain unchanged when `openai_subscription_credential_ids` is empty:

- `api_key` persists on create。
- Empty edit `api_key` input preserves existing API key。
- `api_base` persists on create/update。
- Unknown `tianji_params` fields survive update。
- Models search/pagination/table refresh continue to work。
