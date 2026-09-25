# Quickstart: HO-1183 Verification

Run after implementation:

```bash
go test ./internal/auth/... ./internal/proxy/middleware/... ./internal/proxy/handler/... -run 'TestRedact|TestOpenAISubscriptionCredentialFailureMetadata|TestCredentialInfo_RedactsNestedAccountPayload|TestRecordErrorLog_RedactsUpstreamTokenText|TestCreateAuditLog_RedactsCredentialPayloads|TestJWTValidationFailure_DoesNotLogRawToken' -v
go test ./internal/auth/... ./internal/proxy/middleware/... ./internal/proxy/handler/...
git diff --check
```

Manual fixture checklist:

1. Use synthetic access token, refresh token, API key, and JWT strings only.
2. Put those fixtures in credential metadata, upstream error text, ErrorLogs, and AuditLog payloads.
3. Assert every persisted/returned/logged body excludes the raw fixture strings.
4. Assert safe metadata remains visible.
