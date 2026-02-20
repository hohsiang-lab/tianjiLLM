-- name: GetProxyModel :one
SELECT * FROM "ProxyModelTable"
WHERE model_id = $1;

-- name: ListProxyModels :many
SELECT * FROM "ProxyModelTable"
ORDER BY created_at DESC;

-- name: CreateProxyModel :one
INSERT INTO "ProxyModelTable" (
    model_id, model_name, tianji_params, model_info, created_by
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING *;

-- name: UpdateProxyModel :one
UPDATE "ProxyModelTable"
SET
    model_name = COALESCE($2, model_name),
    tianji_params = COALESCE($3, tianji_params),
    model_info = COALESCE($4, model_info),
    updated_at = NOW(),
    updated_by = $5
WHERE model_id = $1
RETURNING *;

-- name: DeleteProxyModel :exec
DELETE FROM "ProxyModelTable"
WHERE model_id = $1;
