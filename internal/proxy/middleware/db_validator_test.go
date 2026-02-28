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
		name    string
		querier *fakeQuerier
		want    *TokenInfo
		wantErr error
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
			want: &TokenInfo{UserID: &uid, TeamID: &tid, Blocked: true},
		},
		{
			name: "valid key returns userID, teamID, and guardrails",
			querier: &fakeQuerier{result: db.VerificationToken{
				UserID:   &uid,
				TeamID:   &tid,
				Blocked:  ptr(false),
				Policies: []string{"guardrail-a", "guardrail-b"},
			}},
			want: &TokenInfo{UserID: &uid, TeamID: &tid, Blocked: false, Guardrails: []string{"guardrail-a", "guardrail-b"}},
		},
		{
			name: "nil Blocked field treated as not blocked",
			querier: &fakeQuerier{result: db.VerificationToken{
				UserID: &uid,
			}},
			want: &TokenInfo{UserID: &uid},
		},
		{
			name: "nil policies returns nil guardrails",
			querier: &fakeQuerier{result: db.VerificationToken{
				UserID:   &uid,
				Policies: nil,
			}},
			want: &TokenInfo{UserID: &uid},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := &DBValidator{DB: tc.querier}
			info, err := v.ValidateToken(context.Background(), "somehash")

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				assert.Nil(t, info)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, info)
			assert.Equal(t, tc.want.Blocked, info.Blocked)
			assert.Equal(t, tc.want.UserID, info.UserID)
			assert.Equal(t, tc.want.TeamID, info.TeamID)
			assert.Equal(t, tc.want.Guardrails, info.Guardrails)
		})
	}
}
