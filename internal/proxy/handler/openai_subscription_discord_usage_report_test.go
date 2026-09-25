package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/chatgptcodex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAISubscriptionDiscordUsageReport_RendersEveryCredentialAndAdditionalBuckets(t *testing.T) {
	now := time.Date(2026, 8, 14, 6, 10, 0, 0, time.UTC)
	credentials := map[string]db.CredentialTable{
		"cred-a": discordUsageReportCredential(t, now, "cred-a", OpenAISubscriptionCredentialInfo{Status: "active"}),
		"cred-z": discordUsageReportCredential(t, now, "cred-z", OpenAISubscriptionCredentialInfo{Status: "active"}),
	}
	rows := []db.CredentialTable{
		credentials["cred-z"],
		{CredentialID: "not-openai", CredentialType: "api_key"},
		credentials["cred-a"],
	}
	store := newMockStore()
	store.listCredentialsFn = func(context.Context) ([]db.CredentialTable, error) { return rows, nil }
	store.getCredentialFn = func(_ context.Context, credentialID string) (db.CredentialTable, error) {
		return credentials[credentialID], nil
	}

	alphaPrimaryUsed := 0.30
	alphaWeeklyUsed := 0.40
	tokenLikePrimaryUsed := 0.50
	tokenLikeWeeklyUsed := 0.60
	freshA := testCodexUsageSnapshot(0.10, 0.20, "2026-08-14T07:00:00Z", "2026-08-20T00:00:00Z", "available", "available")
	freshA.Email = "operator@example.com"
	freshA.PlanType = "pro"
	freshA.AdditionalBuckets = []chatgptcodex.UsageBucket{{
		Name: "zeta<script>Bearer sk-12345678</script>",
		PrimaryWindow: chatgptcodex.UsageWindow{
			UsedPercent: &tokenLikePrimaryUsed,
			ResetAt:     "2026-08-15T00:00:00Z",
			Status:      "available",
		},
		WeeklyWindow: chatgptcodex.UsageWindow{
			UsedPercent: &tokenLikeWeeklyUsed,
			ResetAt:     "2026-08-20T00:00:00Z",
			Status:      "available",
		},
	}, {
		Name: "Alpha/Model",
		PrimaryWindow: chatgptcodex.UsageWindow{
			UsedPercent: &alphaPrimaryUsed,
			ResetAt:     "2026-08-15T00:00:00Z",
			Status:      "available",
		},
		WeeklyWindow: chatgptcodex.UsageWindow{
			UsedPercent: &alphaWeeklyUsed,
			ResetAt:     "2026-08-20T00:00:00Z",
			Status:      "available",
		},
	}}
	freshZ := testCodexUsageSnapshot(0.25, 0.50, "2026-08-14T08:00:00Z", "2026-08-21T00:00:00Z", "available", "available")
	freshZ.PlanType = "plus"

	h := mockHandlers(store)
	h.Config = &config.ProxyConfig{}
	h.Config.GeneralSettings.MasterKey = openAISubscriptionRefreshTestMasterKey
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	h.CodexUsageFetcher = &fakeCodexUsageFetcher{responses: []fakeCodexUsageResponse{
		{snapshot: freshA},
		{snapshot: freshZ},
	}}
	h.openAISubscriptionNow = func() time.Time { return now }

	report := h.OpenAISubscriptionDiscordUsageReport(context.Background())

	require.Empty(t, report.FailureReason)
	require.Equal(t, 2, report.CredentialCount)
	content := strings.Join(report.Parts, "\n")
	assert.Contains(t, content, "operational snapshot")
	assert.Contains(t, content, "not billing")
	assert.Contains(t, content, "credential=cred-a")
	assert.Contains(t, content, "credential=cred-z")
	assert.Equal(t, 1, strings.Count(content, "credential=cred-a"))
	assert.Equal(t, 1, strings.Count(content, "credential=cred-z"))
	assert.Less(t, strings.Index(content, "credential=cred-a"), strings.Index(content, "credential=cred-z"))
	assert.Contains(t, content, "plan=pro")
	assert.Contains(t, content, "primary_5h=10.0%")
	assert.Contains(t, content, "weekly_7d=20.0%")
	assert.Contains(t, content, "additional_Alpha/Model_primary_5h=30.0%")
	assert.Contains(t, content, "additional_Alpha/Model_weekly_7d=40.0%")
	assert.Contains(t, content, "additional_zetascriptREDACTED/script_primary_5h=50.0%")
	assert.Contains(t, content, "additional_zetascriptREDACTED/script_weekly_7d=60.0%")
	assert.Less(t,
		strings.Index(content, "additional_Alpha/Model_primary_5h=30.0%"),
		strings.Index(content, "additional_zetascriptREDACTED/script_primary_5h=50.0%"),
	)
	assert.Less(t,
		strings.Index(content, "additional_Alpha/Model_weekly_7d=40.0%"),
		strings.Index(content, "additional_zetascriptREDACTED/script_weekly_7d=60.0%"),
	)
	assert.NotContains(t, content, "zeta<script>Bearer sk-12345678</script>")
	assert.NotContains(t, content, "additional=Alpha/Model=")
	assert.NotContains(t, content, "operator@example.com")
	assert.NotContains(t, content, "access-cred-a")
	assert.NotContains(t, content, "refresh-cred-a")

	rows = nil
	zero := h.OpenAISubscriptionDiscordUsageReport(context.Background())
	require.Empty(t, zero.FailureReason)
	assert.Equal(t, 0, zero.CredentialCount)
	assert.Len(t, zero.Parts, 1)
	assert.Contains(t, zero.Parts[0], "zero-credential report")

	longRows := make([]db.CredentialTable, 0, 50)
	longStore := newMockStore()
	for i := range 50 {
		credentialID := fmt.Sprintf("cred-%02d-%s", i, strings.Repeat("x", 44))
		longRows = append(longRows, db.CredentialTable{
			CredentialID:   credentialID,
			CredentialType: CredentialTypeOpenAISubscription,
		})
	}
	longStore.listCredentialsFn = func(context.Context) ([]db.CredentialTable, error) { return longRows, nil }
	longStore.getCredentialFn = func(_ context.Context, credentialID string) (db.CredentialTable, error) {
		for _, credential := range longRows {
			if credential.CredentialID == credentialID {
				return credential, nil
			}
		}
		return db.CredentialTable{}, pgx.ErrNoRows
	}
	longHandler := mockHandlers(longStore)
	longHandler.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	longHandler.openAISubscriptionNow = func() time.Time { return now }
	for _, credential := range longRows {
		snapshot := freshA
		snapshot.Email = "long-fixture@example.com"
		seedFreshCodexUsage(t, longHandler.CodexUsageCache, now, credential.CredentialID, snapshot)
	}

	longReport := longHandler.OpenAISubscriptionDiscordUsageReport(context.Background())

	require.Empty(t, longReport.FailureReason)
	assert.Equal(t, 50, longReport.CredentialCount)
	require.Greater(t, len(longReport.Parts), 1)
	for i, part := range longReport.Parts {
		assert.LessOrEqual(t, utf8.RuneCountInString(part), 2000)
		assert.Contains(t, part, fmt.Sprintf("part %d/%d", i+1, len(longReport.Parts)))
	}
	for _, credential := range longRows {
		assert.Equal(t, 1, strings.Count(strings.Join(longReport.Parts, "\n"), "credential="+credential.CredentialID))
	}
}

