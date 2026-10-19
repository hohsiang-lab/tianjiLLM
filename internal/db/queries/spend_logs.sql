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
WITH target_spend AS (
    SELECT *
    FROM "SpendLogs" AS s
    WHERE s.request_id = $1
), latest_error AS (
    SELECT *
    FROM "ErrorLogs" AS e
    WHERE e.request_id = $1
    ORDER BY e.id DESC
    LIMIT 1
)
SELECT
    COALESCE(sl.request_id, el.request_id) AS request_id,
    COALESCE(sl.call_type, '') AS call_type,
    COALESCE(sl.api_key, el.api_key_hash, '') AS api_key,
    sl.spend,
    sl.total_tokens,
    sl.prompt_tokens,
    sl.completion_tokens,
    COALESCE(sl.starttime, el.created_at) AS starttime,
    sl.endtime,
    sl.completionstartime,
    COALESCE(sl.model, el.model, '') AS model,
    COALESCE(sl.model_id, '') AS model_id,
    COALESCE(sl.model_group, '') AS model_group,
    COALESCE(sl.api_base, '') AS api_base,
    COALESCE(sl."user", '') AS "user",
    COALESCE(sl.metadata, '{}'::jsonb) AS metadata,
    COALESCE(sl.cache_hit, '') AS cache_hit,
    COALESCE(sl.cache_key, '') AS cache_key,
    COALESCE(sl.request_tags, ARRAY[]::text[]) AS request_tags,
    COALESCE(sl.team_id, el.team_id) AS team_id,
    COALESCE(sl.end_user, el.end_user, '') AS end_user,
    sl.requester_ip_address,
    COALESCE(sl.organization_id, el.organization_id, '') AS organization_id,
    COALESCE(sl.provider, el.provider, '') AS provider,
    sl.request_duration_ms,
    COALESCE(sl.upstream_token_key, el.upstream_token_key, '') AS upstream_token_key,
    COALESCE(sl.reasoning_effort, el.reasoning_effort, '') AS reasoning_effort,
    COALESCE(vt.key_alias, '') AS key_alias,
    el.status_code AS error_status_code,
    el.error_type,
    el.error_message,
    el.traceback AS error_traceback,
    CASE WHEN sl.request_id IS NULL THEN 0 ELSE 1 END AS has_spend_log,
    el.id AS error_log_id
FROM target_spend sl
FULL OUTER JOIN latest_error el ON el.request_id = sl.request_id
LEFT JOIN "VerificationToken" vt ON COALESCE(sl.api_key, el.api_key_hash) = vt.token;

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
