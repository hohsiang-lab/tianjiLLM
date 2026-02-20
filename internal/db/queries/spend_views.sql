-- name: CreateSpendArchive :one
INSERT INTO "SpendArchiveTable" (date_from, date_to, storage_type, storage_location, entry_count)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetSpendArchive :one
SELECT * FROM "SpendArchiveTable" WHERE id = $1;

-- name: ListSpendArchives :many
SELECT * FROM "SpendArchiveTable" ORDER BY date_from DESC;

-- name: GetSpendArchiveByDateRange :many
SELECT * FROM "SpendArchiveTable"
WHERE date_from <= $2 AND date_to >= $1
ORDER BY date_from;

-- name: CreateIPWhitelist :one
INSERT INTO "IPWhitelistTable" (ip_address, description, created_by)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetIPWhitelist :one
SELECT * FROM "IPWhitelistTable" WHERE id = $1;

-- name: ListIPWhitelist :many
SELECT * FROM "IPWhitelistTable" ORDER BY created_at DESC;

-- name: DeleteIPWhitelist :exec
DELETE FROM "IPWhitelistTable" WHERE id = $1;

-- name: DeleteIPWhitelistByAddress :exec
DELETE FROM "IPWhitelistTable" WHERE ip_address = $1;

-- Spend aggregation queries

-- name: GetDailySpendByTeam :many
SELECT DATE(starttime) AS day, team_id, COALESCE(SUM(spend), 0)::DOUBLE PRECISION AS total_spend,
       COALESCE(SUM(total_tokens), 0)::BIGINT AS total_tokens, COUNT(*) AS request_count
FROM "SpendLogs"
WHERE starttime >= $1 AND starttime < $2
GROUP BY DATE(starttime), team_id
ORDER BY day DESC, total_spend DESC;

-- name: GetDailySpendByModel :many
SELECT DATE(starttime) AS day, model, COALESCE(SUM(spend), 0)::DOUBLE PRECISION AS total_spend,
       COALESCE(SUM(total_tokens), 0)::BIGINT AS total_tokens, COUNT(*) AS request_count
FROM "SpendLogs"
WHERE starttime >= $1 AND starttime < $2
GROUP BY DATE(starttime), model
ORDER BY day DESC, total_spend DESC;

-- name: GetDailySpendByKey :many
SELECT DATE(starttime) AS day, api_key, COALESCE(SUM(spend), 0)::DOUBLE PRECISION AS total_spend,
       COALESCE(SUM(total_tokens), 0)::BIGINT AS total_tokens, COUNT(*) AS request_count
FROM "SpendLogs"
WHERE starttime >= $1 AND starttime < $2
GROUP BY DATE(starttime), api_key
ORDER BY day DESC, total_spend DESC;

-- name: GetDailySpendByTag :many
SELECT DATE(starttime) AS day, tag, COALESCE(SUM(spend), 0)::DOUBLE PRECISION AS total_spend, COUNT(*) AS request_count
FROM "SpendLogs", unnest(request_tags) AS tag
WHERE starttime >= $1 AND starttime < $2
GROUP BY DATE(starttime), tag
ORDER BY day DESC, total_spend DESC;

-- name: GetGlobalSpend :one
SELECT COALESCE(SUM(spend), 0)::DOUBLE PRECISION AS total_spend,
       COALESCE(SUM(total_tokens), 0)::BIGINT AS total_tokens,
       COUNT(*) AS request_count
FROM "SpendLogs"
WHERE starttime >= $1 AND starttime < $2;

-- name: GetGlobalSpendByProvider :many
SELECT SPLIT_PART(model, '/', 1) AS provider, COALESCE(SUM(spend), 0)::DOUBLE PRECISION AS total_spend,
       COUNT(*) AS request_count
FROM "SpendLogs"
WHERE starttime >= $1 AND starttime < $2
GROUP BY provider
ORDER BY total_spend DESC;

