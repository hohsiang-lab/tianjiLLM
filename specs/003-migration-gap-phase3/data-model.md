# Data Model: Phase 3 — Enterprise Features & Full Parity

**Branch**: `003-migration-gap-phase3` | **Date**: 2026-02-17

## Entity Overview

```
Policy ──1:N──> PolicyAttachment
  │                  │
  └── parent ────┘   └── multi-dimensional scope (teams[], keys[], models[], tags[])
  │
  └──1:N──> PipelineStep ──ref──> Guardrail (existing)

SCIM User ──reuses──> User (existing table, metadata["scim_*"] fields)
SCIM Group ──reuses──> Team (existing table, metadata["scim_data"] field)

BackgroundJob (runtime only, not persisted)

ModelDeployment (existing, extended with CRUD)
Tag (new entity for spend attribution)
EndUser/Customer (new entity for end-user tracking)
GuardrailConfig (new entity for CRUD management)
PromptTemplate (new entity with versioning)
SpendArchive (metadata for cold storage batches)
IPWhitelistEntry (new entity for access control)
```

## Entity Definitions

### Policy (Work Stream A)

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | uuid | PK | Unique identifier |
| name | string | UNIQUE, NOT NULL | Human-readable policy name |
| parent_id | uuid | FK → Policy.id, NULLABLE | Parent policy for inheritance |
| conditions | jsonb | NULLABLE | Match conditions (e.g., `{"model": "gpt-4.*"}`) — uses regexp matching |
| guardrails_add | text[] | NOT NULL, default `{}` | Guardrails to add in this policy |
| guardrails_remove | text[] | NOT NULL, default `{}` | Guardrails to remove from parent chain |
| pipeline | jsonb | NULLABLE | Optional guardrail pipeline (mode + ordered steps with on_pass/on_fail) |
| description | string | NULLABLE | Human-readable description |
| created_by | string | NULLABLE | Admin who created this policy |
| created_at | timestamptz | NOT NULL | Creation timestamp |
| updated_at | timestamptz | NOT NULL | Last update timestamp |

**Validation Rules**:
- `parent_id` cannot create cycles (validated at write time via DFS)
- `name` must be unique across all policies
- `guardrails_add` and `guardrails_remove` must reference valid guardrail names

**State Transitions**: None (stateless config entity)

### PolicyAttachment (Work Stream A)

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | uuid | PK | Unique identifier |
| policy_name | string | FK → Policy.name, NOT NULL | The policy being attached (by name, matching Python) |
| scope | string | NULLABLE | `"*"` for global, null otherwise |
| teams | text[] | NOT NULL, default `{}` | Team IDs this attachment applies to (supports wildcards) |
| keys | text[] | NOT NULL, default `{}` | Key hashes this attachment applies to (supports wildcards) |
| models | text[] | NOT NULL, default `{}` | Model names this attachment applies to (supports wildcards) |
| tags | text[] | NOT NULL, default `{}` | Request tags this attachment applies to (supports wildcards) |
| created_by | string | NULLABLE | Admin who created this attachment |
| created_at | timestamptz | NOT NULL | Creation timestamp |
| updated_at | timestamptz | NOT NULL | Last update timestamp |

**Validation Rules**:
- `scope="*"` means global — applies to all requests (teams/keys/models/tags ignored)
- When `scope` is null, at least one of teams/keys/models/tags must be non-empty
- Wildcard entries (e.g., `"team-finance-*"`) use **simple prefix matching** (endsWith `*` → startsWith prefix), NOT regexp
- One policy can have multiple attachments (different scope combinations)
- A single attachment can match multiple dimensions simultaneously (AND logic across non-empty arrays)
- Tags dimension is opt-in: if scope has no tags, tag matching is skipped

**Matching distinction**: Policy *conditions* (model name) use `regexp.MatchString()`. Attachment *scope* wildcards use simple prefix matching (`strings.HasPrefix()`). These are intentionally different — matching Python TianjiLLM's design.

**Design Note**: This matches Python's `TianjiLLM_PolicyAttachmentTable` which uses multi-dimensional arrays, not single scope_type/scope_id. This enables policies like "apply to team-finance AND gpt-4 models" in a single attachment.

### PipelineStep (Work Stream A, stored as JSONB in Policy.pipeline)

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| guardrail | string | NOT NULL | Guardrail name to execute |
| on_pass | string | NOT NULL, enum: next/allow/modify_response | Action when guardrail passes |
| on_fail | string | NOT NULL, enum: next/block/modify_response | Action when guardrail fails |
| pass_data | boolean | NOT NULL, default false | Whether to forward modified data to next step (e.g., PII-masked content) |
| modify_response_message | string | NULLABLE | Custom response message when action is `modify_response` |

