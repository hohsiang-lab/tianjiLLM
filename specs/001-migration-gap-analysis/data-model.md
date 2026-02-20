# Data Model: TianjiLLM-Go Migration

**Date**: 2026-02-16
**Branch**: `001-migration-gap-analysis`

## Entity Relationship Overview

```
Organization 1──* Team 1──* User
     │                │         │
     │                │         │
     ▼                ▼         ▼
  Budget           Budget    Budget
                      │
                      ▼
                 VirtualKey ──* SpendLog
                      │
                      ├── models[] (allowed)
                      ├── guardrails[] (assigned)
                      └── callbacks[] (assigned)

ModelDeployment ──* DeploymentHealth
      │
      └── Provider + Credential

Policy ──* GuardrailBinding
  │
  └── RoutingRule[]
```

## Entities

### Organization (NEW — Phase 2)

| Field           | Type       | Notes                          |
| --------------- | ---------- | ------------------------------ |
| organization_id | string     | Primary key                    |
| name            | string     | Display name                   |
| max_budget      | decimal    | Spending limit                 |
| spend           | decimal    | Current spend                  |
| models          | []string   | Allowed model names            |
| metadata        | jsonb      | Arbitrary metadata             |
| created_at      | timestamp  |                                |
| updated_at      | timestamp  |                                |

### Team (EXISTS — needs update for org membership)

| Field           | Type       | Notes                          |
| --------------- | ---------- | ------------------------------ |
| team_id         | string     | Primary key                    |
| organization_id | string     | FK → Organization (NEW)        |
| team_alias      | string     | Display name                   |
| max_budget      | decimal    | Spending limit                 |
| spend           | decimal    | Current spend                  |
| models          | []string   | Allowed model names            |
| tpm_limit       | int64      | Tokens per minute limit        |
| rpm_limit       | int64      | Requests per minute limit      |
| budget_duration | string     | Reset period (e.g. "30d")      |
| budget_reset_at | timestamp  | Next reset time                |
| blocked         | bool       | Access blocked                 |
| metadata        | jsonb      |                                |

### User (EXISTS — needs update)

| Field           | Type       | Notes                          |
| --------------- | ---------- | ------------------------------ |
| user_id         | string     | Primary key                    |
| team_id         | string     | FK → Team                      |
| organization_id | string     | FK → Organization (NEW)        |
| user_role       | string     | RBAC role                      |
| max_budget      | decimal    |                                |
| spend           | decimal    |                                |
| models          | []string   |                                |
| tpm_limit       | int64      |                                |
| rpm_limit       | int64      |                                |

### VirtualKey (EXISTS — needs update)

| Field           | Type       | Notes                          |
| --------------- | ---------- | ------------------------------ |
| token           | string     | SHA256 hash, primary key       |
| key_name        | string     | Human-readable name            |
| key_alias       | string     | Short alias                    |
| user_id         | string     | FK → User                      |
| team_id         | string     | FK → Team                      |
| organization_id | string     | FK → Organization (NEW)        |
| max_budget      | decimal    |                                |
| spend           | decimal    |                                |
| models          | []string   | Allowed models                 |
| tpm_limit       | int64      |                                |
| rpm_limit       | int64      |                                |
| max_parallel    | int        | Max concurrent requests (NEW)  |
| budget_duration | string     |                                |
| budget_reset_at | timestamp  |                                |
| blocked         | bool       |                                |
| guardrails      | []string   | Assigned guardrail names (NEW) |
| permissions     | jsonb      |                                |
| metadata        | jsonb      |                                |
| expires         | timestamp  |                                |

### SpendLog (EXISTS — needs dimensions)

| Field           | Type       | Notes                          |
| --------------- | ---------- | ------------------------------ |
| request_id      | string     | Primary key                    |
| api_key         | string     | Token hash                     |
| user_id         | string     |                                |
| team_id         | string     |                                |
| organization_id | string     | NEW                            |
| model           | string     | Model name used                |
| provider        | string     | Provider name (NEW)            |
| call_type       | string     | chat/embedding/etc             |
| spend           | decimal    | Cost in USD                    |
| total_tokens    | int        |                                |
| prompt_tokens   | int        |                                |
| completion_tokens | int      |                                |
| request_tags    | []string   | User-defined tags (NEW)        |
| start_time      | timestamp  |                                |
| end_time        | timestamp  |                                |
| cache_hit       | bool       | NEW                            |
| status          | string     | success/failure                |