-- name: GetTeamDailyActivity :many
SELECT DATE(starttime) AS day, model, COALESCE(SUM(spend), 0)::DOUBLE PRECISION AS total_spend,
       COALESCE(SUM(total_tokens), 0)::BIGINT AS total_tokens, COUNT(*) AS request_count
FROM "SpendLogs"
WHERE team_id = $1 AND starttime >= $2 AND starttime < $3
GROUP BY DATE(starttime), model
ORDER BY day DESC, total_spend DESC;

-- name: GetUserDailyActivity :many
SELECT DATE(starttime) AS day, model, COALESCE(SUM(spend), 0)::DOUBLE PRECISION AS total_spend,
       COALESCE(SUM(total_tokens), 0)::BIGINT AS total_tokens, COUNT(*) AS request_count
FROM "SpendLogs"
WHERE "user" = $1 AND starttime >= $2 AND starttime < $3
GROUP BY DATE(starttime), model
ORDER BY day DESC, total_spend DESC;

-- name: GetSpendLogsByFilter :many
SELECT request_id, starttime, api_key, model, spend, total_tokens, team_id, "user"
FROM "SpendLogs"
WHERE starttime >= $1 AND starttime < $2
  AND ($3::text IS NULL OR api_key = $3)
  AND ($4::text IS NULL OR team_id = $4)
  AND ($5::text IS NULL OR model = $5)
ORDER BY starttime DESC
LIMIT $6 OFFSET $7;

-- name: GetSpendLogsForArchival :many
SELECT *
FROM "SpendLogs"
WHERE starttime >= $1 AND starttime < $2
ORDER BY starttime
LIMIT $3;

-- name: DeleteSpendLogsByDateRange :exec
DELETE FROM "SpendLogs"
WHERE starttime >= $1 AND starttime < $2;

-- name: CountSpendLogsByDateRange :one
SELECT COUNT(*) FROM "SpendLogs"
WHERE starttime >= $1 AND starttime < $2;

-- Request Logs Dashboard queries

-- name: ListRequestLogs :many
SELECT
    sl.request_id,
    sl.starttime,
    sl.endtime,
    sl.api_key,
    sl.model,
    sl.spend,
    sl.total_tokens,
    sl.prompt_tokens,
    sl.completion_tokens,
    sl.call_type,
    sl.cache_hit,
    sl.team_id,
    sl."user",
    sl.end_user,
    sl.requester_ip_address,
    el.status_code AS error_status_code,
    el.error_type
FROM "SpendLogs" sl
LEFT JOIN "ErrorLogs" el ON sl.request_id = el.request_id
WHERE sl.starttime >= sqlc.arg(start_date)
  AND sl.starttime < sqlc.arg(end_date)
  AND (sqlc.narg(filter_api_key)::text IS NULL OR sl.api_key = sqlc.narg(filter_api_key))
  AND (sqlc.narg(filter_team_id)::text IS NULL OR sl.team_id = sqlc.narg(filter_team_id))
  AND (sqlc.narg(filter_model)::text IS NULL OR sl.model = sqlc.narg(filter_model))
  AND (sqlc.narg(filter_request_id)::text IS NULL OR sl.request_id = sqlc.narg(filter_request_id))
  AND (sqlc.narg(filter_status)::text IS NULL
       OR (sqlc.narg(filter_status) = 'success' AND el.id IS NULL)
       OR (sqlc.narg(filter_status) = 'failed' AND el.id IS NOT NULL))
ORDER BY sl.starttime DESC
LIMIT sqlc.arg(query_limit) OFFSET sqlc.arg(query_offset);

