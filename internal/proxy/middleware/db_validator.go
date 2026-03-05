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
// Returns all key metadata needed for auth and guardrail enforcement.
func (d *DBValidator) ValidateToken(ctx context.Context, tokenHash string) (*TokenInfo, error) {
	vt, err := d.DB.GetVerificationToken(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrKeyNotFound
		}
		return nil, ErrDBUnavailable
	}
	return &TokenInfo{
		UserID:     vt.UserID,
		TeamID:     vt.TeamID,
		Blocked:    vt.Blocked != nil && *vt.Blocked,
		Guardrails: vt.Policies,
	}, nil
}

// errorLogInserter is a narrow interface satisfied by *db.Queries.
type errorLogInserter interface {
	InsertErrorLog(ctx context.Context, arg db.InsertErrorLogParams) error
}

// DBAuthErrorLogger implements AuthErrorLogger by writing to the ErrorLogs table.
type DBAuthErrorLogger struct {
	DB errorLogInserter
}

// LogAuthError records an authentication failure to ErrorLogs (fire-and-forget).
func (l *DBAuthErrorLogger) LogAuthError(_ context.Context, requestID string, apiKeyHash string, statusCode int, errorMsg string) {
	_ = l.DB.InsertErrorLog(context.Background(), db.InsertErrorLogParams{
		RequestID:    requestID,
		ApiKeyHash:   apiKeyHash,
		StatusCode:   int32(statusCode),
		ErrorType:    "authentication_error",
		ErrorMessage: errorMsg,
	})
}

// Compile-time interface satisfaction checks.
var _ TokenValidator = (*DBValidator)(nil)
var _ AuthErrorLogger = (*DBAuthErrorLogger)(nil)
var _ errorLogInserter = (*db.Queries)(nil)

// Verify that *db.Queries satisfies the narrow querier interface.
var _ verificationTokenQuerier = (*db.Queries)(nil)
