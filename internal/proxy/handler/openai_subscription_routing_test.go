package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/chatgptcodex"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveOpenAISubscriptionCandidates_IncludesAllConfiguredIDs(t *testing.T) {
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})

	candidates, failures := h.resolveOpenAISubscriptionCandidates(context.Background(), config.TianjiParams{
		Model:                           "openai/gpt-4o",
		OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"},
	}, openAISubscriptionTransportChatGPTCodexBackend)

	require.Empty(t, failures)
	require.Len(t, candidates, 2)
	assert.Equal(t, "cred-a", candidates[0].CredentialID)
	assert.Equal(t, "access-a", candidates[0].BearerToken)
	assert.Equal(t, "cred-b", candidates[1].CredentialID)
	assert.Equal(t, "access-b", candidates[1].BearerToken)
}

func TestResolveOpenAISubscriptionCandidates_DisabledExcluded(t *testing.T) {
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a", Status: "disabled"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})

	candidates, failures := h.resolveOpenAISubscriptionCandidates(context.Background(), config.TianjiParams{
		Model:                           "openai/gpt-4o",
		OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"},
	}, openAISubscriptionTransportChatGPTCodexBackend)

	require.Len(t, candidates, 1)
	assert.Equal(t, "cred-b", candidates[0].CredentialID)
	require.Len(t, failures, 1)
	assert.Equal(t, OpenAISubscriptionCredentialDisabled, failures[0].Code)
}

func TestResolveOpenAISubscriptionCandidates_RefreshFailedExcluded(t *testing.T) {
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a", Status: "refresh_failed", DisabledReason: "refresh_token_invalidated"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})

	candidates, failures := h.resolveOpenAISubscriptionCandidates(context.Background(), config.TianjiParams{
		Model:                           "openai/gpt-4o",
		OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"},
	}, openAISubscriptionTransportChatGPTCodexBackend)

	require.Len(t, candidates, 1)
	assert.Equal(t, "cred-b", candidates[0].CredentialID)
	require.Len(t, failures, 1)
	assert.Equal(t, OpenAISubscriptionCredentialRefreshErr, failures[0].Code)
	assert.NotContains(t, failures[0].Message, "access-a")
}

func TestOpenAISubscriptionRouting_AllFailedCandidatesReturnsClearUnusableError(t *testing.T) {
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"failed-cred": {AccessToken: "access-a", AccountID: "acct-a", Status: "refresh_failed", DisabledReason: "refresh_token_invalidated"},
	})

	_, err := h.resolveOpenAISubscriptionAvailableCandidates(context.Background(), config.TianjiParams{
		Model:                           "openai/gpt-4o",
		OpenAISubscriptionCredentialIDs: []string{"failed-cred", "missing-cred"},
	}, openAISubscriptionTransportChatGPTCodexBackend)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "all configured credentials unusable")
	assert.Contains(t, err.Error(), "failed-cred=refresh_failed")
	assert.Contains(t, err.Error(), "missing-cred=credential_missing")
	assert.NotContains(t, err.Error(), "access-a")
}

func TestResolveOpenAISubscriptionCandidates_PreservesReasonCodes(t *testing.T) {
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"disabled":  {Status: "disabled"},
		"malformed": {Malformed: true},
	})

	_, failures := h.resolveOpenAISubscriptionCandidates(context.Background(), config.TianjiParams{
		Model:                           "openai/gpt-4o",
		OpenAISubscriptionCredentialIDs: []string{"missing", "disabled", "malformed"},
	}, openAISubscriptionTransportChatGPTCodexBackend)

	require.Len(t, failures, 3)
	assert.Equal(t, OpenAISubscriptionCredentialMissing, failures[0].Code)
	assert.Equal(t, OpenAISubscriptionCredentialDisabled, failures[1].Code)
	assert.Equal(t, OpenAISubscriptionCredentialMalformed, failures[2].Code)
}

