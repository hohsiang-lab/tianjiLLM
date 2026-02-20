# Implementation Plan: TianjiLLM Go Migration Phase 2 — Enterprise Production Readiness

**Branch**: `002-migration-gap-phase2` | **Date**: 2026-02-16 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/002-migration-gap-phase2/spec.md`

## Summary

Phase 2 closes the enterprise production readiness gap by adding: secret managers (AWS/GCP/Azure/Vault), enterprise guardrails (Bedrock/Azure/Lakera/Generic), cloud storage logging (S3/GCS/Azure Blob/DynamoDB/SQS), email alerting, additional providers (Vertex AI, SageMaker), observability integrations (Langsmith/Helicone/Braintrust/Arize/MLflow/W&B), Redis Cluster, semantic cache, prompt management (Langfuse), and advanced routing strategies (least-busy, budget-limited).

All 42 functional requirements verified against Python TianjiLLM source code. Technology choices documented in [research.md](research.md) with external source verification per constitution.

## Technical Context

**Language/Version**: Go 1.22+ (latest stable)
**Primary Dependencies**:
- `github.com/aws/aws-sdk-go-v2` (Secrets Manager, S3, DynamoDB, SQS, Bedrock, SageMaker)
- `cloud.google.com/go/secretmanager/apiv1` (Google Secret Manager)
- `cloud.google.com/go/storage` (GCS)
- `google.golang.org/genai` (Vertex AI)
- `github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets` (Azure Key Vault)
- `github.com/Azure/azure-sdk-for-go/sdk/storage/azblob` (Azure Blob)
- `github.com/Azure/azure-sdk-for-go/sdk/azidentity` (Azure auth)
- `github.com/hashicorp/vault/api` (HashiCorp Vault)
- `github.com/redis/go-redis/v9` (Redis Cluster — already in project)
- `go.opentelemetry.io/otel` (OTEL — already in project)
- `net/smtp` (stdlib — email alerting)

**Storage**: PostgreSQL (primary, existing), Redis/Redis Cluster (cache, existing go-redis), S3/GCS/Azure Blob (log storage, new)
**Testing**: `go test` + `testify`, `httptest.NewServer` for mock upstreams
**Target Platform**: Linux server (same as Phase 1)
**Project Type**: Single Go project (existing structure)
**Performance Goals**: <50ms guardrail latency, <60s log delivery, <200ms prompt resolution (cached)
**Constraints**: Zero breaking changes to Phase 1 functionality, plugin architecture for all new subsystems
**Scale/Scope**: 42 functional requirements across 9 work streams

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Python-First Reference | ✅ PASS | All 42 FRs verified against Python source code. Clarifications resolved by checking Python behavior. |
| II. Feature Parity | ✅ PASS | API contracts match Python. Config format compatible. `os.environ/` syntax preserved. |
| III. Research Before Build | ✅ PASS | 11 technology decisions documented in research.md with Context7/GitHub/Python source verification. |
| IV. Test-Driven Migration | ✅ PASS | Contract tests planned for every new subsystem. Mock upstream servers for cloud services. |
| V. Go Best Practices | ✅ PASS | Interface-based plugin system. `context.Context` everywhere. Registry pattern. Composition over inheritance. |
| VI. No Stale Knowledge | ✅ PASS | All SDK APIs verified via Context7 docs. No decisions based on agent pre-trained knowledge. |

**Post-design re-check**: All principles still satisfied. No violations requiring justification.

## Project Structure

### Documentation (this feature)

```text
specs/002-migration-gap-phase2/
├── plan.md              # This file
├── research.md          # Phase 0 output — 11 technology decisions
├── data-model.md        # Phase 1 output — entity definitions
├── quickstart.md        # Phase 1 output — implementation order
├── contracts/           # Phase 1 output — Go interface definitions
│   └── interfaces.go
├── checklists/
│   └── requirements.md  # Spec quality validation
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
internal/
├── secretmanager/          # NEW — Work Stream A
│   ├── secretmanager.go    # Interface + Registry + CachedWrapper
│   ├── aws.go              # AWS Secrets Manager
│   ├── google.go           # Google Secret Manager
│   ├── azure.go            # Azure Key Vault
│   └── vault.go            # HashiCorp Vault
│
├── guardrail/              # EXTEND — Work Stream B
│   ├── guardrail.go        # Existing interface (no changes)
│   ├── moderation.go       # Existing
│   ├── presidio.go         # Existing
│   ├── promptinjection.go  # Existing
│   ├── bedrock.go          # NEW — AWS Bedrock Guardrails
│   ├── azure_text_mod.go   # NEW — Azure Content Safety Text Moderation
│   ├── azure_prompt_shield.go # NEW — Azure Prompt Shield
│   ├── lakera.go           # NEW — Lakera AI v2
│   ├── generic.go          # NEW — Generic Guardrail API
│   ├── contentfilter.go    # NEW — Built-in content filter
│   └── toolpermission.go   # NEW — Tool permission guardrail
│
├── callback/               # EXTEND — Work Streams C + E
│   ├── callback.go         # Existing interface (no changes)
│   ├── registry.go         # Existing (no changes)
│   ├── batchlogger.go      # NEW — Shared batch base
│   ├── s3.go               # NEW — S3 Logger
│   ├── gcs.go              # NEW — GCS Logger
│   ├── azureblob.go        # NEW — Azure Blob Logger
│   ├── dynamodb.go         # NEW — DynamoDB Logger
│   ├── sqs.go              # NEW — SQS Logger
│   ├── email.go            # NEW — SMTP Email Alerting
│   ├── langsmith.go        # NEW — Langsmith
│   ├── helicone.go         # NEW — Helicone (post-request HTTP logger)
│   ├── braintrust.go       # NEW — Braintrust
│   ├── mlflow.go           # NEW — MLflow
│   └── wandb.go            # NEW — Weights & Biases
│   # Existing: prometheus.go, otel.go, datadog.go, langfuse.go, slack.go, webhook.go
│
├── provider/               # EXTEND — Work Stream D
│   ├── vertexai/           # NEW — Vertex AI
│   │   ├── vertexai.go
│   │   └── stream.go
│   ├── sagemaker/          # NEW — SageMaker
│   │   └── sagemaker.go
│   ├── ai21/               # NEW — AI21
│   │   └── ai21.go
│   └── watsonx/            # NEW — WatsonX
│       └── watsonx.go
│
├── cache/                  # EXTEND — Work Stream F
│   ├── cache.go            # Existing interface (no changes)
│   ├── redis_cluster.go    # NEW — Redis Cluster
│   ├── semantic.go         # NEW — Semantic Cache
│   └── disk.go             # NEW — Disk Cache
│   # Existing: memory.go, redis.go, dual.go
│
├── prompt/                 # NEW — Work Stream G
│   ├── prompt.go           # Interface + Registry
│   └── langfuse.go         # Langfuse prompt source
│
├── router/strategy/        # EXTEND — Work Stream H
│   ├── leastbusy.go        # NEW — Least Busy
│   └── budgetlimiter.go    # NEW — Budget Limiter
│   # Existing: shuffle.go, latency.go, cost.go, usage.go, tag.go
│
├── spend/                  # EXTEND — Work Stream I
│   ├── analytics.go        # NEW — Advanced analytics queries
│   └── focus.go            # NEW — FinOps FOCUS export
│   # Existing: calculator.go, tracker.go, redis_buffer.go
│
├── config/                 # MODIFY
│   ├── config.go           # Add secret_manager, guardrails, callbacks, prompt config
│   └── loader.go           # Integrate SecretManager resolution
│
└── model/                  # EXTEND
    └── request.go          # Add prompt_reference field (if needed)

test/
├── contract/               # Contract tests for all new handlers
├── integration/            # Full-flow tests
└── fixtures/               # Mock response JSON
```

**Structure Decision**: Extends existing single-project Go structure. All new code goes into `internal/` following the established package convention. No new top-level directories. Every new subsystem follows the existing registry + self-registration pattern.

## Complexity Tracking

> No constitution violations to justify. All new packages follow existing patterns (interface + registry + implementations).
