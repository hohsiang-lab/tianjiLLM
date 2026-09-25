-- name: InsertHealthCheck :exec
INSERT INTO "HealthCheckTable" (model_name, status, response_time_ms, error_message)
VALUES ($1, $2, $3, $4);

-- name: ListHealthChecks :many
SELECT * FROM "HealthCheckTable"
WHERE ($1::text = '' OR model_name = $1)
ORDER BY checked_at DESC
LIMIT $2 OFFSET $3;