**Pipeline is stored as JSONB on Policy** (not a separate table):
```json
{
  "mode": "pre_call",
  "steps": [
    {"guardrail": "pii_detector", "on_pass": "next", "on_fail": "block", "pass_data": false},
    {"guardrail": "pii_masking", "on_pass": "allow", "on_fail": "block", "pass_data": true}
  ]
}
```

**Validation Rules**:
- `guardrail` must reference a registered guardrail name
- `mode` must be `"pre_call"` or `"post_call"`
- Actions: `next` (continue), `allow` (terminal — pass request), `block` (terminal — reject), `modify_response` (terminal — return custom message)

**Design Note**: Matches Python's `pipeline` JSONB field on Policy table, not a separate `policy_pipeline_steps` table. This simplifies CRUD — the pipeline is just part of the policy document.

### SCIM User/Group Mapping (Work Stream B — No New Tables)

SCIM 2.0 **reuses existing User and Team tables** (matching Python TianjiLLM). No dedicated SCIM tables are created. SCIM-specific data is stored in the existing `metadata` JSONB field.

#### SCIM User → User Table Mapping

| SCIM Attribute | User Table Field | Notes |
|----------------|-----------------|-------|
| `userName` | `user_id` | Primary identifier, UNIQUE |
| `externalId` | `sso_user_id` | IDP external identifier |
| `emails[0].value` | `user_email` | Primary email |
| `name.givenName` | `user_alias` | Display name; also stored in `metadata["scim_metadata"]["givenName"]` |
| `name.familyName` | — | Stored in `metadata["scim_metadata"]["familyName"]` |
| `active` | — | Stored in `metadata["scim_active"]` (bool) |
| `groups[].value` | `teams[]` | Team membership array |
| `meta.created` | `created_at` | RFC 7644 meta |
| `meta.lastModified` | `updated_at` | RFC 7644 meta |

#### SCIM Group → Team Table Mapping

| SCIM Attribute | Team Table Field | Notes |
|----------------|-----------------|-------|
| `id` | `team_id` | UUID, PK |
| `displayName` | `team_alias` | Team display name, UNIQUE |
| `members[].value` | `members[]` | Array of user IDs |
| `externalId` | — | Stored in `metadata["externalId"]` |
| (full SCIM JSON) | — | Stored in `metadata["scim_data"]` for round-trip fidelity |
| `meta.created` | `created_at` | RFC 7644 meta |
| `meta.lastModified` | `updated_at` | RFC 7644 meta |

**Validation Rules**:
- `userName` maps to `user_id` which must be unique
- When `metadata["scim_active"]` transitions to `false`, all associated virtual keys are deactivated
- `externalId` uniqueness enforced when provided (IDP dedup)
- `members[].value` must reference existing users (or auto-create if `scim_upsert_user: true`)

**State Transitions**:
- `active: true → false` — Deactivate user, revoke keys
- `active: false → true` — Reactivate user, keys remain inactive (must be manually re-enabled)

**Design Note**: This matches Python TianjiLLM which has NO dedicated SCIM tables — it directly reads/writes the existing `tianji_usertable` and `tianji_teamtable`, storing SCIM-specific attributes in `metadata` JSONB.

### Tag (Work Stream F)

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | uuid | PK | Unique identifier |
| name | string | UNIQUE, NOT NULL | Tag name |
| description | string | NULLABLE | Human-readable description |
| created_at | timestamptz | NOT NULL | Creation timestamp |

### EndUser (Work Stream F)

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | uuid | PK | Unique identifier |
| end_user_id | string | UNIQUE, NOT NULL | External end user identifier |
| alias | string | NULLABLE | Display name |
| allowed_model_region | string | NULLABLE | Region restriction |
| default_model | string | NULLABLE | Default model for this user |
| budget | decimal | NULLABLE | Budget limit |
| blocked | boolean | NOT NULL, default false | Block status |
| metadata | jsonb | NULLABLE | Custom metadata |
| created_at | timestamptz | NOT NULL | Creation timestamp |
| updated_at | timestamptz | NOT NULL | Last update timestamp |

### GuardrailConfig (Work Stream K)

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | uuid | PK | Unique identifier |
| guardrail_name | string | UNIQUE, NOT NULL | Guardrail identifier |
| guardrail_type | string | NOT NULL | Type (e.g., bedrock, presidio, lakera, content_filter, generic_api) |
| config | jsonb | NOT NULL | Type-specific configuration |
| failure_policy | string | NOT NULL, default 'fail_closed' | fail_open or fail_closed |
| enabled | boolean | NOT NULL, default true | Active status |
| created_at | timestamptz | NOT NULL | Creation timestamp |
| updated_at | timestamptz | NOT NULL | Last update timestamp |

