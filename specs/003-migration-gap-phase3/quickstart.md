# Quickstart: Phase 3 — Enterprise Features & Full Parity

## Prerequisites

- tianjiLLM running with Phase 1 + Phase 2 features
- PostgreSQL with existing schema
- Go 1.22+

## Feature Highlights

### 1. Policy Engine — Conditional Guardrail Assignment

```yaml
# proxy_config.yaml
policy_config:
  policies:
    - name: "finance-strict"
      conditions:
        model: "gpt-4.*"
      guardrails_add: ["pii-detection", "content-filter", "audit-log"]
      pipeline:
        mode: "pre_call"
        steps:
          - guardrail: "pii-detection"
            on_pass: "next"
            on_fail: "block"
            pass_data: false
          - guardrail: "content-filter"
            on_pass: "next"
            on_fail: "block"
            pass_data: false
          - guardrail: "audit-log"
            on_pass: "allow"
            on_fail: "next"
            pass_data: false

    - name: "finance-relaxed"
      inherit: "finance-strict"
      guardrails_remove: ["content-filter"]

  attachments:
    - policy: "finance-strict"
      teams: ["team-finance"]
```

### 2. SCIM 2.0 — Enterprise IDP Provisioning

```yaml
# proxy_config.yaml
general_settings:
  scim_enabled: true
  scim_upsert_user: true  # auto-create users referenced in groups
```

```bash
# IDP provisions a user
curl -X POST http://proxy:4000/scim/v2/Users \
  -H "Authorization: Bearer $MASTER_KEY" \
  -H "Content-Type: application/scim+json" \
  -d '{
    "schemas": ["urn:ietf:params:scim:schemas:core:2.0:User"],
    "userName": "john@acme.com",
    "name": {"givenName": "John", "familyName": "Doe"},
    "active": true
  }'

# IDP provisions a group (team)
curl -X POST http://proxy:4000/scim/v2/Groups \
  -H "Authorization: Bearer $MASTER_KEY" \
  -H "Content-Type: application/scim+json" \
  -d '{
    "schemas": ["urn:ietf:params:scim:schemas:core:2.0:Group"],
    "displayName": "Engineering",
    "members": [{"value": "<user-id>"}]
  }'
```

### 3. Assistants API Pass-through (Authenticated with Logging)

```yaml
# proxy_config.yaml
assistant_settings:
  custom_llm_provider: "openai"  # or "azure"
  tianji_params:
    api_key: "$OPENAI_API_KEY"
```

```bash
# Create an assistant through the proxy
curl -X POST http://proxy:4000/v1/assistants \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "model": "gpt-4o",
    "name": "Code Helper",
    "instructions": "You are a helpful coding assistant."
  }'
```

### 4. Background Scheduler

Automatic — starts with the proxy. Configurable intervals:

```yaml
# proxy_config.yaml
general_settings:
  budget_reset_interval: "24h"          # daily budget reset check
  spend_log_retention_days: 90          # delete logs older than 90 days
  health_check_interval: "60s"          # probe deployments
  deployment_reload_interval: "30s"     # hot-reload from DB
  credential_refresh_interval: "5m"     # refresh credentials
```

### 5. New Callbacks

```yaml
# proxy_config.yaml
tianji_settings:
  callbacks:
    - "lunary"
    - "posthog"
    - "datadog_llm"
    - "gcs_pubsub"
    - "openmeter"
    - "lago"

  callback_configs:
    - type: "posthog"
      api_key: "$POSTHOG_API_KEY"
      base_url: "https://us.i.posthog.com"
    - type: "lago"
      api_key: "$LAGO_API_KEY"
    - type: "gcs_pubsub"
      project: "my-project"
      topic: "llm-events"
```

### 6. New Providers

```yaml
# proxy_config.yaml
model_list:
  - model_name: "qwen-turbo"
    tianji_params:
      model: "dashscope/qwen-turbo"
      api_key: "$DASHSCOPE_API_KEY"

  - model_name: "doubao-pro"
    tianji_params:
      model: "volcengine/doubao-pro-4k"
      api_key: "$VOLCENGINE_API_KEY"

  - model_name: "copilot"
    tianji_params:
      model: "github_copilot/gpt-4o"
      api_key: "$GITHUB_TOKEN"
```

### 7. Management API Examples

```bash
# Add a model via API (no restart needed)
curl -X POST http://proxy:4000/model/new \
  -H "Authorization: Bearer $MASTER_KEY" \
  -d '{"model_name": "gpt-4o-new", "tianji_params": {"model": "openai/gpt-4o", "api_key": "sk-..."}}'

# Create a tag for cost tracking
curl -X POST http://proxy:4000/tag/new \
  -H "Authorization: Bearer $MASTER_KEY" \
  -d '{"name": "project-alpha"}'

# Query global spend by team
curl http://proxy:4000/global/spend/teams \
  -H "Authorization: Bearer $MASTER_KEY"
```

### 8. CyberArk Conjur Secret Manager

```yaml
# proxy_config.yaml
secret_manager:
  type: conjur
  conjur_account: myorg
  conjur_url: https://conjur.example.com
  conjur_login: host/tianji-proxy
  conjur_api_key: $CONJUR_API_KEY

model_list:
  - model_name: "gpt-4o"
    tianji_params:
      model: "openai/gpt-4o"
      api_key: "conjur://prod/openai/api_key"  # resolved from Conjur
```

## Verification

```bash
# Run all tests
make test

# Run specific work stream tests
go test ./internal/policy/... -v
go test ./internal/scim/... -v
go test ./internal/scheduler/... -v

# Full integration test
make check
```
