-- name: GetBudget :one
SELECT * FROM "BudgetTable" WHERE budget_id = $1;

-- name: ListBudgets :many
SELECT * FROM "BudgetTable" ORDER BY created_at DESC;

-- name: CreateBudget :one
INSERT INTO "BudgetTable" (budget_id, max_budget, soft_budget, max_parallel_requests, tpm_limit, rpm_limit, budget_duration, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateBudget :one
UPDATE "BudgetTable"
SET max_budget = COALESCE($2, max_budget),
    soft_budget = COALESCE($3, soft_budget),
    tpm_limit = COALESCE($4, tpm_limit),
    rpm_limit = COALESCE($5, rpm_limit),
    updated_at = NOW(),
    updated_by = $6
WHERE budget_id = $1
RETURNING *;

-- name: DeleteBudget :exec
DELETE FROM "BudgetTable" WHERE budget_id = $1;
