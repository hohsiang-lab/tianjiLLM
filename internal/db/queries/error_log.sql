-- name: InsertErrorLog :exec
INSERT INTO "ErrorLogs" (
    request_id, api_key_hash, model, provider,
    status_code, error_type, error_message, traceback
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: ListErrorLogs :many
SELECT * FROM "ErrorLogs"
WHERE ($1::text = '' OR model = $1)
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;