func TestOpenAISubscriptionRouting_StickyStablePerOrg(t *testing.T) {
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategySticky
	ctx := context.WithValue(context.Background(), middleware.ContextKeyOrgID, "org-a")
	params := config.TianjiParams{Model: "openai/gpt-4o", OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"}}

	first, err := h.resolveOpenAISubscriptionCredential(ctx, params, openAISubscriptionTransportChatGPTCodexBackend)
	require.NoError(t, err)
	second, err := h.resolveOpenAISubscriptionCredential(ctx, params, openAISubscriptionTransportChatGPTCodexBackend)
	require.NoError(t, err)

	assert.Equal(t, first.CredentialID, second.CredentialID)
}

func TestOpenAISubscriptionRouting_StickySeparateOrgs(t *testing.T) {
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategySticky
	params := config.TianjiParams{Model: "openai/gpt-4o", OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"}}

	first, err := h.resolveOpenAISubscriptionCredential(context.WithValue(context.Background(), middleware.ContextKeyOrgID, "org-a"), params, openAISubscriptionTransportChatGPTCodexBackend)
	require.NoError(t, err)
	second, err := h.resolveOpenAISubscriptionCredential(context.WithValue(context.Background(), middleware.ContextKeyOrgID, "org-b"), params, openAISubscriptionTransportChatGPTCodexBackend)
	require.NoError(t, err)
	firstAgain, err := h.resolveOpenAISubscriptionCredential(context.WithValue(context.Background(), middleware.ContextKeyOrgID, "org-a"), params, openAISubscriptionTransportChatGPTCodexBackend)
	require.NoError(t, err)
	secondAgain, err := h.resolveOpenAISubscriptionCredential(context.WithValue(context.Background(), middleware.ContextKeyOrgID, "org-b"), params, openAISubscriptionTransportChatGPTCodexBackend)
	require.NoError(t, err)

	assert.Equal(t, first.CredentialID, firstAgain.CredentialID)
	assert.Equal(t, second.CredentialID, secondAgain.CredentialID)
}

func TestOpenAISubscriptionRouting_StickyStableForMasterScope(t *testing.T) {
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategySticky
	params := config.TianjiParams{Model: "openai/gpt-4o", OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"}}

	first, err := h.resolveOpenAISubscriptionCredential(context.Background(), params, openAISubscriptionTransportChatGPTCodexBackend)
	require.NoError(t, err)
	second, err := h.resolveOpenAISubscriptionCredential(context.Background(), params, openAISubscriptionTransportChatGPTCodexBackend)
	require.NoError(t, err)

	assert.Equal(t, first.CredentialID, second.CredentialID)
}

func TestCodexSessionIdentity_DirectClientMetadataSessionID(t *testing.T) {
	identity := extractCodexSessionIdentity(map[string]any{
		"client_metadata": map[string]any{
			"session_id":            "  sess-direct  ",
			"x-codex-turn-metadata": `{"session_id":"sess-nested"}`,
		},
	})

	assert.Equal(t, "sess-direct", identity.SessionID)
	assert.Equal(t, "direct", identity.Source)
}

func TestCodexSessionIdentity_NestedTurnMetadataSessionID(t *testing.T) {
	identity := extractCodexSessionIdentity(map[string]any{
		"client_metadata": map[string]any{
			"x-codex-turn-metadata": `{"session_id":"  sess-nested  ","thread_id":"thread-ignored"}`,
		},
	})

	assert.Equal(t, "sess-nested", identity.SessionID)
	assert.Equal(t, "x-codex-turn-metadata", identity.Source)
}

func TestCodexSessionIdentity_MalformedNestedMetadataFallsBackSafely(t *testing.T) {
	identity := extractCodexSessionIdentity(map[string]any{
		"client_metadata": map[string]any{
			"x-codex-turn-metadata": `{"session_id":`,
			"thread_id":             "thread-must-not-route",
		},
	})

	assert.Empty(t, identity.SessionID)
	assert.Equal(t, "parse_error", identity.Source)
}

func TestCodexSessionIdentity_BlankSessionIDIsMissing(t *testing.T) {
	for _, payload := range []map[string]any{
		{"client_metadata": map[string]any{"session_id": "   "}},
		{"client_metadata": map[string]any{"x-codex-turn-metadata": `{"session_id":"   "}`}},
		{"client_metadata": map[string]any{"thread_id": "thread-must-not-route", "turn_id": "turn-must-not-route"}},
	} {
		identity := extractCodexSessionIdentity(payload)
		assert.Empty(t, identity.SessionID)
		assert.Equal(t, "missing", identity.Source)
	}
}

func TestOpenAISubscriptionRouting_LowestUtilizationUsesKnownHeaderState(t *testing.T) {
	now := time.Date(2026, 5, 7, 6, 0, 0, 0, time.UTC)
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategyLowestUtilization
	h.openAISubscriptionNow = func() time.Time { return now }
	h.recordOpenAISubscriptionRateLimit("cred-a", callback.OpenAIQuotaState{
		Requests:  callback.OpenAIQuotaDimension{Remaining: 10, RemainingKnown: true},
		Tokens:    callback.OpenAIQuotaDimension{Remaining: 10, RemainingKnown: true},
		UpdatedAt: now,
	})
	h.recordOpenAISubscriptionRateLimit("cred-b", callback.OpenAIQuotaState{
		Requests:  callback.OpenAIQuotaDimension{Remaining: 80, RemainingKnown: true},
		Tokens:    callback.OpenAIQuotaDimension{Remaining: 80, RemainingKnown: true},
		UpdatedAt: now,
	})
	params := config.TianjiParams{Model: "openai/gpt-4o", OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"}}

	selected, err := h.resolveOpenAISubscriptionCredential(context.Background(), params, openAISubscriptionTransportChatGPTCodexBackend)
	require.NoError(t, err)

	assert.Equal(t, "cred-b", selected.CredentialID)
}

func TestCodexStickySession_ReusesSelectableCredential(t *testing.T) {
	now := time.Date(2026, 5, 13, 6, 0, 0, 0, time.UTC)
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
		"cred-c": {AccessToken: "access-c", AccountID: "acct-c"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategySticky
	h.openAISubscriptionNow = func() time.Time { return now }
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-a", testCodexUsageSnapshot(0.10, 0.10, "2026-05-13T10:00:00Z", "2026-05-18T00:00:00Z", "available", "available"))
	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-b", testCodexUsageSnapshot(0.10, 0.10, "2026-05-13T10:00:00Z", "2026-05-18T00:00:00Z", "available", "available"))
	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-c", testCodexUsageSnapshot(0.10, 0.10, "2026-05-13T10:00:00Z", "2026-05-18T00:00:00Z", "available", "available"))
	params := config.TianjiParams{Model: "openai/gpt-5.5", OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b", "cred-c"}}
	resolved := []resolvedOpenAISubscriptionCredential{
		{CredentialID: "cred-a", BearerToken: "access-a", AccountID: "acct-a"},
		{CredentialID: "cred-b", BearerToken: "access-b", AccountID: "acct-b"},
		{CredentialID: "cred-c", BearerToken: "access-c", AccountID: "acct-c"},
	}
	identity := codexSessionIdentity{SessionID: "sess-reuse", Source: "direct"}

	first := h.orderOpenAISubscriptionCodexCandidatesWithIdentity(context.Background(), params, resolved, identity)
	second := h.orderOpenAISubscriptionCodexCandidatesWithIdentity(context.Background(), params, resolved, identity)

	require.Len(t, first, 3)
	require.Len(t, second, 3)
	assert.Equal(t, first[0].CredentialID, second[0].CredentialID)
	assert.NotEmpty(t, h.openAISubscriptionSticky[openAISubscriptionCodexSessionRouteKey(context.Background(), params, identity)])
}

func TestCodexSessionSticky_PrimaryResetChangedDoesNotReselectWhenStillSelectable(t *testing.T) {
	trackKey := "openai-subscription:org:org-a:codex-session:sess-stable:model:openai/gpt-5.5"
	entry := stickyStrategyEntry{
		SelectedID: "cred-a",
		Metadata: codexStickyMetadata{
			Known:          true,
			Available:      true,
			Selectable:     true,
			PrimaryResetAt: "2026-05-13T10:00:00Z",
		},
	}
	candidate := stickyStrategyCandidate{
		ID: "cred-a",
		Metadata: codexStickyMetadata{
			Known:               true,
			Available:           true,
			Selectable:          true,
			PrimaryResetAt:      "2026-05-13T15:00:00Z",
			SecondaryResetScore: 177,
			Score:               250_000,
		},
	}

	assert.True(t, codexStickyCanReuseWithTrack(trackKey, entry, candidate))
}

func TestCodexStickyFallbackRoute_PrimaryResetChangedStillReselectsWhenModelContainsSessionMarker(t *testing.T) {
	trackKey := "openai-subscription:org:org-a:model:openai/:codex-session:/gpt-5.5"
	entry := stickyStrategyEntry{
		SelectedID: "cred-a",
		Metadata: codexStickyMetadata{
			Known:          true,
			Available:      true,
			Selectable:     true,
			PrimaryResetAt: "2026-05-13T10:00:00Z",
		},
	}
	candidate := stickyStrategyCandidate{
		ID: "cred-a",
		Metadata: codexStickyMetadata{
			Known:               true,
			Available:           true,
			Selectable:          true,
			PrimaryResetAt:      "2026-05-13T15:00:00Z",
			SecondaryResetScore: 177,
			Score:               250_000,
		},
	}

	assert.False(t, codexStickyCanReuseWithTrack(trackKey, entry, candidate))
}

func TestCodexSessionSticky_StillReselectsWhenPrimaryGateExceeded(t *testing.T) {
	trackKey := "openai-subscription:org:org-a:codex-session:sess-primary-gate:model:openai/gpt-5.5"
	entry := stickyStrategyEntry{
		SelectedID: "cred-a",
		Metadata: codexStickyMetadata{
			Known:          true,
			Available:      true,
			Selectable:     true,
			PrimaryResetAt: "2026-05-13T10:00:00Z",
		},
	}
	candidate := stickyStrategyCandidate{
		ID: "cred-a",
		Metadata: codexStickyMetadata{
			Known:               true,
			Available:           true,
			Selectable:          false,
			PrimaryResetAt:      "2026-05-13T15:00:00Z",
			SecondaryResetScore: 177,
			Score:               960_000,
		},
	}

	assert.False(t, codexStickyCanReuseWithTrack(trackKey, entry, candidate))
}

func TestCodexSessionSticky_StillReselectsWhenSecondaryGateExceeded(t *testing.T) {
	trackKey := "openai-subscription:org:org-a:codex-session:sess-secondary-gate:model:openai/gpt-5.5"
	entry := stickyStrategyEntry{
		SelectedID: "cred-a",
		Metadata: codexStickyMetadata{
			Known:          true,
			Available:      true,
			Selectable:     true,
			PrimaryResetAt: "2026-05-13T10:00:00Z",
		},
	}
	candidate := stickyStrategyCandidate{
		ID: "cred-a",
		Metadata: codexStickyMetadata{
			Known:               true,
			Available:           true,
			Selectable:          false,
			PrimaryResetAt:      "2026-05-13T15:00:00Z",
			SecondaryResetScore: 177,
			Score:               950_000,
		},
	}

	assert.False(t, codexStickyCanReuseWithTrack(trackKey, entry, candidate))
}

func TestCodexSessionSticky_PrimaryResetChangedDoesNotLogReselectReasonWhenReused(t *testing.T) {
	trackKey := "openai-subscription:org:org-a:codex-session:sess-raw-secret:model:openai/gpt-5.5"
	entry := stickyStrategyEntry{
		SelectedID: "cred-a",
		Metadata: codexStickyMetadata{
			Known:          true,
			Available:      true,
			Selectable:     true,
			PrimaryResetAt: "2026-05-13T10:00:00Z",
		},
	}
	candidate := stickyStrategyCandidate{
		ID: "cred-a",
		Metadata: codexStickyMetadata{
			Known:               true,
			Available:           true,
			Selectable:          true,
			PrimaryResetAt:      "2026-05-13T15:00:00Z",
			SecondaryResetScore: 177,
			Score:               250_000,
		},
	}

	var logs bytes.Buffer
	previousOutput := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previousOutput) })

	reuse := codexStickyCanReuseWithTrack(trackKey, entry, candidate)

	assert.True(t, reuse)
	assert.NotContains(t, logs.String(), "reason=primary_reset_changed")
	assert.NotContains(t, logs.String(), "sess-raw-secret")
}

func TestCodexRendezvousSelect_DeterministicSameSession(t *testing.T) {
	candidates := []openAISubscriptionCandidate{
		{resolvedOpenAISubscriptionCredential: resolvedOpenAISubscriptionCredential{CredentialID: "cred-a"}},
		{resolvedOpenAISubscriptionCredential: resolvedOpenAISubscriptionCredential{CredentialID: "cred-b"}},
		{resolvedOpenAISubscriptionCredential: resolvedOpenAISubscriptionCredential{CredentialID: "cred-c"}},
	}

	first := codexRendezvousSelect("sess-stable", candidates)
	second := codexRendezvousSelect("sess-stable", candidates)

	assert.Equal(t, first.CredentialID, second.CredentialID)
}

func TestCodexRendezvousSelect_SpreadsMultipleSessions(t *testing.T) {
	candidates := []openAISubscriptionCandidate{
		{resolvedOpenAISubscriptionCredential: resolvedOpenAISubscriptionCredential{CredentialID: "cred-a"}},
		{resolvedOpenAISubscriptionCredential: resolvedOpenAISubscriptionCredential{CredentialID: "cred-b"}},
		{resolvedOpenAISubscriptionCredential: resolvedOpenAISubscriptionCredential{CredentialID: "cred-c"}},
		{resolvedOpenAISubscriptionCredential: resolvedOpenAISubscriptionCredential{CredentialID: "cred-d"}},
	}
	selected := map[string]bool{}

	for i := 0; i < 100; i++ {
		selected[codexRendezvousSelect(fmt.Sprintf("sess-%03d", i), candidates).CredentialID] = true
	}

	assert.Greater(t, len(selected), 1)
}

func TestCodexRendezvousSelect_RemovingCredentialOnlyRemapsAffectedSessions(t *testing.T) {
	all := []openAISubscriptionCandidate{
		{resolvedOpenAISubscriptionCredential: resolvedOpenAISubscriptionCredential{CredentialID: "cred-a"}},
		{resolvedOpenAISubscriptionCredential: resolvedOpenAISubscriptionCredential{CredentialID: "cred-b"}},
		{resolvedOpenAISubscriptionCredential: resolvedOpenAISubscriptionCredential{CredentialID: "cred-c"}},
		{resolvedOpenAISubscriptionCredential: resolvedOpenAISubscriptionCredential{CredentialID: "cred-d"}},
	}
	removed := []openAISubscriptionCandidate{all[0], all[2], all[3]}

	for i := 0; i < 100; i++ {
		sessionID := fmt.Sprintf("sess-%03d", i)
		before := codexRendezvousSelect(sessionID, all)
		after := codexRendezvousSelect(sessionID, removed)
		if before.CredentialID != "cred-b" {
			assert.Equal(t, before.CredentialID, after.CredentialID)
		}
	}
}

func TestCodexStickySession_ReselectsWhenCredentialOverGate(t *testing.T) {
	now := time.Date(2026, 5, 13, 6, 0, 0, 0, time.UTC)
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
		"cred-c": {AccessToken: "access-c", AccountID: "acct-c"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategySticky
	h.Config.RatelimitAlertThreshold = 0.95
	h.Config.CodexUsageWeeklyThreshold = 0.95
	h.openAISubscriptionNow = func() time.Time { return now }
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	for _, credentialID := range []string{"cred-a", "cred-b", "cred-c"} {
		seedFreshCodexUsage(t, h.CodexUsageCache, now, credentialID, testCodexUsageSnapshot(0.10, 0.10, "2026-05-13T10:00:00Z", "2026-05-18T00:00:00Z", "available", "available"))
	}
	params := config.TianjiParams{Model: "openai/gpt-5.5", OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b", "cred-c"}}
	resolved := []resolvedOpenAISubscriptionCredential{
		{CredentialID: "cred-a", BearerToken: "access-a", AccountID: "acct-a"},
		{CredentialID: "cred-b", BearerToken: "access-b", AccountID: "acct-b"},
		{CredentialID: "cred-c", BearerToken: "access-c", AccountID: "acct-c"},
	}
	identity := codexSessionIdentity{SessionID: "sess-reselect", Source: "direct"}
	first := h.orderOpenAISubscriptionCodexCandidatesWithIdentity(context.Background(), params, resolved, identity)
	require.Len(t, first, 3)

	seedFreshCodexUsage(t, h.CodexUsageCache, now, first[0].CredentialID, testCodexUsageSnapshot(0.96, 0.10, "2026-05-13T10:00:00Z", "2026-05-18T00:00:00Z", "available", "available"))
	second := h.orderOpenAISubscriptionCodexCandidatesWithIdentity(context.Background(), params, resolved, identity)

	require.Len(t, second, 3)
	assert.NotEqual(t, first[0].CredentialID, second[0].CredentialID)
}

func TestCodexSessionSticky_MissingSessionUsesExistingRouteKey(t *testing.T) {
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategySticky
	ctx := context.WithValue(context.Background(), middleware.ContextKeyOrgID, "org-a")
	params := config.TianjiParams{Model: "openai/gpt-5.5", OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"}}
	resolved := []resolvedOpenAISubscriptionCredential{
		{CredentialID: "cred-a", BearerToken: "access-a", AccountID: "acct-a"},
		{CredentialID: "cred-b", BearerToken: "access-b", AccountID: "acct-b"},
	}

	got := h.orderOpenAISubscriptionCodexCandidatesWithIdentity(ctx, params, resolved, codexSessionIdentity{Source: "missing"})

	require.Len(t, got, 2)
	assert.Equal(t, "cred-a", got[0].CredentialID)
	assert.Equal(t, "cred-a", h.openAISubscriptionSticky[openAISubscriptionRouteKey(ctx, params)])
	assert.Empty(t, h.openAISubscriptionSticky[openAISubscriptionCodexSessionRouteKey(ctx, params, codexSessionIdentity{SessionID: "sess-unused", Source: "direct"})])
}

func TestCodexSessionSticky_LogsDoNotExposeRawSessionID(t *testing.T) {
	now := time.Date(2026, 5, 13, 6, 0, 0, 0, time.UTC)
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategySticky
	h.openAISubscriptionNow = func() time.Time { return now }
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-a", testCodexUsageSnapshot(0.10, 0.10, "2026-05-13T10:00:00Z", "2026-05-18T00:00:00Z", "available", "available"))
	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-b", testCodexUsageSnapshot(0.10, 0.10, "2026-05-13T10:00:00Z", "2026-05-18T00:00:00Z", "available", "available"))
	params := config.TianjiParams{
		Model:                           "openai/gpt-5.5",
		OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"},
		OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportChatGPTCodexBackend,
	}
	rawSessionID := "sess-raw-secret-should-not-log"
	identity := codexSessionIdentity{SessionID: rawSessionID, Source: "direct"}
	resolved := []resolvedOpenAISubscriptionCredential{
		{CredentialID: "cred-a", BearerToken: "access-a", AccountID: "acct-a"},
		{CredentialID: "cred-b", BearerToken: "access-b", AccountID: "acct-b"},
	}

	var logs bytes.Buffer
	previousOutput := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previousOutput) })
	t.Setenv("TIANJI_DEBUG_CODEX_ROUTING", "1")

	first := h.orderOpenAISubscriptionCodexCandidatesWithIdentity(context.Background(), params, resolved, identity)
	require.Len(t, first, 2)
	seedFreshCodexUsage(t, h.CodexUsageCache, now, first[0].CredentialID, testCodexUsageSnapshot(0.96, 0.10, "2026-05-13T10:00:00Z", "2026-05-18T00:00:00Z", "available", "available"))
	_ = h.orderOpenAISubscriptionCodexCandidatesWithIdentity(context.Background(), params, resolved, identity)

	got := logs.String()
	assert.NotContains(t, got, rawSessionID)
	assert.Contains(t, got, "codex-session:sha256:")
	assert.NotContains(t, got, "codex-session:"+rawSessionID)
}

func TestOpenAISubscriptionRouting_StickyReevaluatesUnavailableCredential(t *testing.T) {
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategySticky
	ctx := context.WithValue(context.Background(), middleware.ContextKeyOrgID, "org-a")
	params := config.TianjiParams{Model: "openai/gpt-4o", OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"}}

	first, err := h.resolveOpenAISubscriptionCredential(ctx, params, openAISubscriptionTransportChatGPTCodexBackend)
	require.NoError(t, err)
	h.recordOpenAISubscriptionRateLimit(first.CredentialID, callback.OpenAIQuotaState{
		Requests: callback.OpenAIQuotaDimension{
			Remaining:      0,
			RemainingKnown: true,
			ResetAt:        time.Now().Add(time.Minute),
			ResetKnown:     true,
		},
	})
	second, err := h.resolveOpenAISubscriptionCredential(ctx, params, openAISubscriptionTransportChatGPTCodexBackend)
	require.NoError(t, err)

	assert.NotEqual(t, first.CredentialID, second.CredentialID)
}

func TestOpenAISubscriptionRouting_UsesSharedOpenAIQuotaStoreForGate(t *testing.T) {
	now := time.Date(2026, 5, 7, 6, 0, 0, 0, time.UTC)
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.RateLimitStore = callback.NewInMemoryRateLimitStore()
	h.openAISubscriptionNow = func() time.Time { return now }
	h.RateLimitStore.SetOpenAIQuotaState("cred-a", callback.OpenAIQuotaState{
		SubjectID: "cred-a",
		Requests: callback.OpenAIQuotaDimension{
			Remaining:      0,
			RemainingKnown: true,
			ResetAt:        now.Add(time.Minute),
			ResetKnown:     true,
		},
		UpdatedAt: now,
	})
	params := config.TianjiParams{Model: "openai/gpt-4o", OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"}}

	selected, err := h.resolveOpenAISubscriptionCredential(context.Background(), params, openAISubscriptionTransportChatGPTCodexBackend)
	require.NoError(t, err)

	assert.Equal(t, "cred-b", selected.CredentialID)
}

func TestOpenAISubscriptionRouting_RoundRobinDefault(t *testing.T) {
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	params := config.TianjiParams{Model: "openai/gpt-4o", OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"}}

	first, err := h.resolveOpenAISubscriptionCredential(context.Background(), params, openAISubscriptionTransportChatGPTCodexBackend)
	require.NoError(t, err)
	second, err := h.resolveOpenAISubscriptionCredential(context.Background(), params, openAISubscriptionTransportChatGPTCodexBackend)
	require.NoError(t, err)

	assert.NotEqual(t, first.CredentialID, second.CredentialID)
}

func TestOpenAISubscriptionRouting_StickyCodexFreshSnapshotPrefersSoonestWeeklyReset(t *testing.T) {
	now := time.Date(2026, 5, 13, 6, 0, 0, 0, time.UTC)
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategySticky
	h.openAISubscriptionNow = func() time.Time { return now }
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-a", testCodexUsageSnapshot(0.42, 0.20, "2026-05-13T10:00:00Z", "2026-05-18T00:00:00Z", "available", "available"))
	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-b", testCodexUsageSnapshot(0.08, 0.20, "2026-05-13T09:00:00Z", "2026-05-19T00:00:00Z", "available", "available"))
	metaA := h.codexStickyMetadata("cred-a", now)
	metaB := h.codexStickyMetadata("cred-b", now)
	require.True(t, metaA.Known)
	require.True(t, metaB.Known)
	require.Less(t, metaA.SecondaryResetScore, metaB.SecondaryResetScore)

	selected, err := h.resolveOpenAISubscriptionCredential(context.Background(), config.TianjiParams{
		Model:                           "openai/gpt-5.5",
		OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"},
	}, openAISubscriptionTransportChatGPTCodexBackend)
	require.NoError(t, err)

	assert.Equal(t, "cred-a", selected.CredentialID)
}

func TestOpenAISubscriptionRouting_StickyCodexWeeklyResetBreaksPrimaryTie(t *testing.T) {
	now := time.Date(2026, 5, 13, 6, 0, 0, 0, time.UTC)
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategySticky
	h.openAISubscriptionNow = func() time.Time { return now }
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-a", testCodexUsageSnapshot(0.20, 0.72, "2026-05-13T10:00:00Z", "2026-05-18T00:00:00Z", "available", "available"))
	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-b", testCodexUsageSnapshot(0.20, 0.18, "2026-05-13T10:00:00Z", "2026-05-16T00:00:00Z", "available", "available"))
	metaA := h.codexStickyMetadata("cred-a", now)
	metaB := h.codexStickyMetadata("cred-b", now)
	require.True(t, metaA.Known)
	require.True(t, metaB.Known)
	require.Less(t, metaB.Score, metaA.Score)

	selected, err := h.resolveOpenAISubscriptionCredential(context.Background(), config.TianjiParams{
		Model:                           "openai/gpt-5.5",
		OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"},
	}, openAISubscriptionTransportChatGPTCodexBackend)
	require.NoError(t, err)

	assert.Equal(t, "cred-b", selected.CredentialID)
}

func TestOpenAISubscriptionRouting_StickyCodexSkipsUnavailableKnownSnapshot(t *testing.T) {
	now := time.Date(2026, 5, 13, 6, 0, 0, 0, time.UTC)
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategySticky
	h.openAISubscriptionNow = func() time.Time { return now }
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-a", testCodexUsageSnapshot(0, 1, "2026-05-13T11:00:00Z", "2026-05-18T00:00:00Z", "available", "exhausted"))
	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-b", testCodexUsageSnapshot(0.899, 0.20, "2026-05-13T11:00:00Z", "2026-05-18T00:00:00Z", "available", "available"))
	metaA := h.codexStickyMetadata("cred-a", now)
	metaB := h.codexStickyMetadata("cred-b", now)
	require.True(t, metaA.Known)
	require.False(t, metaA.Available)
	require.True(t, metaB.Known)
	require.True(t, metaB.Available)
	require.Greater(t, metaB.Score, metaA.Score)

	selected, err := h.resolveOpenAISubscriptionCredential(context.Background(), config.TianjiParams{
		Model:                           "openai/gpt-5.5",
		OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"},
	}, openAISubscriptionTransportChatGPTCodexBackend)
	require.NoError(t, err)

	assert.Equal(t, "cred-b", selected.CredentialID)
}

func TestOpenAISubscriptionRouting_StickyCodexSkipsPrimaryGateBeforeResetSelection(t *testing.T) {
	now := time.Date(2026, 5, 13, 6, 0, 0, 0, time.UTC)
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategySticky
	h.openAISubscriptionNow = func() time.Time { return now }
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-a", testCodexUsageSnapshot(0.95, 0.10, "2026-05-13T10:00:00Z", "2026-05-16T00:00:00Z", "available", "available"))
	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-b", testCodexUsageSnapshot(0.10, 0.20, "2026-05-13T10:00:00Z", "2026-05-18T00:00:00Z", "available", "available"))

	metaA := h.codexStickyMetadata("cred-a", now)
	metaB := h.codexStickyMetadata("cred-b", now)
	require.True(t, metaA.Known)
	require.True(t, metaA.Available)
	require.False(t, metaA.Selectable)
	require.True(t, metaB.Selectable)
	require.Less(t, metaA.SecondaryResetScore, metaB.SecondaryResetScore)

	selected, err := h.resolveOpenAISubscriptionCredential(context.Background(), config.TianjiParams{
		Model:                           "openai/gpt-5.5",
		OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"},
	}, openAISubscriptionTransportChatGPTCodexBackend)
	require.NoError(t, err)

	assert.Equal(t, "cred-b", selected.CredentialID)
}

func TestOpenAISubscriptionRouting_StickyCodexSkipsSecondaryGateBeforeResetSelection(t *testing.T) {
	now := time.Date(2026, 5, 13, 6, 0, 0, 0, time.UTC)
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategySticky
	h.openAISubscriptionNow = func() time.Time { return now }
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-a", testCodexUsageSnapshot(0.10, 0.95, "2026-05-13T10:00:00Z", "2026-05-16T00:00:00Z", "available", "available"))
	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-b", testCodexUsageSnapshot(0.10, 0.20, "2026-05-13T10:00:00Z", "2026-05-18T00:00:00Z", "available", "available"))

	metaA := h.codexStickyMetadata("cred-a", now)
	metaB := h.codexStickyMetadata("cred-b", now)
	require.True(t, metaA.Known)
	require.True(t, metaA.Available)
	require.False(t, metaA.Selectable)
	require.True(t, metaB.Selectable)
	require.Less(t, metaA.SecondaryResetScore, metaB.SecondaryResetScore)

	selected, err := h.resolveOpenAISubscriptionCredential(context.Background(), config.TianjiParams{
		Model:                           "openai/gpt-5.5",
		OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"},
	}, openAISubscriptionTransportChatGPTCodexBackend)
	require.NoError(t, err)

	assert.Equal(t, "cred-b", selected.CredentialID)
}

func TestOpenAISubscriptionRouting_StickyCodexSkipsConfiguredSecondaryGateBeforeResetSelection(t *testing.T) {
	now := time.Date(2026, 5, 13, 6, 0, 0, 0, time.UTC)
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategySticky
	h.Config.CodexUsageWeeklyThreshold = 0.75
	h.openAISubscriptionNow = func() time.Time { return now }
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-a", testCodexUsageSnapshot(0.10, 0.80, "2026-05-13T10:00:00Z", "2026-05-16T00:00:00Z", "available", "available"))
	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-b", testCodexUsageSnapshot(0.10, 0.20, "2026-05-13T10:00:00Z", "2026-05-18T00:00:00Z", "available", "available"))

	metaA := h.codexStickyMetadata("cred-a", now)
	metaB := h.codexStickyMetadata("cred-b", now)
	require.True(t, metaA.Known)
	require.True(t, metaA.Available)
	require.False(t, metaA.Selectable)
	require.True(t, metaB.Selectable)
	require.Less(t, metaA.SecondaryResetScore, metaB.SecondaryResetScore)

	selected, err := h.resolveOpenAISubscriptionCredential(context.Background(), config.TianjiParams{
		Model:                           "openai/gpt-5.5",
		OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"},
	}, openAISubscriptionTransportChatGPTCodexBackend)
	require.NoError(t, err)

	assert.Equal(t, "cred-b", selected.CredentialID)
}

func TestOpenAISubscriptionRouting_ChatGPTCodexAllUsageGatedReturnsUnusableError(t *testing.T) {
	now := time.Date(2026, 5, 13, 6, 0, 0, 0, time.UTC)
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategySticky
	h.Config.RatelimitAlertThreshold = 0.95
	h.Config.CodexUsageWeeklyThreshold = 0.95
	h.openAISubscriptionNow = func() time.Time { return now }
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-a", testCodexUsageSnapshot(0.95, 0.10, "2026-05-13T10:00:00Z", "2026-05-18T00:00:00Z", "available", "available"))
	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-b", testCodexUsageSnapshot(0.10, 0.95, "2026-05-13T10:00:00Z", "2026-05-18T00:00:00Z", "available", "available"))

	_, err := h.resolveOpenAISubscriptionCredential(context.Background(), config.TianjiParams{
		Model:                           "openai/gpt-5.5",
		OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"},
	}, openAISubscriptionTransportChatGPTCodexBackend)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "all configured credentials unusable")
	assert.Contains(t, err.Error(), "cred-a=codex_usage_gate")
	assert.Contains(t, err.Error(), "cred-b=codex_usage_gate")
	assert.Empty(t, h.openAISubscriptionSticky)
}

func TestOpenAISubscriptionRouting_ChatGPTCodexFiltersUsageGatedBeforeStickySelection(t *testing.T) {
	now := time.Date(2026, 5, 13, 6, 0, 0, 0, time.UTC)
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategySticky
	h.Config.RatelimitAlertThreshold = 0.95
	h.Config.CodexUsageWeeklyThreshold = 0.95
	h.openAISubscriptionNow = func() time.Time { return now }
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-a", testCodexUsageSnapshot(0.95, 0.10, "2026-05-13T10:00:00Z", "2026-05-18T00:00:00Z", "available", "available"))
	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-b", testCodexUsageSnapshot(0.10, 0.20, "2026-05-13T10:00:00Z", "2026-05-18T00:00:00Z", "available", "available"))

	selected, err := h.resolveOpenAISubscriptionCredential(context.Background(), config.TianjiParams{
		Model:                           "openai/gpt-5.5",
		OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"},
	}, openAISubscriptionTransportChatGPTCodexBackend)

	require.NoError(t, err)
	assert.Equal(t, "cred-b", selected.CredentialID)
}

func TestOpenAISubscriptionRouting_StickyCodexKnownUnavailableDoesNotBeatUnknown(t *testing.T) {
	now := time.Date(2026, 5, 13, 6, 0, 0, 0, time.UTC)
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategySticky
	h.openAISubscriptionNow = func() time.Time { return now }
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-a", testCodexUsageSnapshot(0, 1, "2026-05-13T11:00:00Z", "2026-05-18T00:00:00Z", "available", "exhausted"))

	for _, ids := range [][]string{{"cred-a", "cred-b"}, {"cred-b", "cred-a"}} {
		selected, err := h.resolveOpenAISubscriptionCredential(context.Background(), config.TianjiParams{
			Model:                           "openai/gpt-5.5",
			OpenAISubscriptionCredentialIDs: ids,
		}, openAISubscriptionTransportChatGPTCodexBackend)
		require.NoError(t, err)

		assert.Equal(t, "cred-b", selected.CredentialID)
	}
}

func TestOpenAISubscriptionRouting_StickyCodexReevaluatesWhenCurrentCredentialUnavailable(t *testing.T) {
	now := time.Date(2026, 5, 13, 6, 0, 0, 0, time.UTC)
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategySticky
	h.openAISubscriptionNow = func() time.Time { return now }
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	params := config.TianjiParams{
		Model:                           "openai/gpt-5.5",
		OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"},
	}

	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-a", testCodexUsageSnapshot(0.05, 0.20, "2026-05-13T10:00:00Z", "2026-05-18T00:00:00Z", "available", "available"))
	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-b", testCodexUsageSnapshot(0.20, 0.20, "2026-05-13T10:00:00Z", "2026-05-18T00:00:00Z", "available", "available"))

	first, err := h.resolveOpenAISubscriptionCredential(context.Background(), params, openAISubscriptionTransportChatGPTCodexBackend)
	require.NoError(t, err)
	require.Equal(t, "cred-a", first.CredentialID)

	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-a", testCodexUsageSnapshot(0.95, 0.95, "2026-05-13T15:00:00Z", "2026-05-20T00:00:00Z", "exhausted", "available"))
	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-b", testCodexUsageSnapshot(0.15, 0.20, "2026-05-13T10:00:00Z", "2026-05-18T00:00:00Z", "available", "available"))

	second, err := h.resolveOpenAISubscriptionCredential(context.Background(), params, openAISubscriptionTransportChatGPTCodexBackend)
	require.NoError(t, err)

	assert.Equal(t, "cred-b", second.CredentialID)
}

func TestOpenAISubscriptionRouting_StickyCodexKeepsCurrentCredentialWhenUsageChangesWithinPrimaryWindow(t *testing.T) {
	now := time.Date(2026, 5, 13, 6, 0, 0, 0, time.UTC)
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategySticky
	h.openAISubscriptionNow = func() time.Time { return now }
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	params := config.TianjiParams{
		Model:                           "openai/gpt-5.5",
		OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"},
	}

	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-a", testCodexUsageSnapshot(0.05, 0.10, "2026-05-13T10:00:00Z", "2026-05-18T00:00:00Z", "available", "available"))
	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-b", testCodexUsageSnapshot(0.20, 0.20, "2026-05-13T10:00:00Z", "2026-05-19T00:00:00Z", "available", "available"))

	first, err := h.resolveOpenAISubscriptionCredential(context.Background(), params, openAISubscriptionTransportChatGPTCodexBackend)
	require.NoError(t, err)
	require.Equal(t, "cred-a", first.CredentialID)

	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-a", testCodexUsageSnapshot(0.85, 0.85, "2026-05-13T10:00:00Z", "2026-05-20T00:00:00Z", "available", "available"))
	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-b", testCodexUsageSnapshot(0.05, 0.10, "2026-05-13T10:00:00Z", "2026-05-18T00:00:00Z", "available", "available"))

	second, err := h.resolveOpenAISubscriptionCredential(context.Background(), params, openAISubscriptionTransportChatGPTCodexBackend)
	require.NoError(t, err)

	assert.Equal(t, "cred-a", second.CredentialID)
}

func TestOpenAISubscriptionRouting_StickyCodexMissingCurrentSnapshotReevaluatesToKnownCandidate(t *testing.T) {
	now := time.Date(2026, 5, 13, 6, 0, 0, 0, time.UTC)
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategySticky
	h.openAISubscriptionNow = func() time.Time { return now }
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	params := config.TianjiParams{
		Model:                           "openai/gpt-5.5",
		OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"},
	}

	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-a", testCodexUsageSnapshot(0.05, 0.10, "2026-05-13T10:00:00Z", "2026-05-18T00:00:00Z", "available", "available"))
	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-b", testCodexUsageSnapshot(0.20, 0.20, "2026-05-13T10:00:00Z", "2026-05-19T00:00:00Z", "available", "available"))

	first, err := h.resolveOpenAISubscriptionCredential(context.Background(), params, openAISubscriptionTransportChatGPTCodexBackend)
	require.NoError(t, err)
	require.Equal(t, "cred-a", first.CredentialID)

	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-b", testCodexUsageSnapshot(0.10, 0.20, "2026-05-13T10:00:00Z", "2026-05-18T00:00:00Z", "available", "available"))

	second, err := h.resolveOpenAISubscriptionCredential(context.Background(), params, openAISubscriptionTransportChatGPTCodexBackend)
	require.NoError(t, err)

	assert.Equal(t, "cred-b", second.CredentialID)
}

func TestOpenAISubscriptionRouting_StickyCodexMissingSnapshotsKeepsCurrentBehavior(t *testing.T) {
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategySticky
	params := config.TianjiParams{
		Model:                           "openai/gpt-5.5",
		OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"},
	}

	first, err := h.resolveOpenAISubscriptionCredential(context.Background(), params, openAISubscriptionTransportChatGPTCodexBackend)
	require.NoError(t, err)
	second, err := h.resolveOpenAISubscriptionCredential(context.Background(), params, openAISubscriptionTransportChatGPTCodexBackend)
	require.NoError(t, err)

	assert.Equal(t, "cred-a", first.CredentialID)
	assert.Equal(t, first.CredentialID, second.CredentialID)
}

func TestOpenAISubscriptionRouting_StickyCodexPartialSnapshotsFallbackToCurrentBehavior(t *testing.T) {
	now := time.Date(2026, 5, 13, 6, 0, 0, 0, time.UTC)
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategySticky
	h.openAISubscriptionNow = func() time.Time { return now }
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-b", chatgptcodex.UsageSnapshot{
		PrimaryWindow: chatgptcodex.UsageWindow{
			Name:        "primary_5h",
			UsedPercent: float64Ptr(0.05),
			ResetAt:     "2026-05-13T10:00:00Z",
			Status:      "available",
		},
		WeeklyWindow: chatgptcodex.UsageWindow{
			Name:    "weekly_7d",
			ResetAt: "2026-05-18T00:00:00Z",
			Status:  "available",
		},
	})
	params := config.TianjiParams{
		Model:                           "openai/gpt-5.5",
		OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"},
	}

	meta := h.codexStickyMetadata("cred-b", now)
	require.False(t, meta.Known)

	first, err := h.resolveOpenAISubscriptionCredential(context.Background(), params, openAISubscriptionTransportChatGPTCodexBackend)
	require.NoError(t, err)
	second, err := h.resolveOpenAISubscriptionCredential(context.Background(), params, openAISubscriptionTransportChatGPTCodexBackend)
	require.NoError(t, err)

	assert.Equal(t, "cred-a", first.CredentialID)
	assert.Equal(t, first.CredentialID, second.CredentialID)
}

func TestOpenAISubscriptionRouting_StickyCodexLoadsSharedCacheAfterRestart(t *testing.T) {
	now := time.Date(2026, 5, 13, 6, 0, 0, 0, time.UTC)
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategySticky
	h.openAISubscriptionNow = func() time.Time { return now }
	shared := newRecordingCache()
	h.Cache = shared
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	h.persistOpenAISubscriptionCodexUsageResult(context.Background(), OpenAISubscriptionCodexUsageResult{
		CredentialID: "cred-a",
		Status:       "fresh",
		Snapshot:     testCodexUsageSnapshot(0.70, 0.70, now.Add(time.Hour).Format(time.RFC3339), now.Add(24*time.Hour).Format(time.RFC3339), "available", "available"),
		FetchedAt:    now,
		ExpiresAt:    now.Add(time.Hour),
	}, now)
	h.persistOpenAISubscriptionCodexUsageResult(context.Background(), OpenAISubscriptionCodexUsageResult{
		CredentialID: "cred-b",
		Status:       "fresh",
		Snapshot:     testCodexUsageSnapshot(0.10, 0.20, now.Add(time.Hour).Format(time.RFC3339), now.Add(12*time.Hour).Format(time.RFC3339), "available", "available"),
		FetchedAt:    now,
		ExpiresAt:    now.Add(time.Hour),
	}, now)
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	params := config.TianjiParams{
		Model:                           "openai/gpt-5.5",
		OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"},
	}

	selected, err := h.resolveOpenAISubscriptionCredential(context.Background(), params, openAISubscriptionTransportChatGPTCodexBackend)

	require.NoError(t, err)
	assert.Equal(t, "cred-b", selected.CredentialID)
	assert.Equal(t, 1, shared.mgetCount())
	assert.Equal(t, 0, shared.getCount())
}

func TestOrderCodexCandidates_RoundRobinDoesNotRefreshOrBlock(t *testing.T) {
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategyRoundRobin
	fetcher := newBlockingCodexUsageFetcher()
	h.CodexUsageFetcher = fetcher
	resolved := []resolvedOpenAISubscriptionCredential{
		{CredentialID: "cred-a", BearerToken: "access-a", AccountID: "acct-a"},
		{CredentialID: "cred-b", BearerToken: "access-b", AccountID: "acct-b"},
	}
	params := config.TianjiParams{
		Model:                           "openai/gpt-5.5",
		OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"},
	}
	done := make(chan []resolvedOpenAISubscriptionCredential, 1)

	go func() {
		done <- h.orderOpenAISubscriptionCodexCandidates(context.Background(), params, resolved)
	}()

	select {
	case got := <-done:
		require.Len(t, got, 2)
		assert.Equal(t, "cred-a", got[0].CredentialID)
		assert.Equal(t, "cred-b", got[1].CredentialID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("round-robin Codex ordering blocked request routing")
	}
	assert.Never(t, func() bool { return fetcher.startedCount() > 0 }, 100*time.Millisecond, 10*time.Millisecond)
}

func TestOrderCodexCandidates_LowestUtilizationUsesCachedSnapshotWithoutRefresh(t *testing.T) {
	now := time.Date(2026, 5, 13, 6, 0, 0, 0, time.UTC)
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategyLowestUtilization
	h.openAISubscriptionNow = func() time.Time { return now }
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	seedFreshCodexUsageUntil(t, h.CodexUsageCache, now.Add(-11*time.Second), now.Add(time.Hour), "cred-a", testCodexUsageSnapshot(0.10, 0.10, now.Add(time.Hour).Format(time.RFC3339), now.Add(24*time.Hour).Format(time.RFC3339), "available", "available"))
	seedFreshCodexUsageUntil(t, h.CodexUsageCache, now.Add(-11*time.Second), now.Add(time.Hour), "cred-b", testCodexUsageSnapshot(0.90, 0.90, now.Add(time.Hour).Format(time.RFC3339), now.Add(24*time.Hour).Format(time.RFC3339), "available", "available"))
	fetcher := &fakeCodexUsageFetcher{responses: []fakeCodexUsageResponse{
		{snapshot: testCodexUsageSnapshot(0.90, 0.90, now.Add(time.Hour).Format(time.RFC3339), now.Add(24*time.Hour).Format(time.RFC3339), "available", "available")},
		{snapshot: testCodexUsageSnapshot(0.10, 0.10, now.Add(time.Hour).Format(time.RFC3339), now.Add(24*time.Hour).Format(time.RFC3339), "available", "available")},
	}}
	h.CodexUsageFetcher = fetcher
	resolved := []resolvedOpenAISubscriptionCredential{
		{CredentialID: "cred-a", BearerToken: "access-a", AccountID: "acct-a"},
		{CredentialID: "cred-b", BearerToken: "access-b", AccountID: "acct-b"},
	}
	params := config.TianjiParams{
		Model:                           "openai/gpt-5.5",
		OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"},
		OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportChatGPTCodexBackend,
	}

	got := h.orderOpenAISubscriptionCodexCandidates(context.Background(), params, resolved)

	require.Len(t, got, 2)
	assert.Equal(t, "cred-a", got[0].CredentialID)
	assert.Empty(t, fetcher.callsSnapshot())
}

func TestRefreshCodexUsageAfterResponseStartsAsync(t *testing.T) {
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
	})
	fetcher := newBlockingCodexUsageFetcher()
	h.CodexUsageFetcher = fetcher
	resolved := []resolvedOpenAISubscriptionCredential{
		{CredentialID: "cred-a", BearerToken: "access-a", AccountID: "acct-a"},
	}

	start := time.Now()
	h.refreshOpenAISubscriptionCodexUsageAfterResponse(context.Background(), resolved)

	assert.Less(t, time.Since(start), 100*time.Millisecond)
	require.Eventually(t, func() bool { return fetcher.startedCount() == 1 }, time.Second, 10*time.Millisecond)
	fetcher.release()
	require.Eventually(t, func() bool { return fetcher.finishedCount() == 1 }, time.Second, 10*time.Millisecond)
}

func TestRefreshCodexUsageAfterResponseAsyncRefreshHasTimeout(t *testing.T) {
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategyRoundRobin
	asyncTimeoutSeconds := 1
	h.Config.TianjiSettings.CodexUsageAsyncTimeoutSeconds = &asyncTimeoutSeconds
	fetcher := newBlockingCodexUsageFetcher()
	h.CodexUsageFetcher = fetcher
	resolved := []resolvedOpenAISubscriptionCredential{
		{CredentialID: "cred-a", BearerToken: "access-a", AccountID: "acct-a"},
	}

	h.refreshOpenAISubscriptionCodexUsageAfterResponse(context.Background(), resolved)

	require.Eventually(t, func() bool { return fetcher.startedCount() == 1 }, time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool { return fetcher.finishedCount() == 1 }, 2*time.Second, 10*time.Millisecond)
}

func TestRefreshCodexUsageAfterResponseDedupesInFlight(t *testing.T) {
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategyRoundRobin
	fetcher := newBlockingCodexUsageFetcher()
	h.CodexUsageFetcher = fetcher
	resolved := []resolvedOpenAISubscriptionCredential{
		{CredentialID: "cred-a", BearerToken: "access-a", AccountID: "acct-a"},
	}

	for i := 0; i < 5; i++ {
		h.refreshOpenAISubscriptionCodexUsageAfterResponse(context.Background(), resolved)
	}

	require.Eventually(t, func() bool { return fetcher.startedCount() == 1 }, time.Second, 10*time.Millisecond)
	assert.Never(t, func() bool { return fetcher.startedCount() > 1 }, 100*time.Millisecond, 10*time.Millisecond)
	fetcher.release()
	require.Eventually(t, func() bool { return fetcher.finishedCount() == 1 }, time.Second, 10*time.Millisecond)
}

func TestOpenAISubscriptionRouting_StickyCodexMetadataUsesAbsoluteSecondaryResetScore(t *testing.T) {
	fetchedAt := time.Date(2026, 5, 13, 6, 0, 0, 0, time.UTC)
	now := fetchedAt.Add(30 * time.Minute)
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategySticky
	h.openAISubscriptionNow = func() time.Time { return now }
	refreshIntervalSeconds := int((time.Hour) / time.Second)
	h.Config.TianjiSettings.CodexUsageRefreshIntervalSeconds = &refreshIntervalSeconds
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	snapshot := testCodexUsageSnapshot(0.45, 0.10, "2026-05-13T07:00:00Z", "2026-05-18T00:00:00Z", "available", "available")
	h.CodexUsageCache.putSuccess("cred-a", OpenAISubscriptionCodexUsageResult{
		CredentialID: "cred-a",
		Status:       "fresh",
		FetchedAt:    fetchedAt,
		ExpiresAt:    fetchedAt.Add(time.Hour),
		Snapshot:     snapshot,
	})

	meta := h.codexStickyMetadata("cred-a", now)

	require.True(t, meta.Known)
	assert.Equal(t, time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC).Unix(), meta.SecondaryResetScore)
}

func TestOpenAISubscriptionRouting_StickyCodexAgedScoreDoesNotReselectWithinPrimaryWindow(t *testing.T) {
	fetchedAt := time.Date(2026, 5, 13, 6, 0, 0, 0, time.UTC)
	now := fetchedAt
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategySticky
	h.openAISubscriptionNow = func() time.Time { return now }
	refreshIntervalSeconds := int((time.Hour) / time.Second)
	h.Config.TianjiSettings.CodexUsageRefreshIntervalSeconds = &refreshIntervalSeconds
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	seedFreshCodexUsageUntil(t, h.CodexUsageCache, fetchedAt, fetchedAt.Add(5*time.Hour), "cred-a", testCodexUsageSnapshot(0, 0, "2026-05-13T11:00:00Z", "2026-05-18T00:00:00Z", "available", "available"))
	seedFreshCodexUsageUntil(t, h.CodexUsageCache, fetchedAt, fetchedAt.Add(5*time.Hour), "cred-b", testCodexUsageSnapshot(0.80, 0, "2026-05-13T07:00:00Z", "2026-05-18T00:00:00Z", "available", "available"))
	params := config.TianjiParams{
		Model:                           "openai/gpt-5.5",
		OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"},
	}

	first, err := h.resolveOpenAISubscriptionCredential(context.Background(), params, openAISubscriptionTransportChatGPTCodexBackend)
	require.NoError(t, err)
	require.Equal(t, "cred-a", first.CredentialID)

	now = fetchedAt.Add(45 * time.Minute)
	second, err := h.resolveOpenAISubscriptionCredential(context.Background(), params, openAISubscriptionTransportChatGPTCodexBackend)
	require.NoError(t, err)

	assert.Equal(t, "cred-a", second.CredentialID)
}

func codexSessionForCredential(t *testing.T, targetCredentialID string, credentialIDs []string) string {
	t.Helper()
	candidates := make([]openAISubscriptionCandidate, 0, len(credentialIDs))
	for _, credentialID := range credentialIDs {
		candidates = append(candidates, openAISubscriptionCandidate{
			resolvedOpenAISubscriptionCredential: resolvedOpenAISubscriptionCredential{CredentialID: credentialID},
		})
	}
	for i := 0; i < 1000; i++ {
		sessionID := fmt.Sprintf("sess-target-%03d", i)
		if codexRendezvousSelect(sessionID, candidates).CredentialID == targetCredentialID {
			return sessionID
		}
	}
	t.Fatalf("no session mapped to %s", targetCredentialID)
	return ""
}

func TestOpenAISubscriptionRouting_StickyCodexUsesStaleButUsableSnapshot(t *testing.T) {
	fetchedAt := time.Date(2026, 5, 13, 6, 0, 0, 0, time.UTC)
	now := fetchedAt
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategySticky
	h.openAISubscriptionNow = func() time.Time { return now }
	refreshIntervalSeconds := 10
	h.Config.TianjiSettings.CodexUsageRefreshIntervalSeconds = &refreshIntervalSeconds
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	fetcher := &fakeCodexUsageFetcher{}
	h.CodexUsageFetcher = fetcher
	seedFreshCodexUsageUntil(t, h.CodexUsageCache, fetchedAt, fetchedAt.Add(5*time.Hour), "cred-a", testCodexUsageSnapshot(0.10, 0.10, "2026-05-13T11:00:00Z", "2026-05-18T00:00:00Z", "available", "available"))
	seedFreshCodexUsageUntil(t, h.CodexUsageCache, fetchedAt, fetchedAt.Add(5*time.Hour), "cred-b", testCodexUsageSnapshot(0.20, 0.10, "2026-05-13T10:00:00Z", "2026-05-19T00:00:00Z", "available", "available"))
	params := config.TianjiParams{
		Model:                           "openai/gpt-5.5",
		OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"},
	}

	first, err := h.resolveOpenAISubscriptionCredential(context.Background(), params, openAISubscriptionTransportChatGPTCodexBackend)
	require.NoError(t, err)
	require.Equal(t, "cred-a", first.CredentialID)

	now = fetchedAt.Add(11 * time.Second)
	second, err := h.resolveOpenAISubscriptionCredential(context.Background(), params, openAISubscriptionTransportChatGPTCodexBackend)
	require.NoError(t, err)

	assert.Equal(t, "cred-a", second.CredentialID)
	require.Len(t, fetcher.calls, 0)
}

func TestOpenAISubscriptionRouting_StickyCodexPrimaryWindowResetReevaluates(t *testing.T) {
	now := time.Date(2026, 5, 13, 6, 0, 0, 0, time.UTC)
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategySticky
	h.openAISubscriptionNow = func() time.Time { return now }
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	params := config.TianjiParams{
		Model:                           "openai/gpt-5.5",
		OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"},
	}

	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-a", testCodexUsageSnapshot(0.10, 0.20, "2026-05-13T10:00:00Z", "2026-05-18T00:00:00Z", "available", "available"))
	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-b", testCodexUsageSnapshot(0.10, 0.20, "2026-05-13T10:00:00Z", "2026-05-19T00:00:00Z", "available", "available"))

	first, err := h.resolveOpenAISubscriptionCredential(context.Background(), params, openAISubscriptionTransportChatGPTCodexBackend)
	require.NoError(t, err)
	require.Equal(t, "cred-a", first.CredentialID)

	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-a", testCodexUsageSnapshot(0.10, 0.20, "2026-05-13T15:00:00Z", "2026-05-20T00:00:00Z", "available", "available"))
	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-b", testCodexUsageSnapshot(0.10, 0.20, "2026-05-13T10:00:00Z", "2026-05-18T00:00:00Z", "available", "available"))

	second, err := h.resolveOpenAISubscriptionCredential(context.Background(), params, openAISubscriptionTransportChatGPTCodexBackend)
	require.NoError(t, err)

	assert.Equal(t, "cred-b", second.CredentialID)
}

func TestOpenAISubscriptionRouting_StickyCodexLegacySeedBackfillsMetadataForPrimaryReset(t *testing.T) {
	now := time.Date(2026, 5, 13, 6, 0, 0, 0, time.UTC)
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-a", AccountID: "acct-a"},
		"cred-b": {AccessToken: "access-b", AccountID: "acct-b"},
	})
	h.Config.NativeUpstreamStrategy = config.StrategySticky
	h.openAISubscriptionNow = func() time.Time { return now }
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	params := config.TianjiParams{
		Model:                           "openai/gpt-5.5",
		OpenAISubscriptionCredentialIDs: []string{"cred-a", "cred-b"},
	}
	h.openAISubscriptionSticky = map[string]string{
		openAISubscriptionRouteKey(context.Background(), params): "cred-a",
	}

	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-a", testCodexUsageSnapshot(0.10, 0.20, "2026-05-13T10:00:00Z", "2026-05-18T00:00:00Z", "available", "available"))
	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-b", testCodexUsageSnapshot(0.10, 0.20, "2026-05-13T10:00:00Z", "2026-05-19T00:00:00Z", "available", "available"))

	first, err := h.resolveOpenAISubscriptionCredential(context.Background(), params, openAISubscriptionTransportChatGPTCodexBackend)
	require.NoError(t, err)
	require.Equal(t, "cred-a", first.CredentialID)

	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-a", testCodexUsageSnapshot(0.10, 0.20, "2026-05-13T15:00:00Z", "2026-05-20T00:00:00Z", "available", "available"))
	seedFreshCodexUsage(t, h.CodexUsageCache, now, "cred-b", testCodexUsageSnapshot(0.10, 0.20, "2026-05-13T10:00:00Z", "2026-05-18T00:00:00Z", "available", "available"))

	second, err := h.resolveOpenAISubscriptionCredential(context.Background(), params, openAISubscriptionTransportChatGPTCodexBackend)
	require.NoError(t, err)

	assert.Equal(t, "cred-b", second.CredentialID)
}