-- name: CountRequestLogs :one
SELECT COUNT(*)
FROM "SpendLogs" sl
LEFT JOIN "ErrorLogs" el ON sl.request_id = el.request_id
WHERE sl.starttime >= sqlc.arg(start_date)
  AND sl.starttime < sqlc.arg(end_date)
  AND (sqlc.narg(filter_api_key)::text IS NULL OR sl.api_key = sqlc.narg(filter_api_key))
  AND (sqlc.narg(filter_team_id)::text IS NULL OR sl.team_id = sqlc.narg(filter_team_id))
  AND (sqlc.narg(filter_model)::text IS NULL OR sl.model = sqlc.narg(filter_model))
  AND (sqlc.narg(filter_request_id)::text IS NULL OR sl.request_id = sqlc.narg(filter_request_id))
  AND (sqlc.narg(filter_status)::text IS NULL
       OR (sqlc.narg(filter_status) = 'success' AND el.id IS NULL)
       OR (sqlc.narg(filter_status) = 'failed' AND el.id IS NOT NULL));

-- Usage Dashboard queries

-- name: GetUsageMetrics :one
SELECT
    COUNT(*) AS total_requests,
    COALESCE(SUM(total_tokens), 0)::BIGINT AS total_tokens,
    COALESCE(SUM(spend), 0)::DOUBLE PRECISION AS total_spend
FROM "SpendLogs"
WHERE starttime >= sqlc.arg(start_date) AND starttime < sqlc.arg(end_date);

-- name: GetFailedRequestCount :one
SELECT COUNT(DISTINCT el.request_id) AS failed_requests
FROM "ErrorLogs" el
WHERE el.request_id IN (
    SELECT request_id FROM "SpendLogs"
    WHERE starttime >= sqlc.arg(start_date) AND starttime < sqlc.arg(end_date)
);

-- name: GetTopKeysBySpend :many
SELECT
    sl.api_key,
    COALESCE(vt.key_alias, '') AS key_alias,
    COALESCE(SUM(sl.spend), 0)::DOUBLE PRECISION AS total_spend,
    COUNT(*) AS request_count
FROM "SpendLogs" sl
LEFT JOIN "VerificationToken" vt ON sl.api_key = vt.token
WHERE sl.starttime >= sqlc.arg(start_date) AND sl.starttime < sqlc.arg(end_date)
GROUP BY sl.api_key, vt.key_alias
ORDER BY total_spend DESC
LIMIT sqlc.arg(query_limit);

-- name: GetTopModelsBySpend :many
SELECT
    model,
    COALESCE(SUM(spend), 0)::DOUBLE PRECISION AS total_spend,
    COALESCE(SUM(total_tokens), 0)::BIGINT AS total_tokens,
    COUNT(*) AS request_count
FROM "SpendLogs"
WHERE starttime >= sqlc.arg(start_date) AND starttime < sqlc.arg(end_date)
GROUP BY model
ORDER BY total_spend DESC
LIMIT sqlc.arg(query_limit);

-- name: GetDailySpendByCallType :many
SELECT
    DATE(starttime) AS day,
    call_type,
    COUNT(*) AS request_count,
    SUM(total_tokens)::BIGINT AS total_tokens
FROM "SpendLogs"
WHERE starttime >= sqlc.arg(start_date) AND starttime < sqlc.arg(end_date)
GROUP BY DATE(starttime), call_type
ORDER BY day DESC, request_count DESC;

-- name: GetDailyActivityByKey :many
SELECT
    DATE(sl.starttime) AS day,
    sl.api_key,
    COUNT(*) AS request_count,
    COALESCE(SUM(sl.spend), 0)::DOUBLE PRECISION AS total_spend,
    COALESCE(SUM(sl.total_tokens), 0)::BIGINT AS total_tokens
FROM "SpendLogs" sl
WHERE sl.starttime >= sqlc.arg(start_date) AND sl.starttime < sqlc.arg(end_date)
  AND sl.api_key IN (
      SELECT s2.api_key FROM "SpendLogs" s2
      WHERE s2.starttime >= sqlc.arg(start_date) AND s2.starttime < sqlc.arg(end_date)
      GROUP BY s2.api_key ORDER BY SUM(s2.spend) DESC
      LIMIT sqlc.arg(key_limit)
  )
GROUP BY DATE(sl.starttime), sl.api_key
ORDER BY day DESC, total_spend DESC;
