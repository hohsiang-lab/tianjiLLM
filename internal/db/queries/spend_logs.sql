-- name: CreateSpendLog :exec
INSERT INTO "SpendLogs" (request_id, call_type, api_key, spend, total_tokens, prompt_tokens, completion_tokens, starttime, endtime, model, model_id, model_group, api_base, "user", metadata, cache_hit, cache_key, request_tags, team_id, end_user, requester_ip_address, request_duration_ms, organization_id, provider, upstream_token_key, reasoning_effort)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26);

-- name: GetSpendByKey :many
SELECT api_key, SUM(spend) as total_spend, SUM(total_tokens) as total_tokens
FROM "SpendLogs"
WHERE api_key = ANY($1::text[])
AND starttime >= $2
GROUP BY api_key;

-- name: GetSpendByUser :many
SELECT "user", SUM(spend) as total_spend, SUM(total_tokens) as total_tokens
FROM "SpendLogs"
WHERE "user" = ANY($1::text[])
AND starttime >= $2
GROUP BY "user";

-- name: GetSpendByTag :many
SELECT unnest(request_tags) as tag, SUM(spend) as total_spend, COUNT(*) as request_count
FROM "SpendLogs"
WHERE starttime >= $1
GROUP BY tag
ORDER BY total_spend DESC;

-- name: GetSpendByTeam :many
SELECT team_id, SUM(spend) as total_spend, SUM(total_tokens) as total_tokens
FROM "SpendLogs"
WHERE team_id IS NOT NULL AND starttime >= $1
GROUP BY team_id
ORDER BY total_spend DESC;

-- name: GetSpendByModel :many
SELECT model, SUM(spend) as total_spend, SUM(total_tokens) as total_tokens
FROM "SpendLogs"
WHERE starttime >= $1
GROUP BY model
ORDER BY total_spend DESC;

-- name: GetSpendByEndUser :many
SELECT end_user, SUM(spend) as total_spend, SUM(total_tokens) as total_tokens
FROM "SpendLogs"
WHERE end_user IS NOT NULL AND end_user != '' AND starttime >= $1
GROUP BY end_user
ORDER BY total_spend DESC;

-- name: DeleteOldSpendLogs :exec
DELETE FROM "SpendLogs"
WHERE starttime < $1;

-- name: GetSpendLogDetail :one
SELECT
    sl.*,
    COALESCE(vt.key_alias, '') AS key_alias,
    el.status_code AS error_status_code,
    el.error_type,
    el.error_message,
    el.traceback AS error_traceback
FROM "SpendLogs" sl
LEFT JOIN "VerificationToken" vt ON sl.api_key = vt.token
LEFT JOIN "ErrorLogs" el ON el.request_id = sl.request_id AND el.id = (
    SELECT id FROM "ErrorLogs" WHERE request_id = sl.request_id ORDER BY id DESC LIMIT 1
)
WHERE sl.request_id = $1;

-- name: GetRequestPayload :one
SELECT messages, response
FROM "RequestPayloads"
WHERE request_id = $1;

-- name: CreateRequestPayload :exec
INSERT INTO "RequestPayloads" (request_id, messages, response)
VALUES ($1, $2, $3);

-- name: DeleteOldRequestPayloads :exec
DELETE FROM "RequestPayloads"
WHERE created_at < $1;