func testCodexUsageSnapshot(primary, weekly float64, primaryReset, weeklyReset, primaryStatus, weeklyStatus string) chatgptcodex.UsageSnapshot {
	const primaryWindowSec int64 = 18000
	const weeklyWindowSec int64 = 604800
	allowed := primaryStatus == "available" && weeklyStatus == "available"
	limitReached := !allowed
	primaryResetAfter := resetAfterSeconds(primaryReset, time.Date(2026, 5, 13, 6, 0, 0, 0, time.UTC))
	weeklyResetAfter := resetAfterSeconds(weeklyReset, time.Date(2026, 5, 13, 6, 0, 0, 0, time.UTC))
	primaryWindow := chatgptcodex.UsageWindow{
		Name:               "primary_5h",
		UsedPercent:        float64Ptr(primary),
		ResetAt:            primaryReset,
		ResetAfterSeconds:  &primaryResetAfter,
		LimitWindowSeconds: int64Ptr(primaryWindowSec),
		Status:             primaryStatus,
	}
	weeklyWindow := chatgptcodex.UsageWindow{
		Name:               "weekly_7d",
		UsedPercent:        float64Ptr(weekly),
		ResetAt:            weeklyReset,
		ResetAfterSeconds:  &weeklyResetAfter,
		LimitWindowSeconds: int64Ptr(weeklyWindowSec),
		Status:             weeklyStatus,
	}
	return chatgptcodex.UsageSnapshot{
		RateLimit: &chatgptcodex.UsageRateLimit{
			Allowed:         &allowed,
			LimitReached:    &limitReached,
			PrimaryWindow:   primaryWindow,
			SecondaryWindow: weeklyWindow,
		},
		PrimaryWindow: primaryWindow,
		WeeklyWindow:  weeklyWindow,
	}
}

