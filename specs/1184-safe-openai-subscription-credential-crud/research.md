# Research: Safe OpenAI Subscription Credential CRUD

## Repo Reality

### Existing routes and auth

`internal/proxy/server.go` already defines `/credentials/new`、`/credentials/list`、`/credentials/info/{credential_id}`、`/credentials/update`、`/credentials/delete/{credential_id}` inside a route group using `s.AuthMiddleware`。

`internal/auth/rbac.go` maps `/credentials` prefix to `RoleProxyAdmin`，符合 Linear 要求的 existing auth/session/permission conventions。

**Decision**: Reuse existing `/credentials` management route family and harden OpenAI subscription behavior there unless implementation proves a typed alias is required。

### Existing credential storage

`internal/db/schema/004_credentials.up.sql` already provides `credential_type`、encrypted `credential_value`、`credential_info JSONB`、`organization_id`、timestamps。

`internal/db/queries/credential.sql` already provides create, get, list, list by org, update value, update info, update value+info, and delete operations。

**Decision**: No schema migration planned。Use existing sqlc-backed methods first。

### Existing OpenAI subscription helpers

`internal/proxy/handler/credentials.go` already defines:

- `CredentialTypeOpenAISubscription = "openai_subscription"`
- `OpenAISubscriptionTokenBundle`
- `OpenAISubscriptionCredentialInfo`
- token bundle canonicalization and validation
- encrypted persistence helpers
- safe metadata marshaling/sanitization
- redacted response mapping

**Decision**: Build HO-1184 around these helper contracts instead of inventing a second credential model。

### Current gap

Current generic `CredentialUpdate` accepts `credential_id` and `credential_value` only。HO-1184 needs explicit metadata update behavior, cross-type guards, and idempotent local delete tests around `openai_subscription` CRUD。

**Decision**: Add tests first for missing API guarantees, then implement the narrowest handler changes needed。

## External Evidence

### OpenAI key safety

OpenAI production best practices say OpenAI API keys are used for authentication and should be kept out of code/public repositories; applications should load them through environment variables or secret-management services: <https://platform.openai.com/docs/guides/production-best-practices>

OpenAI API reference security guidance also says API keys should be securely loaded from environment variables or key management services on the server: <https://platform.openai.com/docs/api-reference/debugging-requests>

**Decision**: Tianji management APIs must treat OpenAI subscription token bundles as secret material: write-only input, encrypted persistence, redacted output。

### OpenAI delete/revoke boundary

OpenAI Help Center exposes API-key deletion as an OpenAI-side account/dashboard operation: <https://help.openai.com/en?q=api+key>

**Decision**: HO-1184 delete means Tianji local row deletion only。It must not imply or attempt OpenAI remote revoke/logout/delete。

## Alternatives Considered

### Alternative A - Add new `/openai/subscription/credentials/*` routes

Rejected for Todo plan。Repo already has authenticated credential CRUD and RBAC for `/credentials`。Parallel routes would increase API surface and duplicate permission behavior。

### Alternative B - Return encrypted `credential_value` for admins

Rejected。Encrypted blobs are still sensitive operational material and sibling specs already define redacted management responses。

### Alternative C - Remote revoke on delete

Rejected。Linear scope says delete removes local credential only and no remote OpenAI revoke。Remote revoke would require OAuth provider semantics and error handling outside this CRUD slice。

### Alternative D - Allow update to change `credential_type`

Rejected。Changing type in place risks treating opaque API-key values as subscription token bundles or the reverse。Type migration should be explicit and out of scope。