func TestOpenAISubscriptionDiscordUsageReport_MapsGlobalSevenDayPrimaryToWeekly(t *testing.T) {
	used := 0.04
	windowSeconds := int64(7 * 24 * 60 * 60)
	row := openAISubscriptionDiscordUsageReportRowText("cred-a", "fresh", "", chatgptcodex.UsageSnapshot{
		PrimaryWindow: chatgptcodex.UsageWindow{
			UsedPercent:        &used,
			LimitWindowSeconds: &windowSeconds,
		},
	})

	assert.Contains(t, row, "weekly_7d=4.0%")
	assert.NotContains(t, row, "primary_5h=4.0%")
}

func discordUsageReportCredential(t *testing.T, now time.Time, credentialID string, info OpenAISubscriptionCredentialInfo) db.CredentialTable {
	t.Helper()
	infoJSON, err := json.Marshal(info)
	require.NoError(t, err)
	return db.CredentialTable{
		CredentialID:   credentialID,
		CredentialName: "must-not-render-" + credentialID,
		CredentialType: CredentialTypeOpenAISubscription,
		CredentialValue: encryptOpenAITokenBundle(t, OpenAISubscriptionTokenBundle{
			AccessToken:  "access-" + credentialID,
			RefreshToken: "refresh-" + credentialID,
			ExpiresAt:    now.Add(time.Hour),
			AccountID:    "org-must-not-render",
		}, openAISubscriptionRefreshTestMasterKey),
		CredentialInfo: infoJSON,
	}
}

