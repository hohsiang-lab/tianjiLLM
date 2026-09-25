-- name: CreateCredential :one
INSERT INTO "CredentialTable" (credential_id, credential_name, credential_type, credential_value, credential_info, organization_id, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetCredential :one
SELECT * FROM "CredentialTable" WHERE credential_id = $1;

-- name: ListCredentials :many
SELECT * FROM "CredentialTable" ORDER BY created_at DESC;

-- name: ListCredentialsByOrg :many
SELECT * FROM "CredentialTable" WHERE organization_id = $1 ORDER BY created_at DESC;

-- name: ListOpenAISubscriptionRefreshCandidates :many
SELECT * FROM "CredentialTable"
WHERE credential_type = 'openai_subscription'
  AND COALESCE(credential_info->>'status', 'active') NOT IN ('disabled', 'refresh_failed')
  AND COALESCE(credential_info->>'disabled_reason', '') NOT IN ('refresh_failed', 'refresh_token_invalidated', 'auth_failed_after_refresh', 'operator_disabled')
ORDER BY created_at ASC;

-- name: UpdateCredential :exec
UPDATE "CredentialTable"
SET credential_value = $2, updated_at = NOW()
WHERE credential_id = $1;

-- name: UpdateCredentialValueAndInfo :exec
UPDATE "CredentialTable"
SET credential_value = $2, credential_info = $3, updated_at = NOW()
WHERE credential_id = $1;

-- name: UpdateCredentialInfo :exec
UPDATE "CredentialTable"
SET credential_info = $2, updated_at = NOW()
WHERE credential_id = $1;

-- name: DeleteCredential :exec
DELETE FROM "CredentialTable" WHERE credential_id = $1;