### Credential (NEW — Phase 2)

| Field           | Type       | Notes                          |
| --------------- | ---------- | ------------------------------ |
| credential_id   | string     | Primary key                    |
| credential_name | string     | Reference name                 |
| provider        | string     | Provider type                  |
| credential_values | bytea   | Encrypted JSON blob (NaCl SecretBox — XSalsa20-Poly1305, key = SHA256(master_key or TIANJI_SALT_KEY), base64url-encoded output) |
| created_by      | string     | FK → User                      |
| created_at      | timestamp  |                                |
| updated_at      | timestamp  |                                |

### Callback (NEW — Phase 3, in-memory registry)

| Field           | Type       | Notes                          |
| --------------- | ---------- | ------------------------------ |
| name            | string     | Unique identifier              |
| type            | string     | webhook/prometheus/otel/etc    |
| config          | map        | Type-specific config           |

**Not stored in DB** — registered from YAML config at startup.

### Guardrail (NEW — Phase 4, in-memory registry)

| Field               | Type       | Notes                      |
| ------------------- | ---------- | -------------------------- |
| guardrail_name      | string     | Unique identifier          |
| supported_hooks     | []string   | pre_call, post_call, etc   |
| default_on          | bool       | Apply to all requests      |
| config              | map        | Type-specific config       |

**Not stored in DB** — registered from YAML config at startup. Assigned to keys/teams via `guardrails` field.

### ModelPricing (NEW — Phase 3, embedded JSON)

| Field                   | Type    | Notes                     |
| ----------------------- | ------- | ------------------------- |
| model_name              | string  | Key in JSON               |
| input_cost_per_token    | float64 | USD per input token       |
| output_cost_per_token   | float64 | USD per output token      |
| max_tokens              | int     | Context window size       |
| max_input_tokens        | int     |                           |
| max_output_tokens       | int     |                           |
| tianji_provider        | string  | Provider name             |

**Loaded from embedded JSON** at startup, overridable via `custom_pricing` in config.

### Policy (NEW — Phase 5, in-memory from config)

| Field               | Type       | Notes                                     |
| ------------------- | ---------- | ----------------------------------------- |
| policy_name         | string     | Unique identifier                         |
| conditions          | []Condition | Match rules (team_alias, key_alias, model, tags) |
| guardrails          | []string   | Guardrail names to apply when matched     |
| routing_strategy    | string     | Override strategy for matched requests     |
| metadata            | map        | Arbitrary policy metadata                 |

**Not stored in DB** — loaded from YAML config at startup. Python stores policies in DB with `TianjiLLM_PolicyTable` but initial Go implementation uses config-only.

#### Condition

| Field       | Type     | Notes                                      |
| ----------- | -------- | ------------------------------------------ |
| field       | string   | Match field: "team_alias", "key_alias", "model", "tags" |
| pattern     | string   | Wildcard pattern (e.g., "gpt-*", "healthcare-*") |

#### GuardrailBinding (implicit — not a separate table)

A policy's `guardrails` field binds guardrail names to that policy's match scope. When a request matches a policy's conditions, the listed guardrails are applied in addition to any key/team-level guardrails.

#### RoutingRule (implicit — embedded in Policy)

A policy's `routing_strategy` field overrides the default routing strategy for matched requests. Combined with `conditions`, this enables conditional routing (e.g., route all "healthcare" tagged requests through cost-optimized strategy).

## State Transitions

### VirtualKey Lifecycle

```
Created → Active → Blocked → Active (unblock)
                  → Expired (time-based)
                  → Deleted (hard delete)
```

### Budget Lifecycle

```
Active → Warning (>80% spend) → Exceeded → Reset (on budget_duration cycle)
```

### Deployment Health

```
Healthy → Degraded (failures < allowed_fails) → Cooldown (failures >= allowed_fails)
       → Healthy (after cooldown_time expires or success recorded)
```
