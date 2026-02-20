# API Contracts: TianjiLLM-Go Migration

**Date**: 2026-02-16
**Branch**: `001-migration-gap-analysis`

All contracts follow Python TianjiLLM's exact request/response format. Responses are OpenAI-compatible unless noted.

## Phase 1: New LLM Endpoints

### Files API

| Method | Path                        | Request              | Response             |
| ------ | --------------------------- | -------------------- | -------------------- |
| POST   | /v1/files                   | multipart/form-data  | OpenAIFileObject     |
| GET    | /v1/files                   | query: purpose       | {data: [FileObject]} |
| GET    | /v1/files/{file_id}         | —                    | OpenAIFileObject     |
| GET    | /v1/files/{file_id}/content | —                    | file bytes           |
| DELETE | /v1/files/{file_id}         | —                    | {id, deleted: true}  |

### Batches API

| Method | Path                            | Request              | Response          |
| ------ | ------------------------------- | -------------------- | ----------------- |
| POST   | /v1/batches                     | {input_file_id, ...} | BatchObject       |
| GET    | /v1/batches/{batch_id}          | —                    | BatchObject       |
| POST   | /v1/batches/{batch_id}/cancel   | —                    | BatchObject       |
| GET    | /v1/batches                     | query: limit, after  | {data: [Batch]}   |

### Fine-tuning API

| Method | Path                                          | Request               | Response              |
| ------ | --------------------------------------------- | --------------------- | --------------------- |
| POST   | /v1/fine_tuning/jobs                          | {training_file, ...}  | FineTuningJob         |
| GET    | /v1/fine_tuning/jobs/{job_id}                 | —                     | FineTuningJob         |
| POST   | /v1/fine_tuning/jobs/{job_id}/cancel          | —                     | FineTuningJob         |
| GET    | /v1/fine_tuning/jobs/{job_id}/events          | query: limit, after   | {data: [Event]}       |
| GET    | /v1/fine_tuning/jobs/{job_id}/checkpoints     | query: limit, after   | {data: [Checkpoint]}  |

### Rerank API

| Method | Path        | Request                              | Response                         |
| ------ | ----------- | ------------------------------------ | -------------------------------- |
| POST   | /v1/rerank  | {model, query, documents, top_n}     | {results: [{index, relevance}]}  |

### Pass-through

| Method  | Path                          | Request | Response       |
| ------- | ----------------------------- | ------- | -------------- |
| ANY     | /v1/{provider}/{path...}      | raw     | raw (proxied)  |

## Phase 2: Management Endpoints

### Organization CRUD (NEW)

| Method | Path                            | Request                       | Response        |
| ------ | ------------------------------- | ----------------------------- | --------------- |
| POST   | /organization/new               | {name, max_budget, models}    | Organization    |
| GET    | /organization/info              | query: organization_id        | Organization    |
| PATCH  | /organization/update            | {organization_id, ...fields}  | Organization    |
| DELETE | /organization/delete            | {organization_id}             | {deleted: true} |
| POST   | /organization/member/add        | {organization_id, user_id}    | Organization    |
| DELETE | /organization/member/delete     | {organization_id, user_id}    | Organization    |

### Key Update (MISSING)

| Method | Path         | Request                    | Response          |
| ------ | ------------ | -------------------------- | ----------------- |
| POST   | /key/update  | {key, ...fields_to_update} | VerificationToken |

### Team Update + Members (MISSING)

| Method | Path                 | Request                           | Response |
| ------ | -------------------- | --------------------------------- | -------- |
| POST   | /team/update         | {team_id, ...fields_to_update}    | Team     |
| POST   | /team/member/add     | {team_id, member: {user_id, role}}| Team     |
| POST   | /team/member/delete  | {team_id, user_id}                | Team     |

### Budget (STUB → REAL)

| Method | Path          | Request                              | Response   |
| ------ | ------------- | ------------------------------------ | ---------- |
| POST   | /budget/new   | {max_budget, budget_duration, ...}   | Budget     |
| GET    | /budget/info  | query: budget_id                     | Budget     |

### Credential Management (NEW)

| Method | Path                 | Request                                    | Response    |
| ------ | -------------------- | ------------------------------------------ | ----------- |
| POST   | /credentials/new     | {name, provider, credential_values}        | Credential  |
| GET    | /credentials/list    | —                                          | [Credential]|
| GET    | /credentials/info    | query: credential_id                       | Credential  |
| PATCH  | /credentials/update  | {credential_id, ...fields}                 | Credential  |
| DELETE | /credentials/delete  | {credential_id}                            | {deleted}   |

### Access Groups (NEW)

| Method | Path                        | Request                   | Response     |
| ------ | --------------------------- | ------------------------- | ------------ |
| POST   | /model_access_group/new     | {name, models}            | AccessGroup  |
| GET    | /model_access_group/info    | query: group_id           | AccessGroup  |
| PUT    | /model_access_group/update  | {group_id, models}        | AccessGroup  |
| DELETE | /model_access_group/delete  | {group_id}                | {deleted}    |

## Phase 3: Observability Endpoints

### Spend Analytics (EXPAND)

| Method | Path             | Description                    |
| ------ | ---------------- | ------------------------------ |
| GET    | /spend/keys      | Existing                       |
| GET    | /spend/users     | Existing                       |
| GET    | /spend/teams     | NEW — aggregate by team        |
| GET    | /spend/tags      | NEW — aggregate by request tag |
| GET    | /spend/models    | NEW — aggregate by model       |
| GET    | /spend/end_users | NEW — aggregate by end_user (customer) |

### Callback Management

| Method | Path            | Request               | Response      |
| ------ | --------------- | --------------------- | ------------- |
| GET    | /callback/list  | —                     | [CallbackInfo]|

### Cache Management (NEW)

| Method | Path             | Request        | Response                |
| ------ | ---------------- | -------------- | ----------------------- |
| GET    | /cache/ping      | —              | {status: "healthy"}     |
| POST   | /cache/delete    | {keys: [...]}  | {deleted_count: N}      |
| POST   | /cache/flushall  | —              | {status: "flushed"}     |

### Health Endpoints (EXPAND)

| Method | Path              | Response                           |
| ------ | ----------------- | ---------------------------------- |
| GET    | /health           | Existing                           |
| GET    | /health/readiness | Existing                           |
| GET    | /health/liveness  | Existing                           |
| GET    | /health/services  | NEW — DB, Redis, provider statuses |

## Phase 5: Routing Endpoints (NEW)

| Method | Path              | Request                 | Response            |
| ------ | ----------------- | ----------------------- | ------------------- |
| GET    | /router/settings  | —                       | RouterConfig        |
| PATCH  | /router/settings  | {routing_strategy, ...} | RouterConfig        |

## SSO/Auth Endpoints (Phase 2)

| Method | Path          | Request               | Response                |
| ------ | ------------- | --------------------- | ----------------------- |
| GET    | /sso/callback | query: code, state    | {token, user_info}      |
| GET    | /sso/login    | —                     | redirect to IDP         |