func resetAfterSeconds(reset string, now time.Time) int64 {
	parsed := callback.ParseResetTime(reset)
	if parsed.IsZero() {
		if rfc3339, err := time.Parse(time.RFC3339, reset); err == nil {
			parsed = rfc3339
		}
	}
	if parsed.IsZero() {
		return 1
	}
	seconds := int64(parsed.Sub(now).Seconds())
	if seconds <= 0 {
		return 1
	}
	return seconds
}

func seedFreshCodexUsage(t *testing.T, cache *OpenAISubscriptionCodexUsageCache, now time.Time, credentialID string, snapshot chatgptcodex.UsageSnapshot) {
	t.Helper()
	seedFreshCodexUsageUntil(t, cache, now, now.Add(time.Minute), credentialID, snapshot)
}

func seedFreshCodexUsageUntil(t *testing.T, cache *OpenAISubscriptionCodexUsageCache, fetchedAt, expiresAt time.Time, credentialID string, snapshot chatgptcodex.UsageSnapshot) {
	t.Helper()
	cache.putSuccess(credentialID, OpenAISubscriptionCodexUsageResult{
		CredentialID: credentialID,
		Status:       "fresh",
		FetchedAt:    fetchedAt,
		ExpiresAt:    expiresAt,
		Snapshot:     snapshot,
	})
}

