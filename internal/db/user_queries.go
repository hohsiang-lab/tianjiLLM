package db

import (
	"context"
)

// --- Additional user queries (hand-written, not sqlc-generated) ---

const listUsersPaginated = `-- name: ListUsersPaginated :many
SELECT user_id, user_alias, user_email, user_role, teams, max_budget, spend, models, metadata, tpm_limit, rpm_limit, budget_duration, budget_reset_at, budget_id, created_at, created_by, updated_at, updated_by
FROM "UserTable"
WHERE ($1::text = '' OR user_alias ILIKE '%' || $1 || '%' OR user_email ILIKE '%' || $1 || '%')
  AND ($2::text = '' OR user_role = $2)
  AND ($3::text = '' OR COALESCE(metadata->>'status', 'active') = $3)
  AND COALESCE(metadata->>'status', 'active') != 'deleted'
ORDER BY created_at DESC
LIMIT $4 OFFSET $5
`

type ListUsersPaginatedParams struct {
	Search       string `json:"search"`
	RoleFilter   string `json:"role_filter"`
	StatusFilter string `json:"status_filter"`
	Limit        int32  `json:"limit"`
	Offset       int32  `json:"offset"`
}

func (q *Queries) ListUsersPaginated(ctx context.Context, arg ListUsersPaginatedParams) ([]UserTable, error) {
	rows, err := q.db.Query(ctx, listUsersPaginated,
		arg.Search,
		arg.RoleFilter,
		arg.StatusFilter,
		arg.Limit,
		arg.Offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []UserTable
	for rows.Next() {
		var i UserTable
		if err := rows.Scan(
			&i.UserID,
			&i.UserAlias,
			&i.UserEmail,
			&i.UserRole,
			&i.Teams,
			&i.MaxBudget,
			&i.Spend,
			&i.Models,
			&i.Metadata,
			&i.TpmLimit,
			&i.RpmLimit,
			&i.BudgetDuration,
			&i.BudgetResetAt,
			&i.BudgetID,
			&i.CreatedAt,
			&i.CreatedBy,
			&i.UpdatedAt,
			&i.UpdatedBy,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const countUsers = `-- name: CountUsers :one
SELECT COUNT(*) FROM "UserTable"
WHERE ($1::text = '' OR user_alias ILIKE '%' || $1 || '%' OR user_email ILIKE '%' || $1 || '%')
  AND ($2::text = '' OR user_role = $2)
  AND ($3::text = '' OR COALESCE(metadata->>'status', 'active') = $3)
  AND COALESCE(metadata->>'status', 'active') != 'deleted'
`

type CountUsersParams struct {
	Search       string `json:"search"`
	RoleFilter   string `json:"role_filter"`
	StatusFilter string `json:"status_filter"`
}

func (q *Queries) CountUsers(ctx context.Context, arg CountUsersParams) (int64, error) {
	row := q.db.QueryRow(ctx, countUsers,
		arg.Search,
		arg.RoleFilter,
		arg.StatusFilter,
	)
	var count int64
	err := row.Scan(&count)
	return count, err
}

const countUsersByRole = `-- name: CountUsersByRole :one
SELECT COUNT(*) FROM "UserTable"
WHERE user_role = $1
  AND COALESCE(metadata->>'status', 'active') != 'deleted'
`

func (q *Queries) CountUsersByRole(ctx context.Context, role string) (int64, error) {
	row := q.db.QueryRow(ctx, countUsersByRole, role)
	var count int64
	err := row.Scan(&count)
	return count, err
}

const softDeleteUser = `-- name: SoftDeleteUser :exec
UPDATE "UserTable"
SET metadata = jsonb_set(COALESCE(metadata, '{}'::jsonb), '{status}', '"deleted"'), updated_at = NOW(), updated_by = $2
WHERE user_id = $1
`

type SoftDeleteUserParams struct {
	UserID    string `json:"user_id"`
	UpdatedBy string `json:"updated_by"`
}

func (q *Queries) SoftDeleteUser(ctx context.Context, arg SoftDeleteUserParams) error {
	_, err := q.db.Exec(ctx, softDeleteUser, arg.UserID, arg.UpdatedBy)
	return err
}

const setUserStatus = `-- name: SetUserStatus :exec
UPDATE "UserTable"
SET metadata = jsonb_set(COALESCE(metadata, '{}'::jsonb), '{status}', to_jsonb($2::text)), updated_at = NOW(), updated_by = $3
WHERE user_id = $1
`

type SetUserStatusParams struct {
	UserID    string `json:"user_id"`
	Status    string `json:"status"`
	UpdatedBy string `json:"updated_by"`
}

func (q *Queries) SetUserStatus(ctx context.Context, arg SetUserStatusParams) error {
	_, err := q.db.Exec(ctx, setUserStatus, arg.UserID, arg.Status, arg.UpdatedBy)
	return err
}
