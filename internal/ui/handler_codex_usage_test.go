package ui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/praxisllmlab/tianjiLLM/internal/auth"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/chatgptcodex"
	proxyhandler "github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/praxisllmlab/tianjiLLM/internal/ui/components/toast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodexUsageSelectionScore_UsesUpstreamRateLimit(t *testing.T) {
	allowed := true
	limitReached := false
	score := codexUsageSelectionScore(proxyhandler.OpenAISubscriptionCodexUsageResult{
		Snapshot: chatgptcodex.UsageSnapshot{
			RateLimit: &chatgptcodex.UsageRateLimit{
				Allowed:      &allowed,
				LimitReached: &limitReached,
				PrimaryWindow: chatgptcodex.UsageWindow{
					UsedPercent:        float64Ptr(0.15),
					ResetAfterSeconds:  int64Ptr(9945),
					LimitWindowSeconds: int64Ptr(18000),
				},
				SecondaryWindow: chatgptcodex.UsageWindow{
					UsedPercent:        float64Ptr(0.72),
					ResetAfterSeconds:  int64Ptr(346814),
					LimitWindowSeconds: int64Ptr(604800),
				},
			},
		},
	}, 0.9, chatgptcodex.DefaultSecondaryGate, time.Time{})

	require.NotNil(t, score)
	assert.InDelta(t, 2.8679, *score, 0.001)
}

func TestCodexUsageSelectionScore_IgnoresNormalizedOnlySnapshot(t *testing.T) {
	score := codexUsageSelectionScore(proxyhandler.OpenAISubscriptionCodexUsageResult{
		Snapshot: chatgptcodex.UsageSnapshot{
			PrimaryWindow: chatgptcodex.UsageWindow{UsedPercent: float64Ptr(0)},
			WeeklyWindow:  chatgptcodex.UsageWindow{UsedPercent: float64Ptr(1)},
		},
	}, 0.9, chatgptcodex.DefaultSecondaryGate, time.Time{})

	assert.Nil(t, score)
}

func TestCodexUsageSelectionScore_UsesConfiguredWeeklyGate(t *testing.T) {
	allowed := true
	limitReached := false
	result := proxyhandler.OpenAISubscriptionCodexUsageResult{
		Snapshot: chatgptcodex.UsageSnapshot{
			RateLimit: &chatgptcodex.UsageRateLimit{
				Allowed:      &allowed,
				LimitReached: &limitReached,
				PrimaryWindow: chatgptcodex.UsageWindow{
					UsedPercent:        float64Ptr(0.15),
					ResetAfterSeconds:  int64Ptr(9945),
					LimitWindowSeconds: int64Ptr(18000),
				},
				SecondaryWindow: chatgptcodex.UsageWindow{
					UsedPercent:        float64Ptr(0.72),
					ResetAfterSeconds:  int64Ptr(346814),
					LimitWindowSeconds: int64Ptr(604800),
				},
			},
		},
	}

	defaultScore := codexUsageSelectionScore(result, 0.9, chatgptcodex.DefaultSecondaryGate, time.Time{})
	customScore := codexUsageSelectionScore(result, 0.9, 0.8, time.Time{})

	require.NotNil(t, defaultScore)
	require.NotNil(t, customScore)
	assert.Greater(t, *customScore, *defaultScore)
}

func TestCodexUsageCardForCredential_SelectionScoreUsesRateLimit(t *testing.T) {
	allowed := true
	limitReached := false
	h := &UIHandler{}
	now := time.Date(2026, 5, 15, 6, 0, 0, 0, time.UTC)

	card := h.codexUsageCardForCredential(db.CredentialTable{
		CredentialID:   "cred-a",
		CredentialName: "Codex A",
	}, proxyhandler.OpenAISubscriptionCodexUsageResult{
		Status:    "fresh",
		FetchedAt: now,
		Snapshot: chatgptcodex.UsageSnapshot{
			PrimaryWindow: chatgptcodex.UsageWindow{UsedPercent: float64Ptr(0)},
			WeeklyWindow:  chatgptcodex.UsageWindow{UsedPercent: float64Ptr(1)},
			RateLimit: &chatgptcodex.UsageRateLimit{
				Allowed:      &allowed,
				LimitReached: &limitReached,
				PrimaryWindow: chatgptcodex.UsageWindow{
					UsedPercent:        float64Ptr(0.15),
					ResetAfterSeconds:  int64Ptr(9945),
					LimitWindowSeconds: int64Ptr(18000),
				},
				SecondaryWindow: chatgptcodex.UsageWindow{
					UsedPercent:        float64Ptr(0.72),
					ResetAfterSeconds:  int64Ptr(346814),
					LimitWindowSeconds: int64Ptr(604800),
				},
			},
		},
	}, now)

	require.NotNil(t, card.SelectionScore)
	assert.InDelta(t, 2.8679, *card.SelectionScore, 0.001)
}

