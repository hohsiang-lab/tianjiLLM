-- name: CreateSkill :one
INSERT INTO "SkillsTable" (
    display_title, description, instructions, source,
    latest_version, file_content, file_name, file_type,
    metadata, created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetSkill :one
SELECT * FROM "SkillsTable"
WHERE skill_id = $1;

-- name: ListSkills :many
SELECT * FROM "SkillsTable"
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: DeleteSkill :exec
DELETE FROM "SkillsTable"
WHERE skill_id = $1;
