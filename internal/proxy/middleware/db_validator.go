package middleware

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

// Sentinel errors for virtual key validation failures.
var (
	ErrKeyNotFound   = errors.New("api key not found")
	ErrDBUnavailable = errors.New("database unavailable")
)

// verificationTokenQuerier is a narrow interface satisfied by *db.Queries.
// It exists to make DBValidator unit-testable without a real DB connection.
type verificationTokenQuerier interface {
	GetVerificationToken(ctx context.Context, token string) (db.VerificationToken, error)
}

// DBValidator bridges *db.Queries to TokenValidator.
type DBValidator struct {
	DB verificationTokenQuerier
}

// ValidateToken looks up a virtual key by its SHA256 hash in a single DB call.
// Returns user/team IDs, blocked status, guardrail policy names, and any error.
func (d *DBValidator) ValidateToken(ctx context.Context, tokenHash string) (userID, teamID *string, blocked bool, guardrails []string, err error) {
	vt, err := d.DB.GetVerificationToken(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, false, nil, ErrKeyNotFound
		}
		return nil, nil, false, nil, ErrDBUnavailable
	}
	blocked = vt.Blocked != nil && *vt.Blocked
	return vt.UserID, vt.TeamID, blocked, vt.Policies, nil
}

// Compile-time interface satisfaction checks.
var _ TokenValidator = (*DBValidator)(nil)

// Verify that *db.Queries satisfies the narrow querier interface.
var _ verificationTokenQuerier = (*db.Queries)(nil)