func TestCodexUsageCardForCredential_SelectionScoreUsesConfiguredWeeklyGate(t *testing.T) {
	allowed := true
	limitReached := false
	h := &UIHandler{Config: &config.ProxyConfig{CodexUsageWeeklyThreshold: 0.8}}
	now := time.Date(2026, 5, 15, 6, 0, 0, 0, time.UTC)

	card := h.codexUsageCardForCredential(db.CredentialTable{
		CredentialID:   "cred-a",
		CredentialName: "Codex A",
	}, proxyhandler.OpenAISubscriptionCodexUsageResult{
		Status:    "fresh",
		FetchedAt: now,
		Snapshot: chatgptcodex.UsageSnapshot{
			RateLimit: &chatgptcodex.UsageRateLimit{
				Allowed:      &allowed,
				LimitReached: &limitReached,
				PrimaryWindow: chatgptcodex.UsageWindow{
					UsedPercent:        float64Ptr(0.15),
					ResetAfterSeconds:  int64Ptr(9945),
					LimitWindowSeconds: int64Ptr(18000),
				},
				SecondaryWindow: chatgptcodex.UsageWindow{
					UsedPercent:        float64Ptr(0.72),
					ResetAfterSeconds:  int64Ptr(346814),
					LimitWindowSeconds: int64Ptr(604800),
				},
			},
		},
	}, now)

	require.NotNil(t, card.SelectionScore)
	assert.Greater(t, *card.SelectionScore, 2.8679)
}

func TestCodexUsageCardForCredential_MovesSevenDayPrimaryWindowToWeeklyDisplay(t *testing.T) {
	h := &UIHandler{}
	now := time.Date(2026, 7, 13, 13, 25, 0, 0, time.UTC)
	used := 0.76
	windowSeconds := int64(7 * 24 * 60 * 60)

	card := h.codexUsageCardForCredential(db.CredentialTable{
		CredentialID:   "cred-a",
		CredentialName: "Codex A",
	}, proxyhandler.OpenAISubscriptionCodexUsageResult{
		Status:    "fresh",
		FetchedAt: now,
		Snapshot: chatgptcodex.UsageSnapshot{
			PrimaryWindow: chatgptcodex.UsageWindow{
				UsedPercent:        &used,
				ResetAt:            now.Add(6*24*time.Hour + 5*time.Hour).Format(time.RFC3339),
				LimitWindowSeconds: &windowSeconds,
			},
		},
	}, now)

	assert.Nil(t, card.PrimaryWindow.UsedPercent)
	require.NotNil(t, card.WeeklyWindow.UsedPercent)
	assert.InDelta(t, 0.76, *card.WeeklyWindow.UsedPercent, 0.0001)
	assert.Equal(t, "Weekly", card.WeeklyWindow.Name)
}

func TestCodexUsageCardForCredential_KeepsFiveHourPrimaryWindowAsCurrentSession(t *testing.T) {
	h := &UIHandler{}
	now := time.Date(2026, 7, 13, 13, 25, 0, 0, time.UTC)
	used := 0.25
	windowSeconds := int64(5 * 60 * 60)

	card := h.codexUsageCardForCredential(db.CredentialTable{
		CredentialID:   "cred-a",
		CredentialName: "Codex A",
	}, proxyhandler.OpenAISubscriptionCodexUsageResult{
		Status:    "fresh",
		FetchedAt: now,
		Snapshot: chatgptcodex.UsageSnapshot{
			PrimaryWindow: chatgptcodex.UsageWindow{
				UsedPercent:        &used,
				LimitWindowSeconds: &windowSeconds,
			},
		},
	}, now)

	require.NotNil(t, card.PrimaryWindow.UsedPercent)
	assert.InDelta(t, 0.25, *card.PrimaryWindow.UsedPercent, 0.0001)
	assert.Nil(t, card.WeeklyWindow.UsedPercent)
}

