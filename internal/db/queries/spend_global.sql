-- name: GetGlobalActivity :many
SELECT
    DATE(starttime) as date,
    COUNT(*) as request_count,
    COALESCE(SUM(prompt_tokens), 0)::BIGINT as total_prompt_tokens,
    COALESCE(SUM(completion_tokens), 0)::BIGINT as total_completion_tokens,
    COALESCE(SUM(total_tokens), 0)::BIGINT as total_tokens,
    COALESCE(SUM(spend), 0)::DOUBLE PRECISION as total_spend
FROM "SpendLogs"
WHERE starttime >= $1 AND starttime < $2
GROUP BY DATE(starttime)
ORDER BY date DESC;

-- name: GetGlobalActivityByModel :many
SELECT
    model,
    DATE(starttime) as date,
    COUNT(*) as request_count,
    COALESCE(SUM(prompt_tokens), 0)::BIGINT as total_prompt_tokens,
    COALESCE(SUM(completion_tokens), 0)::BIGINT as total_completion_tokens,
    COALESCE(SUM(spend), 0)::DOUBLE PRECISION as total_spend
FROM "SpendLogs"
WHERE starttime >= $1 AND starttime < $2
GROUP BY model, DATE(starttime)
ORDER BY date DESC, model;

-- name: GetGlobalSpendReport :many
SELECT
    COALESCE(team_id, '') as group_key,
    DATE(starttime) as date,
    COALESCE(SUM(spend), 0)::DOUBLE PRECISION as total_spend,
    COUNT(*) as request_count
FROM "SpendLogs"
WHERE starttime >= $1 AND starttime < $2
GROUP BY group_key, DATE(starttime)
ORDER BY date DESC;

-- name: GetGlobalSpendReportByCustomer :many
SELECT
    COALESCE(end_user, '') as group_key,
    DATE(starttime) as date,
    COALESCE(SUM(spend), 0)::DOUBLE PRECISION as total_spend,
    COUNT(*) as request_count
FROM "SpendLogs"
WHERE starttime >= $1 AND starttime < $2
GROUP BY group_key, DATE(starttime)
ORDER BY date DESC;

-- name: GetGlobalSpendReportByKey :many
SELECT
    api_key as group_key,
    DATE(starttime) as date,
    COALESCE(SUM(spend), 0)::DOUBLE PRECISION as total_spend,
    COUNT(*) as request_count
FROM "SpendLogs"
WHERE starttime >= $1 AND starttime < $2
GROUP BY group_key, DATE(starttime)
ORDER BY date DESC;

-- name: ResetAllKeySpend :exec
UPDATE "VerificationToken"
SET spend = 0, updated_at = NOW();

-- name: ResetAllTeamSpend :exec
UPDATE "TeamTable"
SET spend = 0, updated_at = NOW();

-- name: ResetKeySpendByToken :exec
UPDATE "VerificationToken"
SET spend = $2, updated_at = NOW()
WHERE token = $1;

-- name: GetCacheHitStats :many
SELECT
    DATE(starttime) as date,
    COUNT(CASE WHEN cache_hit = 'True' THEN 1 END) as cache_hits,
    COUNT(CASE WHEN cache_hit != 'True' OR cache_hit = '' THEN 1 END) as cache_misses,
    COUNT(*) as total_requests
FROM "SpendLogs"
WHERE starttime >= $1 AND starttime < $2
GROUP BY DATE(starttime)
ORDER BY date DESC;

-- name: ListDistinctKeyAliases :many
SELECT DISTINCT key_alias
FROM "VerificationToken"
WHERE key_alias IS NOT NULL AND key_alias != ''
ORDER BY key_alias;

-- name: GetVerificationTokenBatch :many
SELECT * FROM "VerificationToken"
WHERE token = ANY($1::text[]);
