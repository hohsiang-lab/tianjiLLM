package middleware

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

// fakeQuerier is a test double for verificationTokenQuerier.
type fakeQuerier struct {
	result db.VerificationToken
	err    error
}

func (f *fakeQuerier) GetVerificationToken(_ context.Context, _ string) (db.VerificationToken, error) {
	return f.result, f.err
}

func ptr[T any](v T) *T { return &v }

func TestDBValidator_ValidateToken(t *testing.T) {
	uid := "user-1"
	tid := "team-1"

	tests := []struct {
		name           string
		querier        *fakeQuerier
		wantUserID     *string
		wantTeamID     *string
		wantBlocked    bool
		wantGuardrails []string
		wantErr        error
	}{
		{
			name:    "key not found maps to ErrKeyNotFound",
			querier: &fakeQuerier{err: pgx.ErrNoRows},
			wantErr: ErrKeyNotFound,
		},
		{
			name:    "DB error maps to ErrDBUnavailable",
			querier: &fakeQuerier{err: errors.New("connection refused")},
			wantErr: ErrDBUnavailable,
		},
		{
			name: "blocked key returns blocked=true",
			querier: &fakeQuerier{result: db.VerificationToken{
				UserID:  &uid,
				TeamID:  &tid,
				Blocked: ptr(true),
			}},
			wantUserID:  &uid,
			wantTeamID:  &tid,
			wantBlocked: true,
		},
		{
			name: "valid key returns userID, teamID, and guardrails",
			querier: &fakeQuerier{result: db.VerificationToken{
				UserID:   &uid,
				TeamID:   &tid,
				Blocked:  ptr(false),
				Policies: []string{"guardrail-a", "guardrail-b"},
			}},
			wantUserID:     &uid,
			wantTeamID:     &tid,
			wantBlocked:    false,
			wantGuardrails: []string{"guardrail-a", "guardrail-b"},
		},
		{
			name: "nil Blocked field treated as not blocked",
			querier: &fakeQuerier{result: db.VerificationToken{
				UserID: &uid,
			}},
			wantUserID:  &uid,
			wantBlocked: false,
		},
		{
			name: "nil policies returns nil guardrails",
			querier: &fakeQuerier{result: db.VerificationToken{
				UserID:   &uid,
				Policies: nil,
			}},
			wantUserID:     &uid,
			wantGuardrails: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := &DBValidator{DB: tc.querier}
			userID, teamID, blocked, guardrails, err := v.ValidateToken(context.Background(), "somehash")

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantBlocked, blocked)
			assert.Equal(t, tc.wantUserID, userID)
			assert.Equal(t, tc.wantTeamID, teamID)
			assert.Equal(t, tc.wantGuardrails, guardrails)
		})
	}
}
