package callback

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAIQuotaStore_MergesPartialUpdatesAndNormalizesExpiredGate(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	store := NewInMemoryRateLimitStore()

	store.SetOpenAIQuotaState("cred-a", OpenAIQuotaState{
		SubjectID: "cred-a",
		Requests: OpenAIQuotaDimension{
			Limit:            100,
			LimitKnown:       true,
			Remaining:        0,
			RemainingKnown:   true,
			ResetAt:          now.Add(time.Minute),
			ResetKnown:       true,
			Utilization:      1,
			UtilizationKnown: true,
		},
		UpdatedAt: now,
	})
	store.SetOpenAIQuotaState("cred-a", OpenAIQuotaState{
		SubjectID: "cred-a",
		Tokens: OpenAIQuotaDimension{
			Limit:            200000,
			LimitKnown:       true,
			Remaining:        150000,
			RemainingKnown:   true,
			ResetAt:          now.Add(6 * time.Minute),
			ResetKnown:       true,
			Utilization:      0.25,
			UtilizationKnown: true,
		},
		UpdatedAt: now.Add(time.Second),
	})

	state, ok := store.GetOpenAIQuotaState("cred-a", now)
	require.True(t, ok)
	assert.Equal(t, 100, state.Requests.Limit)
	assert.Equal(t, 0, state.Requests.Remaining)
	assert.Equal(t, 200000, state.Tokens.Limit)
	assert.Equal(t, 150000, state.Tokens.Remaining)
	assert.True(t, state.Gated(now))

	state, ok = store.GetOpenAIQuotaState("cred-a", now.Add(2*time.Minute))
	require.True(t, ok)
	assert.False(t, state.Gated(now.Add(2*time.Minute)))
	assert.False(t, state.Requests.ResetKnown)
	assert.True(t, state.Tokens.ResetKnown)
}

func TestOpenAIQuotaStore_KeysBySubjectIDNotTokenMaterial(t *testing.T) {
	store := NewInMemoryRateLimitStore()
	store.SetOpenAIQuotaState("cred-a", OpenAIQuotaState{
		SubjectID: "cred-a",
		UpdatedAt: time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC),
	})

	_, ok := store.GetOpenAIQuotaState("sk-real-token-must-not-be-key", time.Now())
	assert.False(t, ok)
}