func float64Ptr(v float64) *float64 {
	return &v
}

func int64Ptr(v int64) *int64 {
	return &v
}

type blockingCodexUsageFetcher struct {
	unblock  chan struct{}
	once     sync.Once
	mu       sync.Mutex
	started  int
	finished int
}

func newBlockingCodexUsageFetcher() *blockingCodexUsageFetcher {
	return &blockingCodexUsageFetcher{unblock: make(chan struct{})}
}

func (f *blockingCodexUsageFetcher) Fetch(ctx context.Context, _ chatgptcodex.UsageRequest) (chatgptcodex.UsageSnapshot, error) {
	f.mu.Lock()
	f.started++
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		f.finished++
		f.mu.Unlock()
	}()
	select {
	case <-f.unblock:
		now := time.Now()
		return testCodexUsageSnapshot(0.10, 0.10, now.Add(time.Hour).Format(time.RFC3339), now.Add(24*time.Hour).Format(time.RFC3339), "available", "available"), nil
	case <-ctx.Done():
		return chatgptcodex.UsageSnapshot{}, ctx.Err()
	}
}

func (f *blockingCodexUsageFetcher) startedCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.started
}

func (f *blockingCodexUsageFetcher) finishedCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.finished
}

func (f *blockingCodexUsageFetcher) release() {
	f.once.Do(func() { close(f.unblock) })
}

