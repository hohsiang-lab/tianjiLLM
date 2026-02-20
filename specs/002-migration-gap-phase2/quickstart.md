# Quickstart — Phase 2 Implementation

## Prerequisites

- Phase 1 complete (all 99 tasks)
- Go 1.22+
- `make check` passing
- Access to Python TianjiLLM source at `/Users/norman/src/github.com/upstream Python TianjiLLM`

## Implementation Order

### Round 1: Secret Managers (P0 — blocks enterprise adoption)

1. Create `internal/secretmanager/secretmanager.go` — interface + registry + cached wrapper
2. Implement AWS Secrets Manager (`aws.go`)
3. Implement Google Secret Manager (`google.go`)
4. Implement Azure Key Vault (`azure.go`)
5. Implement HashiCorp Vault (`vault.go`)
6. Integrate with `internal/config/config.go` — resolve `os.environ/SECRET_NAME` syntax
7. Contract tests with mock HTTP servers

### Round 2: Enterprise Guardrails (P0)

8. Implement AWS Bedrock Guardrails (`internal/guardrail/bedrock.go`)
9. Implement Azure Content Safety — text moderation + prompt shield
10. Implement Lakera AI v2
11. Implement Generic Guardrail API
12. Implement built-in Content Filter
13. Implement Tool Permission guardrail
14. Add fail-open/fail-closed policy support

### Round 3: Cloud Storage Logging (P1)

15. Create `internal/callback/batchlogger.go` — shared batch base
16. Implement S3 Logger
17. Implement GCS Logger
18. Implement Azure Blob Logger
19. Implement DynamoDB Logger
20. Implement SQS Logger
21. Implement Email Alerting (SMTP)

### Round 4: Providers + Observability (P1)

22. Implement Vertex AI provider (extends Gemini)
23. Implement SageMaker provider
24. Implement Langsmith callback
25. Implement Helicone callback (header injection)
26. Implement Braintrust callback
27. Implement MLflow callback
28. Implement Arize/Phoenix (OTEL exporter)
29. Implement W&B callback

### Round 5: Cache + Router + Analytics (P2)

30. Implement Redis Cluster cache
31. Implement Semantic Cache (Redis Stack + embeddings)
32. Implement Disk Cache
33. Implement Least Busy router strategy
34. Implement Budget Limiter router strategy
35. Implement Advanced Spend Analytics endpoints
36. Implement Prompt Management (Langfuse + generic)

## Quick Verification

After each round:

```bash
make check                    # lint + test + build
go test ./internal/secretmanager/... -v   # round 1
go test ./internal/guardrail/... -v       # round 2
go test ./internal/callback/... -v        # round 3
go test ./internal/provider/vertexai/... -v  # round 4
go test ./internal/cache/... -v           # round 5
```

## Config Example (Target State)

```yaml
general_settings:
  secret_manager:
    type: aws_secrets_manager
    region: us-east-1

  guardrails:
    - name: bedrock-safety
      type: bedrock
      guardrail_id: "abc123"
      hooks: [pre_call, post_call]
      failure_policy: fail_closed

    - name: pii-filter
      type: lakera
      api_key: "os.environ/LAKERA_API_KEY"
      hooks: [pre_call]

  callbacks:
    - type: s3
      bucket: tianji-logs
      prefix: production/
      flush_interval: 5s
      batch_size: 512

    - type: langsmith
      api_key: "os.environ/LANGSMITH_API_KEY"
      project: my-project

  cache:
    type: redis_cluster
    addrs:
      - "redis-1:7000"
      - "redis-2:7001"
      - "redis-3:7002"

  prompt_management:
    type: langfuse
    public_key: "os.environ/LANGFUSE_PUBLIC_KEY"
    secret_key: "os.environ/LANGFUSE_SECRET_KEY"
```