func TestCodexUsageCardForCredential_MapsGlobalAndAdditionalWindows(t *testing.T) {
	h := &UIHandler{}
	now := time.Date(2026, 8, 20, 13, 25, 0, 0, time.UTC)
	globalPrimary := chatgptcodex.UsageWindow{
		UsedPercent:        float64Ptr(0.04),
		LimitWindowSeconds: int64Ptr(604800),
	}
	sparkPrimary := chatgptcodex.UsageWindow{
		UsedPercent:        float64Ptr(0),
		LimitWindowSeconds: int64Ptr(18000),
	}
	sparkWeekly := chatgptcodex.UsageWindow{
		UsedPercent:        float64Ptr(0),
		LimitWindowSeconds: int64Ptr(604800),
	}

	card := h.codexUsageCardForCredential(db.CredentialTable{
		CredentialID:   "cred-a",
		CredentialName: "Codex A",
	}, proxyhandler.OpenAISubscriptionCodexUsageResult{
		Status: "fresh",
		Snapshot: chatgptcodex.UsageSnapshot{
			PrimaryWindow: globalPrimary,
			AdditionalBuckets: []chatgptcodex.UsageBucket{{
				Name:          "GPT-5.3-Codex-Spark",
				PrimaryWindow: sparkPrimary,
				WeeklyWindow:  sparkWeekly,
			}},
		},
	}, now)

	assert.Nil(t, card.PrimaryWindow.UsedPercent)
	require.NotNil(t, card.WeeklyWindow.UsedPercent)
	assert.InDelta(t, 0.04, *card.WeeklyWindow.UsedPercent, 0.0001)
	require.Len(t, card.AdditionalBuckets, 1)
	assert.InDelta(t, 0, *card.AdditionalBuckets[0].PrimaryWindow.UsedPercent, 0.0001)
	assert.InDelta(t, 0, *card.AdditionalBuckets[0].WeeklyWindow.UsedPercent, 0.0001)
	assert.Equal(t, "5-hour limit", card.AdditionalBuckets[0].PrimaryWindow.Name)
	assert.Equal(t, "Weekly limit", card.AdditionalBuckets[0].WeeklyWindow.Name)
}

func TestCodexUsageCardForCredential_MapsResetCreditsAvailableCount(t *testing.T) {
	availableCount := 0
	h := &UIHandler{}
	now := time.Date(2026, 5, 15, 6, 0, 0, 0, time.UTC)

	card := h.codexUsageCardForCredential(db.CredentialTable{
		CredentialID:   "cred-a",
		CredentialName: "Codex A",
	}, proxyhandler.OpenAISubscriptionCodexUsageResult{
		Status:    "fresh",
		FetchedAt: now,
		Snapshot: chatgptcodex.UsageSnapshot{
			RateLimitResetCredits: &chatgptcodex.UsageResetCredits{
				AvailableCount: &availableCount,
			},
		},
	}, now)

	require.NotNil(t, card.ResetCreditsAvailable)
	assert.Equal(t, 0, *card.ResetCreditsAvailable)
}

func TestCodexUsageCardForCredential_MapsLifecycleAction(t *testing.T) {
	h := &UIHandler{}
	now := time.Date(2026, 5, 15, 6, 0, 0, 0, time.UTC)

	activeCard := h.codexUsageCardForCredential(db.CredentialTable{
		CredentialID:    "cred-active",
		CredentialName:  "Codex Active",
		CredentialInfo:  []byte(`{"status":"active"}`),
		CredentialValue: "redacted",
	}, proxyhandler.OpenAISubscriptionCodexUsageResult{}, now)

	assert.Equal(t, "disable", activeCard.LifecycleAction)
	assert.Equal(t, "Disable", activeCard.LifecycleActionLabel)
	assert.Equal(t, "Disable this OpenAI subscription credential?", activeCard.LifecycleActionPrompt)

	disabledCard := h.codexUsageCardForCredential(db.CredentialTable{
		CredentialID:    "cred-disabled",
		CredentialName:  "Codex Disabled",
		CredentialInfo:  []byte(`{"status":"disabled","disabled_reason":"operator_disabled"}`),
		CredentialValue: "redacted",
	}, proxyhandler.OpenAISubscriptionCodexUsageResult{}, now)

	assert.Equal(t, "enable", disabledCard.LifecycleAction)
	assert.Equal(t, "Enable", disabledCard.LifecycleActionLabel)
	assert.Empty(t, disabledCard.LifecycleActionPrompt)
}

