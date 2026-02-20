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

-- name: GetProxyModelByName :one
SELECT * FROM "ProxyModelTable"
WHERE model_name = $1;

-- name: ListProxyModelsPage :many
SELECT * FROM "ProxyModelTable"
WHERE
    (sqlc.narg('search')::text IS NULL
     OR model_name ILIKE '%' || sqlc.narg('search')::text || '%'
     OR model_id ILIKE '%' || sqlc.narg('search')::text || '%'
     OR tianji_params->>'model' ILIKE '%' || sqlc.narg('search')::text || '%')
ORDER BY created_at DESC
LIMIT sqlc.arg('page_limit')
OFFSET sqlc.arg('page_offset');

-- name: CountProxyModels :one
SELECT COUNT(*) FROM "ProxyModelTable"
WHERE
    (sqlc.narg('search')::text IS NULL
     OR model_name ILIKE '%' || sqlc.narg('search')::text || '%'
     OR model_id ILIKE '%' || sqlc.narg('search')::text || '%'
     OR tianji_params->>'model' ILIKE '%' || sqlc.narg('search')::text || '%');
