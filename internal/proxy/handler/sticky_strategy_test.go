package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStickyStrategySelect_ReusesCurrentCandidate(t *testing.T) {
	var store stickyStrategyStore
	candidates := []stickyStrategyCandidate{
		{ID: "cred-a", Metadata: "same"},
		{ID: "cred-b", Metadata: "same"},
	}
	policy := stickyStrategyPolicy{
		Select: func([]stickyStrategyCandidate) stickyStrategyCandidate {
			return candidates[0]
		},
		CanReuse: func(entry stickyStrategyEntry, candidate stickyStrategyCandidate) bool {
			return entry.Metadata == candidate.Metadata
		},
	}

	first, _, ok := store.Select("track", candidates, policy)
	require.True(t, ok)
	second, _, ok := store.Select("track", candidates, policy)
	require.True(t, ok)

	assert.Equal(t, "cred-a", first.ID)
	assert.Equal(t, "cred-a", second.ID)
}

func TestStickyStrategySelect_RefreshesMetadataOnReuse(t *testing.T) {
	var store stickyStrategyStore
	store.SeedIfAbsent("track", stickyStrategyEntry{SelectedID: "cred-a"})
	candidates := []stickyStrategyCandidate{
		{ID: "cred-a", Metadata: "current"},
		{ID: "cred-b", Metadata: "other"},
	}

	selected, entry, ok := store.Select("track", candidates, stickyStrategyPolicy{
		CanReuse: func(stickyStrategyEntry, stickyStrategyCandidate) bool { return true },
	})
	require.True(t, ok)

	assert.Equal(t, "cred-a", selected.ID)
	assert.Equal(t, "cred-a", entry.SelectedID)
	assert.Equal(t, "current", entry.Metadata)
}

func TestStickyStrategySelect_ReselectsWhenCurrentUnavailable(t *testing.T) {
	var store stickyStrategyStore
	firstCandidates := []stickyStrategyCandidate{{ID: "cred-a"}, {ID: "cred-b"}}
	secondCandidates := []stickyStrategyCandidate{{ID: "cred-b"}}

	first, _, ok := store.Select("track", firstCandidates, stickyStrategyPolicy{})
	require.True(t, ok)
	second, _, ok := store.Select("track", secondCandidates, stickyStrategyPolicy{})
	require.True(t, ok)

	assert.Equal(t, "cred-a", first.ID)
	assert.Equal(t, "cred-b", second.ID)
}

func TestStickyStrategySelect_ReselectsWhenPolicyRejectsReuse(t *testing.T) {
	var store stickyStrategyStore
	candidates := []stickyStrategyCandidate{
		{ID: "cred-a", Metadata: "old"},
		{ID: "cred-b", Metadata: "new"},
	}
	policy := stickyStrategyPolicy{
		CanReuse: func(stickyStrategyEntry, stickyStrategyCandidate) bool { return false },
		Select: func(candidates []stickyStrategyCandidate) stickyStrategyCandidate {
			return candidates[1]
		},
	}

	first, _, ok := store.Select("track", candidates[:1], stickyStrategyPolicy{})
	require.True(t, ok)
	second, entry, ok := store.Select("track", candidates, policy)
	require.True(t, ok)

	assert.Equal(t, "cred-a", first.ID)
	assert.Equal(t, "cred-b", second.ID)
	assert.Equal(t, "cred-b", entry.SelectedID)
	assert.Equal(t, "new", entry.Metadata)
}

func TestSelectLowestStickyScore_FallsBackDeterministically(t *testing.T) {
	candidates := []stickyStrategyCandidate{{ID: "cred-a"}, {ID: "cred-b"}}

	selected := selectLowestStickyScore(candidates, func(stickyStrategyCandidate) (int64, bool) {
		return 0, false
	})

	assert.Equal(t, "cred-a", selected.ID)
}

func TestSelectLowestStickyScore_PicksLowestKnownScore(t *testing.T) {
	candidates := []stickyStrategyCandidate{{ID: "cred-a"}, {ID: "cred-b"}, {ID: "cred-c"}}
	scores := map[string]int64{"cred-a": 50, "cred-b": 10, "cred-c": 10}

	selected := selectLowestStickyScore(candidates, func(candidate stickyStrategyCandidate) (int64, bool) {
		score, ok := scores[candidate.ID]
		return score, ok
	})

	assert.Equal(t, "cred-b", selected.ID)
}