func TestCodexUsageResetMessageMapsOutcomes(t *testing.T) {
	assert.Equal(t, "Codex usage reset consumed (2 window(s) reset)", codexUsageResetMessage(proxyhandler.OpenAISubscriptionCodexUsageResetResult{
		Code:         "reset",
		WindowsReset: 2,
	}))
	assert.Equal(t, "Codex usage reset: nothing to reset", codexUsageResetMessage(proxyhandler.OpenAISubscriptionCodexUsageResetResult{
		Code: "nothing_to_reset",
	}))
	assert.Equal(t, "Codex usage reset failed: no reset credits available", codexUsageResetMessage(proxyhandler.OpenAISubscriptionCodexUsageResetResult{
		Code: "no_credit",
	}))
	assert.Equal(t, "Codex usage reset consumed (2 window(s) reset), but usage refresh failed: upstream_request_failed", codexUsageResetMessage(proxyhandler.OpenAISubscriptionCodexUsageResetResult{
		Code:            "reset",
		WindowsReset:    2,
		LastErrorReason: "upstream_request_failed",
	}))
	assert.Equal(t, toast.VariantWarning, codexUsageResetToastVariant(proxyhandler.OpenAISubscriptionCodexUsageResetResult{
		Code:            "reset",
		LastErrorReason: "upstream_request_failed",
	}))
	assert.Equal(t, toast.VariantError, codexUsageResetToastVariant(proxyhandler.OpenAISubscriptionCodexUsageResetResult{
		LastErrorReason: "auth_error",
	}))
	assert.Equal(t, toast.VariantError, codexUsageResetToastVariant(proxyhandler.OpenAISubscriptionCodexUsageResetResult{Code: "no_credit"}))
	assert.Equal(t, toast.VariantDefault, codexUsageResetToastVariant(proxyhandler.OpenAISubscriptionCodexUsageResetResult{Code: "nothing_to_reset"}))
}