### PromptTemplate (Work Stream K)

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | uuid | PK | Unique identifier |
| name | string | NOT NULL | Prompt name |
| version | int | NOT NULL | Version number (auto-increment per name) |
| template | text | NOT NULL | Prompt template with `{{variable}}` placeholders |
| variables | text[] | NOT NULL, default `{}` | List of required variable names |
| model | string | NULLABLE | Suggested model for this prompt |
| metadata | jsonb | NULLABLE | Custom metadata |
| created_at | timestamptz | NOT NULL | Creation timestamp |

**Validation Rules**:
- `(name, version)` must be UNIQUE
- Creating a new prompt with existing name auto-increments version
- `variables` must match placeholders found in `template`

### SpendArchive (Work Stream I)

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | uuid | PK | Unique identifier |
| date_from | date | NOT NULL | Start of archived date range |
| date_to | date | NOT NULL | End of archived date range |
| storage_type | string | NOT NULL | s3 or gcs |
| storage_location | string | NOT NULL | Full URI (e.g., `s3://bucket/prefix/2026-01/`) |
| entry_count | bigint | NOT NULL | Number of entries archived |
| exported_at | timestamptz | NOT NULL | Export timestamp |

**Validation Rules**:
- `(date_from, date_to)` ranges must not overlap (idempotent archival)
- `entry_count` must match actual exported count

### IPWhitelistEntry (Work Stream L)

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | uuid | PK | Unique identifier |
| ip_address | string | NOT NULL | IP address or CIDR range |
| description | string | NULLABLE | Reason for whitelisting |
| created_by | string | NOT NULL | Admin who created this entry |
| created_at | timestamptz | NOT NULL | Creation timestamp |

**Validation Rules**:
- `ip_address` must be valid IPv4/IPv6 address or CIDR notation
- Duplicate IP addresses rejected

## Relationship Diagram

```
┌─────────────────────────────────────────────────┐
│                  Policy Engine                   │
│                                                  │
│  Policy ──parent──> Policy (tree, max depth ~10) │
│    │                                             │
│    ├── PipelineStep[] (ordered guardrail list)    │
│    │       └── ref → Guardrail (existing)        │
│    │                                             │
│    └── PolicyAttachment[]                        │
│            └── multi-dim: teams[] + keys[]       │
│                + models[] + tags[] | scope="*"   │
└─────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────┐
│              SCIM 2.0 (No New Tables)            │
│                                                  │
│  SCIM User ──reuses──> User (existing)           │
│    └── userName → user_id                        │
│    └── active → metadata["scim_active"]          │
│    └── deactivate → revoke keys                  │
│                                                  │
│  SCIM Group ──reuses──> Team (existing)          │
│    └── displayName → team_alias                  │
│    └── members[] → User.teams[]                  │
│    └── upsert_user config controls auto-create   │
└─────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────┐
│              Management Extensions               │
│                                                  │
│  Tag ──ref by──> SpendLog (existing)             │
│  EndUser ──ref by──> SpendLog (existing)         │
│  GuardrailConfig ──ref by──> Policy.PipelineStep │
│  PromptTemplate (standalone, versioned)          │
│  IPWhitelistEntry (standalone)                   │
└─────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────┐
│               Cold Storage                       │
│                                                  │
│  SpendLog (existing) ──archived to──> S3/GCS     │
│  SpendArchive (tracks what was archived)         │
└─────────────────────────────────────────────────┘
```

## Database Migrations Required

1. `CREATE TABLE policies` — Policy entity (includes `pipeline` JSONB column for pipeline steps)
2. `CREATE TABLE policy_attachments` — PolicyAttachment entity (multi-dimensional: teams[], keys[], models[], tags[])
3. `CREATE TABLE tags` — Tag entity
4. `CREATE TABLE end_users` — EndUser entity
5. `CREATE TABLE guardrail_configs` — GuardrailConfig entity
6. `CREATE TABLE prompt_templates` — PromptTemplate entity
7. `CREATE TABLE spend_archives` — SpendArchive entity
8. `CREATE TABLE ip_whitelist` — IPWhitelistEntry entity
9. `CREATE INDEX` — indexes on foreign keys, lookup fields, date ranges

**Note**: No SCIM-specific tables needed — SCIM reuses existing `users` and `teams` tables (matching Python TianjiLLM).
