-- name: InsertErrorLog :exec
INSERT INTO "ErrorLogs" (
    request_id, api_key_hash, model, provider,
    status_code, error_type, error_message, traceback, team_id
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: ListErrorLogs :many
SELECT
    id,
    request_id,
    api_key_hash,
    model,
    provider,
    status_code,
    error_type,
    error_message,
    traceback,
    created_at,
    team_id
FROM "ErrorLogs"
WHERE (sqlc.narg(filter_model)::text IS NULL OR model = sqlc.narg(filter_model))
ORDER BY created_at DESC
LIMIT sqlc.arg(query_limit) OFFSET sqlc.arg(query_offset);
