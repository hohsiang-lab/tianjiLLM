-- name: CreateTag :one
INSERT INTO "TagTable" (name, description)
VALUES ($1, $2)
RETURNING *;

-- name: GetTag :one
SELECT * FROM "TagTable" WHERE id = $1;

-- name: GetTagByName :one
SELECT * FROM "TagTable" WHERE name = $1;

-- name: ListTags :many
SELECT * FROM "TagTable" ORDER BY name ASC;

-- name: UpdateTag :one
UPDATE "TagTable"
SET name = COALESCE($2, name),
    description = COALESCE($3, description)
WHERE id = $1
RETURNING *;

-- name: DeleteTag :exec
DELETE FROM "TagTable" WHERE id = $1;
