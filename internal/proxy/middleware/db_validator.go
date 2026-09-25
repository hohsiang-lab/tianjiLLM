package middleware

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

// Sentinel errors for virtual key validation failures.
var (
	ErrKeyNotFound   = errors.New("api key not found")
	ErrDBUnavailable = errors.New("database unavailable")
)

// limitQuerier is a narrow interface satisfied by *db.Queries.
// It exists to make DBValidator unit-testable without a real DB connection.
type limitQuerier interface {
	GetVerificationToken(ctx context.Context, token string) (db.VerificationToken, error)
	GetTeam(ctx context.Context, teamID string) (db.TeamTable, error)
	GetOrganization(ctx context.Context, organizationID string) (db.OrganizationTable, error)
	GetBudget(ctx context.Context, budgetID string) (db.BudgetTable, error)
}

// DBValidator bridges *db.Queries to TokenValidator.
type DBValidator struct {
	DB limitQuerier
}

// ValidateToken looks up a virtual key by its SHA256 hash and resolves inherited limits.
// May issue up to 4 DB queries (key + team + org + budget) depending on which limits are set.
// RPM/TPM/MaxBudget are resolved via the inheritance chain: key → team → org.
// MaxParallelRequests comes from the key's budget record. Models and Expires are key-level only.
func (d *DBValidator) ValidateToken(ctx context.Context, tokenHash string) (*TokenInfo, error) {
	vt, err := d.DB.GetVerificationToken(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrKeyNotFound
		}
		return nil, ErrDBUnavailable
	}

	info := &TokenInfo{
		UserID:     vt.UserID,
		TeamID:     vt.TeamID,
		OrgID:      vt.OrganizationID,
		Blocked:    vt.Blocked != nil && *vt.Blocked,
		Guardrails: vt.Policies,
		RpmLimit:   vt.RpmLimit,
		TpmLimit:   vt.TpmLimit,
		MaxBudget:  vt.MaxBudget,
		Spend:      vt.Spend,
		Models:     vt.Models,
	}

	// Convert pgtype.Timestamptz → *time.Time
	if vt.Expires.Valid {
		t := vt.Expires.Time
		info.Expires = &t
	}

	// Inheritance chain: key → team → org (only for nil limits).
	d.resolveInheritedLimits(ctx, info, &vt)

	// MaxParallelRequests comes from BudgetTable (via BudgetID).
	if vt.BudgetID != nil {
		if budget, err := d.DB.GetBudget(ctx, *vt.BudgetID); err == nil {
			info.MaxParallelRequests = budget.MaxParallelRequests
		} else {
			log.Printf("warn: failed to get budget %s for limit resolution: %v", *vt.BudgetID, err)
		}
	}

	return info, nil
}

// resolveInheritedLimits fills nil limit fields from team, then org.
func (d *DBValidator) resolveInheritedLimits(ctx context.Context, info *TokenInfo, vt *db.VerificationToken) {
	needsTeam := info.RpmLimit == nil || info.TpmLimit == nil || info.MaxBudget == nil

	if needsTeam && vt.TeamID != nil {
		team, err := d.DB.GetTeam(ctx, *vt.TeamID)
		if err != nil {
			log.Printf("warn: failed to get team %s for limit resolution: %v", *vt.TeamID, err)
		} else {
			if info.RpmLimit == nil {
				info.RpmLimit = team.RpmLimit
			}
			if info.TpmLimit == nil {
				info.TpmLimit = team.TpmLimit
			}
			if info.MaxBudget == nil {
				info.MaxBudget = team.MaxBudget
			}
		}
	}

	needsOrg := info.RpmLimit == nil || info.TpmLimit == nil || info.MaxBudget == nil

	if needsOrg && vt.OrganizationID != nil {
		org, err := d.DB.GetOrganization(ctx, *vt.OrganizationID)
		if err != nil {
			log.Printf("warn: failed to get org %s for limit resolution: %v", *vt.OrganizationID, err)
		} else {
			if info.RpmLimit == nil {
				info.RpmLimit = org.RpmLimit
			}
			if info.TpmLimit == nil {
				info.TpmLimit = org.TpmLimit
			}
			if info.MaxBudget == nil {
				info.MaxBudget = org.MaxBudget
			}
		}
	}
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
// ctx is always context.Background() at the call site; a 5-second timeout is applied
// to bound DB write latency and prevent goroutine leaks under slow DB conditions.
func (l *DBAuthErrorLogger) LogAuthError(_ context.Context, requestID string, apiKeyHash string, statusCode int, errorMsg string) {
	insertCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := l.DB.InsertErrorLog(insertCtx, db.InsertErrorLogParams{
		RequestID:    requestID,
		ApiKeyHash:   apiKeyHash,
		StatusCode:   int32(statusCode),
		ErrorType:    "authentication_error",
		ErrorMessage: errorMsg,
	}); err != nil {
		log.Printf("error: failed to write auth error log for request_id=%s: %v", requestID, err)
	}
}

// Compile-time interface satisfaction checks.
var _ TokenValidator = (*DBValidator)(nil)
var _ AuthErrorLogger = (*DBAuthErrorLogger)(nil)
var _ errorLogInserter = (*db.Queries)(nil)

// Verify that *db.Queries satisfies the narrow querier interface.
var _ limitQuerier = (*db.Queries)(nil)
