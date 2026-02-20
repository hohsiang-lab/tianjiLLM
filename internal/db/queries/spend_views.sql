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
SELECT DATE(starttime) AS day, team_id, SUM(spend) AS total_spend,
       SUM(total_tokens) AS total_tokens, COUNT(*) AS request_count
FROM "SpendLogs"
WHERE starttime >= $1 AND starttime < $2
GROUP BY DATE(starttime), team_id
ORDER BY day DESC, total_spend DESC;

-- name: GetDailySpendByModel :many
SELECT DATE(starttime) AS day, model, SUM(spend) AS total_spend,
       SUM(total_tokens) AS total_tokens, COUNT(*) AS request_count
FROM "SpendLogs"
WHERE starttime >= $1 AND starttime < $2
GROUP BY DATE(starttime), model
ORDER BY day DESC, total_spend DESC;

-- name: GetDailySpendByKey :many
SELECT DATE(starttime) AS day, api_key, SUM(spend) AS total_spend,
       SUM(total_tokens) AS total_tokens, COUNT(*) AS request_count
FROM "SpendLogs"
WHERE starttime >= $1 AND starttime < $2
GROUP BY DATE(starttime), api_key
ORDER BY day DESC, total_spend DESC;

-- name: GetDailySpendByTag :many
SELECT DATE(starttime) AS day, tag, SUM(spend) AS total_spend, COUNT(*) AS request_count
FROM "SpendLogs", unnest(request_tags) AS tag
WHERE starttime >= $1 AND starttime < $2
GROUP BY DATE(starttime), tag
ORDER BY day DESC, total_spend DESC;

-- name: GetGlobalSpend :one
SELECT COALESCE(SUM(spend), 0) AS total_spend,
       COALESCE(SUM(total_tokens), 0)::BIGINT AS total_tokens,
       COUNT(*) AS request_count
FROM "SpendLogs"
WHERE starttime >= $1 AND starttime < $2;

-- name: GetGlobalSpendByProvider :many
SELECT SPLIT_PART(model, '/', 1) AS provider, SUM(spend) AS total_spend,
       COUNT(*) AS request_count
FROM "SpendLogs"
WHERE starttime >= $1 AND starttime < $2
GROUP BY provider
ORDER BY total_spend DESC;

-- name: GetTeamDailyActivity :many
SELECT DATE(starttime) AS day, model, SUM(spend) AS total_spend,
       SUM(total_tokens) AS total_tokens, COUNT(*) AS request_count
FROM "SpendLogs"
WHERE team_id = $1 AND starttime >= $2 AND starttime < $3
GROUP BY DATE(starttime), model
ORDER BY day DESC, total_spend DESC;

-- name: GetUserDailyActivity :many
SELECT DATE(starttime) AS day, model, SUM(spend) AS total_spend,
       SUM(total_tokens) AS total_tokens, COUNT(*) AS request_count
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
