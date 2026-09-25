-- name: GetTeam :one
SELECT * FROM "TeamTable" WHERE team_id = $1;

-- name: ListTeams :many
SELECT * FROM "TeamTable" ORDER BY created_at DESC;

-- name: ListTeamAliases :many
SELECT team_id, team_alias FROM "TeamTable"
WHERE team_id = ANY(sqlc.arg(team_ids)::text[]);