func TestHandleCodexUsageAPI_RefreshesStaleUsageOnEntry(t *testing.T) {
	masterKey := "sk-ui-codex-usage-test"
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	encrypted := encryptedOpenAISubscriptionTokenBundle(t, masterKey, proxyhandler.OpenAISubscriptionTokenBundle{
		AccessToken:  "access-ui",
		RefreshToken: "refresh-ui",
		ExpiresAt:    time.Now().Add(time.Hour),
		AccountID:    "acct_ui",
	})
	info := []byte(`{"status":"active"}`)
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	rows := pgxmock.NewRows(credentialColumns()).
		AddRow("cred-ui", "Codex UI", openAISubscriptionCredentialType, encrypted, info, nil, now, "tester", now, "tester")
	mock.ExpectQuery(`SELECT .+ FROM "CredentialTable" ORDER BY created_at DESC`).WillReturnRows(rows)
	mock.ExpectQuery(`SELECT .+ FROM "CredentialTable" WHERE credential_id = \$1`).
		WithArgs("cred-ui").
		WillReturnRows(pgxmock.NewRows(credentialColumns()).
			AddRow("cred-ui", "Codex UI", openAISubscriptionCredentialType, encrypted, info, nil, now, "tester", now, "tester"))

	availableCount := 3
	fetcher := &uiCodexUsageFetcher{snapshot: chatgptcodex.UsageSnapshot{
		Email:         "operator@example.com",
		PlanType:      "pro",
		PrimaryWindow: chatgptcodex.UsageWindow{UsedPercent: float64Ptr(0.11), ResetAt: time.Now().Add(time.Hour).Format(time.RFC3339), Status: "available"},
		RateLimitResetCredits: &chatgptcodex.UsageResetCredits{
			AvailableCount: &availableCount,
		},
	}}
	h := &UIHandler{
		DB:                db.New(mock),
		Config:            &config.ProxyConfig{GeneralSettings: config.GeneralSettings{MasterKey: masterKey}},
		CodexUsageFetcher: fetcher,
		CodexUsageCache:   proxyhandler.NewOpenAISubscriptionCodexUsageCache(),
	}
	req := httptest.NewRequest(http.MethodGet, "/ui/api/codex-usage", nil)
	w := httptest.NewRecorder()

	h.handleCodexUsageAPI(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	calls := fetcher.callsSnapshot()
	require.Len(t, calls, 1)
	assert.Equal(t, "access-ui", calls[0].AccessToken)
	assert.Equal(t, "acct_ui", calls[0].AccountID)
	assert.Contains(t, w.Body.String(), "operator@example.com")
	assert.Contains(t, w.Body.String(), `"ResetCreditsAvailable":3`)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestHandleCredentialDisable_FromCodexUsageTabRendersCodexUsageTab(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	orgID := "org-ui"
	activeInfo := []byte(`{"status":"active","email":"operator@example.com"}`)
	disabledInfo := []byte(`{"status":"disabled","disabled_reason":"operator_disabled","email":"operator@example.com"}`)

	mock.ExpectQuery(`SELECT .+ FROM "CredentialTable" WHERE credential_id = \$1`).
		WithArgs("cred-ui").
		WillReturnRows(pgxmock.NewRows(credentialColumns()).
			AddRow("cred-ui", "Codex UI", openAISubscriptionCredentialType, "redacted", activeInfo, &orgID, now, "tester", now, "tester"))
	mock.ExpectExec(`UPDATE "CredentialTable"`).
		WithArgs("cred-ui", pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectQuery(`SELECT .+ FROM "CredentialTable" ORDER BY created_at DESC`).
		WillReturnRows(pgxmock.NewRows(credentialColumns()).
			AddRow("cred-ui", "Codex UI", openAISubscriptionCredentialType, "redacted", disabledInfo, &orgID, now, "tester", now, "tester"))
	mock.ExpectQuery(`SELECT .+ FROM "CredentialTable" WHERE credential_id = \$1`).
		WithArgs("cred-ui").
		WillReturnRows(pgxmock.NewRows(credentialColumns()).
			AddRow("cred-ui", "Codex UI", openAISubscriptionCredentialType, "redacted", disabledInfo, &orgID, now, "tester", now, "tester"))

	h := &UIHandler{
		DB:              db.New(mock),
		CodexUsageCache: proxyhandler.NewOpenAISubscriptionCodexUsageCache(),
	}
	req := httptest.NewRequest(http.MethodPost, "/ui/credentials/cred-ui/disable", nil)
	req.Header.Set("HX-Target", "codex-usage-tab-content")
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("credential_id", "cred-ui")
	req = req.WithContext(contextWithChiRoute(req.Context(), routeCtx))
	w := httptest.NewRecorder()

	h.handleCredentialDisable(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := w.Body.String()
	assert.Contains(t, body, `id="codex-usage-tab-content"`)
	assert.NotContains(t, body, `id="credentials-table"`)
	assert.Contains(t, body, "Credential disabled")
	assert.Contains(t, body, "credential_disabled")
	assert.Contains(t, body, "Enable")
	assert.Contains(t, body, `/ui/credentials/cred-ui/enable`)
	assert.NotContains(t, body, `/ui/credentials/cred-ui/codex-usage/refresh`)
	require.NoError(t, mock.ExpectationsWereMet())
}

type uiCodexUsageFetcher struct {
	mu       sync.Mutex
	calls    []chatgptcodex.UsageRequest
	snapshot chatgptcodex.UsageSnapshot
}

func (f *uiCodexUsageFetcher) Fetch(_ context.Context, req chatgptcodex.UsageRequest) (chatgptcodex.UsageSnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, req)
	return f.snapshot, nil
}

func (f *uiCodexUsageFetcher) callsSnapshot() []chatgptcodex.UsageRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]chatgptcodex.UsageRequest(nil), f.calls...)
}

func encryptedOpenAISubscriptionTokenBundle(t *testing.T, masterKey string, bundle proxyhandler.OpenAISubscriptionTokenBundle) string {
	t.Helper()
	body, err := json.Marshal(bundle)
	require.NoError(t, err)
	encrypted, err := auth.Encrypt(string(body), masterKey)
	require.NoError(t, err)
	return encrypted
}

func credentialColumns() []string {
	return []string{"credential_id", "credential_name", "credential_type", "credential_value", "credential_info", "organization_id", "created_at", "created_by", "updated_at", "updated_by"}
}

func int64Ptr(v int64) *int64 {
	return &v
}

func float64Ptr(v float64) *float64 {
	return &v
}