type openAISubscriptionTestCredential struct {
	AccessToken    string
	RefreshToken   string
	AccountID      string
	Status         string
	DisabledReason string
	Malformed      bool
}

func newOpenAISubscriptionRoutingHarness(t *testing.T, fixtures map[string]openAISubscriptionTestCredential) *Handlers {
	t.Helper()
	masterKey := openAISubscriptionRefreshTestMasterKey
	mu := sync.Mutex{}
	credentials := make(map[string]db.CredentialTable, len(fixtures))
	for id, fixture := range fixtures {
		status := fixture.Status
		if status == "" {
			status = "active"
		}
		info, err := json.Marshal(OpenAISubscriptionCredentialInfo{Status: status, DisabledReason: fixture.DisabledReason})
		require.NoError(t, err)
		value := "not-json"
		if !fixture.Malformed {
			refreshToken := fixture.RefreshToken
			if refreshToken == "" {
				refreshToken = "refresh-" + id
			}
			value = encryptOpenAITokenBundle(t, OpenAISubscriptionTokenBundle{
				AccessToken:  fixture.AccessToken,
				RefreshToken: refreshToken,
				ExpiresAt:    time.Now().Add(time.Hour),
				AccountID:    fixture.AccountID,
			}, masterKey)
		}
		credentials[id] = db.CredentialTable{
			CredentialID:    id,
			CredentialType:  CredentialTypeOpenAISubscription,
			CredentialValue: value,
			CredentialInfo:  info,
		}
	}
	m := newMockStore()
	m.listCredentialsFn = func(_ context.Context) ([]db.CredentialTable, error) {
		mu.Lock()
		defer mu.Unlock()
		result := make([]db.CredentialTable, 0, len(credentials))
		for _, credential := range credentials {
			result = append(result, credential)
		}
		return result, nil
	}
	m.getCredentialFn = func(_ context.Context, id string) (db.CredentialTable, error) {
		mu.Lock()
		defer mu.Unlock()
		credential, ok := credentials[id]
		if !ok {
			return db.CredentialTable{}, pgx.ErrNoRows
		}
		return credential, nil
	}
	m.updateCredentialValueAndInfoFn = func(_ context.Context, arg db.UpdateCredentialValueAndInfoParams) error {
		mu.Lock()
		defer mu.Unlock()
		credential, ok := credentials[arg.CredentialID]
		if !ok {
			return pgx.ErrNoRows
		}
		credential.CredentialValue = arg.CredentialValue
		credential.CredentialInfo = arg.CredentialInfo
		credentials[arg.CredentialID] = credential
		return nil
	}
	m.updateCredentialInfoFn = func(_ context.Context, arg db.UpdateCredentialInfoParams) error {
		mu.Lock()
		defer mu.Unlock()
		credential, ok := credentials[arg.CredentialID]
		if !ok {
			return pgx.ErrNoRows
		}
		credential.CredentialInfo = arg.CredentialInfo
		credentials[arg.CredentialID] = credential
		return nil
	}
	h := mockHandlers(m)
	h.Config.GeneralSettings.MasterKey = masterKey
	h.CodexUsageFetcher = &fakeCodexUsageFetcher{}
	return h
}