func TestOpenAISubscriptionDiscordUsageReport_UsesSafeStatusWithoutUpstreamForNonSelectableCredential(t *testing.T) {
	now := time.Date(2026, 8, 14, 6, 10, 0, 0, time.UTC)
	orgID := "org-must-not-render"
	credentials := map[string]db.CredentialTable{
		"a-backoff":               discordUsageReportCredential(t, now, "a-backoff", OpenAISubscriptionCredentialInfo{Status: "active"}),
		"b-disabled":              discordUsageReportCredential(t, now, "b-disabled", OpenAISubscriptionCredentialInfo{Status: "disabled", LastError: "Bearer secret.jwt.fixture"}),
		"c-gone":                  discordUsageReportCredential(t, now, "c-gone", OpenAISubscriptionCredentialInfo{Status: "active"}),
		"e-reconnect":             discordUsageReportCredential(t, now, "e-reconnect", OpenAISubscriptionCredentialInfo{Status: "refresh_failed", DisabledReason: string(OpenAISubscriptionCredentialReconnect)}),
		"f-stale":                 discordUsageReportCredential(t, now, "f-stale", OpenAISubscriptionCredentialInfo{Status: "active"}),
		"g-operator-disabled":     discordUsageReportCredential(t, now, "g-operator-disabled", OpenAISubscriptionCredentialInfo{Status: "refresh_failed", DisabledReason: "operator_disabled"}),
		"not-openai-subscription": {CredentialID: "not-openai-subscription", CredentialType: "api_key"},
	}
	malformed := discordUsageReportCredential(t, now, "d-malformed", OpenAISubscriptionCredentialInfo{Status: "active"})
	malformed.CredentialInfo = []byte(`{"email":`)
	credentials["d-malformed"] = malformed
	rows := make([]db.CredentialTable, 0, len(credentials))
	for _, credential := range credentials {
		credential.CredentialName = "credential-name-must-not-render"
		credential.OrganizationID = &orgID
		rows = append(rows, credential)
	}
	store := newMockStore()
	store.listCredentialsFn = func(context.Context) ([]db.CredentialTable, error) { return rows, nil }
	store.getCredentialFn = func(_ context.Context, credentialID string) (db.CredentialTable, error) {
		if credentialID == "c-gone" {
			return db.CredentialTable{}, pgx.ErrNoRows
		}
		return credentials[credentialID], nil
	}
	fetcher := &fakeCodexUsageFetcher{responses: []fakeCodexUsageResponse{
		{err: &chatgptcodex.UsageHTTPError{StatusCode: 429, Reason: "rate_limited"}},
		{err: &chatgptcodex.UsageHTTPError{StatusCode: 429, Reason: "rate_limited"}},
	}}
	h := mockHandlers(store)
	h.Config = &config.ProxyConfig{}
	h.Config.GeneralSettings.MasterKey = openAISubscriptionRefreshTestMasterKey
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	h.CodexUsageFetcher = fetcher
	h.CodexUsageBackoffJitter = func(time.Duration) time.Duration { return 0 }
	h.openAISubscriptionNow = func() time.Time { return now }
	staleSnapshot := testCodexUsageSnapshot(0.40, 0.60, "2026-08-14T07:00:00Z", "2026-08-20T00:00:00Z", "available", "available")
	staleSnapshot.Email = "owner@example.com"
	seedFreshCodexUsageUntil(t, h.CodexUsageCache, now.Add(-2*time.Minute), now.Add(time.Minute), "f-stale", staleSnapshot)
	seedFreshCodexUsage(t, h.CodexUsageCache, now, "c-gone", staleSnapshot)

	report := h.OpenAISubscriptionDiscordUsageReport(context.Background())

	require.Empty(t, report.FailureReason)
	assert.Equal(t, 7, report.CredentialCount)
	content := strings.Join(report.Parts, "\n")
	for _, expected := range []string{
		"credential=a-backoff | status=backoff",
		"credential=b-disabled | status=disabled",
		"credential=c-gone | status=unavailable",
		"credential=d-malformed | status=malformed",
		"credential=e-reconnect | status=reconnect_required",
		"credential=f-stale | status=stale",
		"credential=g-operator-disabled | status=disabled",
	} {
		assert.Contains(t, content, expected)
	}
	assert.Len(t, fetcher.callsSnapshot(), 2, "non-selectable and deleted credentials must not call the usage fetcher")
	for _, secret := range []string{
		"access-a-backoff",
		"refresh-a-backoff",
		"owner@example.com",
		"credential-name-must-not-render",
		"org-must-not-render",
		"secret.jwt.fixture",
		"https://discord.example.invalid/webhook",
	} {
		assert.NotContains(t, content, secret)
	}
}
