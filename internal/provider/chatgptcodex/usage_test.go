package chatgptcodex

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsageClientFetch_BuildsWhamUsageRequestAndNormalizesSnapshot(t *testing.T) {
	var gotPath string
	var gotHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"email": "operator@example.com",
			"plan_type": "pro",
			"primary": {"window": "5h", "used_percent": 6, "reset_at": "2026-05-13T10:00:00Z", "status": "available"},
			"weekly": {"used_percent": 14, "reset_at": "2026-05-18T00:00:00Z", "status": "available"},
			"additional_rate_limits": [{"name": "GPT-5.3-Codex-Spark", "used_percent": 0, "status": "available"}],
			"credits": {"status": "enabled"}
		}`))
	}))
	defer server.Close()

	client := UsageClient{BaseURL: server.URL, HTTPClient: server.Client()}
	snapshot, err := client.Fetch(context.Background(), UsageRequest{
		AccessToken: "access-secret",
		AccountID:   "acct_123",
	})

	require.NoError(t, err)
	assert.Equal(t, "/backend-api/wham/usage", gotPath)
	assert.Equal(t, "Bearer access-secret", gotHeaders.Get("Authorization"))
	assert.Equal(t, "acct_123", gotHeaders.Get("ChatGPT-Account-Id"))
	assert.Contains(t, gotHeaders.Get("Referer"), "/codex/settings/usage")
	assert.Equal(t, "operator@example.com", snapshot.Email)
	assert.Equal(t, "pro", snapshot.PlanType)
	assert.InDelta(t, 0.06, *snapshot.PrimaryWindow.UsedPercent, 0.0001)
	assert.InDelta(t, 0.14, *snapshot.WeeklyWindow.UsedPercent, 0.0001)
	require.Len(t, snapshot.AdditionalBuckets, 1)
	assert.Equal(t, "GPT-5.3-Codex-Spark", snapshot.AdditionalBuckets[0].Name)
	require.NotNil(t, snapshot.AdditionalBuckets[0].PrimaryWindow.UsedPercent)
	assert.InDelta(t, 0, *snapshot.AdditionalBuckets[0].PrimaryWindow.UsedPercent, 0.0001)
	assert.Equal(t, "available", snapshot.AdditionalBuckets[0].PrimaryWindow.Status)
	assert.Equal(t, "enabled", snapshot.CreditsStatus)
}

func TestUsageBucketUnmarshal_PreservesLegacyFieldsAndPrefersNewWindow(t *testing.T) {
	var legacy UsageBucket
	err := json.Unmarshal([]byte(`{
		"name":"legacy",
		"used_percent":0.20,
		"reset_at":"2026-08-20T07:30:00Z",
		"status":"available"
	}`), &legacy)
	require.NoError(t, err)
	require.NotNil(t, legacy.PrimaryWindow.UsedPercent)
	assert.InDelta(t, 0.20, *legacy.PrimaryWindow.UsedPercent, 0.0001)
	assert.Equal(t, "2026-08-20T07:30:00Z", legacy.PrimaryWindow.ResetAt)
	assert.Equal(t, "available", legacy.PrimaryWindow.Status)

	var current UsageBucket
	err = json.Unmarshal([]byte(`{
		"name":"current",
		"primary_window":{
			"used_percent":0.90,
			"reset_at":"2026-08-20T08:30:00Z",
			"status":"new"
		},
		"used_percent":0.20,
		"reset_at":"2026-08-20T07:30:00Z",
		"status":"legacy"
	}`), &current)
	require.NoError(t, err)
	require.NotNil(t, current.PrimaryWindow.UsedPercent)
	assert.InDelta(t, 0.90, *current.PrimaryWindow.UsedPercent, 0.0001)
	assert.Equal(t, "2026-08-20T08:30:00Z", current.PrimaryWindow.ResetAt)
	assert.Equal(t, "new", current.PrimaryWindow.Status)
}

func TestUsageClientFetch_NormalizesGlobalAndAdditionalWindows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"plan_type": "pro",
			"rate_limit": {
				"allowed": true,
				"limit_reached": false,
				"primary_window": {
					"used_percent": 4,
					"limit_window_seconds": 604800,
					"reset_after_seconds": 592249,
					"reset_at": 1787801789
				}
			},
			"additional_rate_limits": [
				{
					"limit_name": "GPT-5.3-Codex-Spark",
					"rate_limit": {
						"allowed": true,
						"limit_reached": false,
						"primary_window": {
							"used_percent": 0,
							"limit_window_seconds": 18000,
							"reset_after_seconds": 18000,
							"reset_at": 1787227541
						},
						"secondary_window": {
							"used_percent": 0,
							"limit_window_seconds": 604800,
							"reset_after_seconds": 604800,
							"reset_at": 1787814341
						}
					}
				}
			]
		}`))
	}))
	defer server.Close()

	client := UsageClient{BaseURL: server.URL, HTTPClient: server.Client()}
	snapshot, err := client.Fetch(context.Background(), UsageRequest{AccessToken: "access-secret"})

	require.NoError(t, err)
	require.NotNil(t, snapshot.PrimaryWindow.UsedPercent)
	assert.InDelta(t, 0.04, *snapshot.PrimaryWindow.UsedPercent, 0.0001)
	require.NotNil(t, snapshot.PrimaryWindow.LimitWindowSeconds)
	assert.Equal(t, int64(604800), *snapshot.PrimaryWindow.LimitWindowSeconds)
	assert.Nil(t, snapshot.WeeklyWindow.UsedPercent)
	require.Len(t, snapshot.AdditionalBuckets, 1)
	assert.Equal(t, "GPT-5.3-Codex-Spark", snapshot.AdditionalBuckets[0].Name)
	require.NotNil(t, snapshot.AdditionalBuckets[0].PrimaryWindow.UsedPercent)
	assert.InDelta(t, 0, *snapshot.AdditionalBuckets[0].PrimaryWindow.UsedPercent, 0.0001)
	require.NotNil(t, snapshot.AdditionalBuckets[0].PrimaryWindow.LimitWindowSeconds)
	assert.Equal(t, int64(18000), *snapshot.AdditionalBuckets[0].PrimaryWindow.LimitWindowSeconds)
	require.NotNil(t, snapshot.AdditionalBuckets[0].WeeklyWindow.UsedPercent)
	assert.InDelta(t, 0, *snapshot.AdditionalBuckets[0].WeeklyWindow.UsedPercent, 0.0001)
	require.NotNil(t, snapshot.AdditionalBuckets[0].WeeklyWindow.LimitWindowSeconds)
	assert.Equal(t, int64(604800), *snapshot.AdditionalBuckets[0].WeeklyWindow.LimitWindowSeconds)
}

func TestUsageClientFetch_PreservesWeeklyWhenAdditionalBucketsMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{
			"account": {"email": "operator@example.com", "plan_type": "pro"},
			"usage": {"weekly": {"utilization": 0.42, "status": "available"}}
		}`))
	}))
	defer server.Close()

	client := UsageClient{BaseURL: server.URL, HTTPClient: server.Client()}
	snapshot, err := client.Fetch(context.Background(), UsageRequest{AccessToken: "access-secret"})

	require.NoError(t, err)
	assert.Equal(t, "operator@example.com", snapshot.Email)
	assert.Equal(t, "pro", snapshot.PlanType)
	require.NotNil(t, snapshot.WeeklyWindow.UsedPercent)
	assert.InDelta(t, 0.42, *snapshot.WeeklyWindow.UsedPercent, 0.0001)
	assert.Empty(t, snapshot.AdditionalBuckets)
}

func TestUsageClientFetch_NormalizesWhamPrimaryAndSecondaryWindows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"email": "operator@example.com",
			"plan_type": "team",
				"rate_limit": {
					"allowed": true,
					"limit_reached": false,
					"primary_window": {
						"used_percent": 1,
						"limit_window_seconds": 18000,
						"reset_after_seconds": 9945,
						"reset_at": 1778786516
					},
					"secondary_window": {
						"used_percent": 4,
						"limit_window_seconds": 604800,
						"reset_after_seconds": 346814,
						"reset_at": 1779351602
					}
				},
			"rate_limit_reset_credits": {
				"available_count": 2
			},
			"additional_rate_limits": null
		}`))
	}))
	defer server.Close()

	client := UsageClient{BaseURL: server.URL, HTTPClient: server.Client()}
	snapshot, err := client.Fetch(context.Background(), UsageRequest{AccessToken: "access-secret"})

	require.NoError(t, err)
	require.NotNil(t, snapshot.PrimaryWindow.UsedPercent)
	assert.InDelta(t, 0.01, *snapshot.PrimaryWindow.UsedPercent, 0.0001)
	assert.Equal(t, "1778786516", snapshot.PrimaryWindow.ResetAt)
	require.NotNil(t, snapshot.PrimaryWindow.ResetAfterSeconds)
	assert.Equal(t, int64(9945), *snapshot.PrimaryWindow.ResetAfterSeconds)
	require.NotNil(t, snapshot.PrimaryWindow.LimitWindowSeconds)
	assert.Equal(t, int64(18000), *snapshot.PrimaryWindow.LimitWindowSeconds)
	require.NotNil(t, snapshot.WeeklyWindow.UsedPercent)
	assert.InDelta(t, 0.04, *snapshot.WeeklyWindow.UsedPercent, 0.0001)
	assert.Equal(t, "1779351602", snapshot.WeeklyWindow.ResetAt)
	require.NotNil(t, snapshot.RateLimit)
	require.NotNil(t, snapshot.RateLimit.Allowed)
	assert.True(t, *snapshot.RateLimit.Allowed)
	require.NotNil(t, snapshot.RateLimit.LimitReached)
	assert.False(t, *snapshot.RateLimit.LimitReached)
	require.NotNil(t, snapshot.RateLimitResetCredits)
	require.NotNil(t, snapshot.RateLimitResetCredits.AvailableCount)
	assert.Equal(t, 2, *snapshot.RateLimitResetCredits.AvailableCount)
	assert.Empty(t, snapshot.AdditionalBuckets)
}

func TestUsageClientConsumeResetCredit_BuildsWhamRequestAndParsesResult(t *testing.T) {
	var gotPath string
	var gotHeaders http.Header
	var gotBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotHeaders = r.Header.Clone()
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"reset","windows_reset":2}`))
	}))
	defer server.Close()

	client := UsageClient{BaseURL: server.URL, HTTPClient: server.Client()}
	result, err := client.ConsumeResetCredit(context.Background(), ResetCreditRequest{
		AccessToken:     "access-secret",
		AccountID:       "acct_123",
		RedeemRequestID: "redeem-123",
	})

	require.NoError(t, err)
	assert.Equal(t, "/backend-api/wham/rate-limit-reset-credits/consume", gotPath)
	assert.Equal(t, "Bearer access-secret", gotHeaders.Get("Authorization"))
	assert.Equal(t, "acct_123", gotHeaders.Get("ChatGPT-Account-Id"))
	assert.Contains(t, gotHeaders.Get("Referer"), "/codex/settings/usage")
	assert.Equal(t, "application/json", gotHeaders.Get("Content-Type"))
	assert.Equal(t, "redeem-123", gotBody["redeem_request_id"])
	assert.Equal(t, "reset", result.Code)
	assert.Equal(t, int64(2), result.WindowsReset)
}

func TestUsageClientConsumeResetCredit_ParsesAlternateOutcomeField(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"outcome":"no_credit"}`))
	}))
	defer server.Close()

	client := UsageClient{BaseURL: server.URL, HTTPClient: server.Client()}
	result, err := client.ConsumeResetCredit(context.Background(), ResetCreditRequest{
		AccessToken:     "access-secret",
		RedeemRequestID: "redeem-123",
	})

	require.NoError(t, err)
	assert.Equal(t, "no_credit", result.Code)
}

func TestUsageClientConsumeResetCredit_Non2xxReturnsUsageHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	defer server.Close()

	client := UsageClient{BaseURL: server.URL, HTTPClient: server.Client()}
	_, err := client.ConsumeResetCredit(context.Background(), ResetCreditRequest{
		AccessToken:     "access-secret",
		RedeemRequestID: "redeem-123",
	})

	var httpErr *UsageHTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusUnauthorized, httpErr.StatusCode)
	assert.Equal(t, "auth_error", httpErr.Reason)
}

func TestUsageClientConsumeResetCredit_RequiresRedeemRequestID(t *testing.T) {
	client := UsageClient{}
	_, err := client.ConsumeResetCredit(context.Background(), ResetCreditRequest{AccessToken: "access-secret"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "redeem_request_id required")
}

func TestUsageClientFetch_NormalizesAppServerResetCreditsCamelCase(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"rateLimitResetCredits": {
				"availableCount": 0
			},
			"rateLimits": {
				"limitId": "codex",
				"primary": {
					"usedPercent": 25,
					"resetsAt": 1730947200
				}
			}
		}`))
	}))
	defer server.Close()

	client := UsageClient{BaseURL: server.URL, HTTPClient: server.Client()}
	snapshot, err := client.Fetch(context.Background(), UsageRequest{AccessToken: "access-secret"})

	require.NoError(t, err)
	require.NotNil(t, snapshot.RateLimitResetCredits)
	require.NotNil(t, snapshot.RateLimitResetCredits.AvailableCount)
	assert.Equal(t, 0, *snapshot.RateLimitResetCredits.AvailableCount)
}

func TestSelectionScore_UsesUpstreamRateLimitOnly(t *testing.T) {
	allowed := true
	limitReached := false
	score, available, ok := SelectionScore(UsageSnapshot{
		RateLimit: &UsageRateLimit{
			Allowed:      &allowed,
			LimitReached: &limitReached,
			PrimaryWindow: UsageWindow{
				UsedPercent:        float64Ptr(0.15),
				ResetAfterSeconds:  int64Ptr(9945),
				LimitWindowSeconds: int64Ptr(18000),
			},
			SecondaryWindow: UsageWindow{
				UsedPercent:        float64Ptr(0.72),
				ResetAfterSeconds:  int64Ptr(346814),
				LimitWindowSeconds: int64Ptr(604800),
			},
		},
	}, 0.9)

	require.True(t, ok)
	assert.True(t, available)
	assert.InDelta(t, 2.8679, score, 0.001)
}

func TestSelectionScoreAtWithGates_UsesConfiguredSecondaryGate(t *testing.T) {
	allowed := true
	limitReached := false
	snapshot := UsageSnapshot{
		RateLimit: &UsageRateLimit{
			Allowed:      &allowed,
			LimitReached: &limitReached,
			PrimaryWindow: UsageWindow{
				UsedPercent:        float64Ptr(0.15),
				ResetAfterSeconds:  int64Ptr(9945),
				LimitWindowSeconds: int64Ptr(18000),
			},
			SecondaryWindow: UsageWindow{
				UsedPercent:        float64Ptr(0.72),
				ResetAfterSeconds:  int64Ptr(346814),
				LimitWindowSeconds: int64Ptr(604800),
			},
		},
	}

	defaultScore, defaultAvailable, defaultOK := SelectionScore(snapshot, 0.9)
	customScore, customAvailable, customOK := SelectionScoreAtWithGates(snapshot, 0.9, 0.8, time.Time{}, time.Time{})

	require.True(t, defaultOK)
	require.True(t, customOK)
	assert.True(t, defaultAvailable)
	assert.True(t, customAvailable)
	assert.Greater(t, customScore, defaultScore)
}

func TestSelectionScore_DefaultPrimaryGateIsCodexGate(t *testing.T) {
	allowed := true
	limitReached := false
	score, available, ok := SelectionScore(UsageSnapshot{
		RateLimit: &UsageRateLimit{
			Allowed:      &allowed,
			LimitReached: &limitReached,
			PrimaryWindow: UsageWindow{
				UsedPercent:        float64Ptr(0.4),
				ResetAfterSeconds:  int64Ptr(9000),
				LimitWindowSeconds: int64Ptr(18000),
			},
			SecondaryWindow: UsageWindow{
				UsedPercent:        float64Ptr(0),
				ResetAfterSeconds:  int64Ptr(0),
				LimitWindowSeconds: int64Ptr(604800),
			},
		},
	}, 0)

	require.True(t, ok)
	assert.True(t, available)
	assert.InDelta(t, 0.9, score, 0.001)
}

func TestSelectionScore_LimitReachedIsUnavailable(t *testing.T) {
	allowed := false
	limitReached := true
	score, available, ok := SelectionScore(UsageSnapshot{
		RateLimit: &UsageRateLimit{
			Allowed:      &allowed,
			LimitReached: &limitReached,
			PrimaryWindow: UsageWindow{
				UsedPercent:        float64Ptr(0),
				ResetAfterSeconds:  int64Ptr(18000),
				LimitWindowSeconds: int64Ptr(18000),
			},
			SecondaryWindow: UsageWindow{
				UsedPercent:        float64Ptr(1),
				ResetAfterSeconds:  int64Ptr(3934),
				LimitWindowSeconds: int64Ptr(604800),
			},
		},
	}, 0.9)

	require.True(t, ok)
	assert.False(t, available)
	assert.Equal(t, MaxSelectionScore, score)
}

func TestSelectionScore_MissingRateLimitFieldsUnknown(t *testing.T) {
	_, _, ok := SelectionScore(UsageSnapshot{
		PrimaryWindow: UsageWindow{UsedPercent: float64Ptr(0)},
		WeeklyWindow:  UsageWindow{UsedPercent: float64Ptr(1)},
	}, 0.9)

	assert.False(t, ok)
}

func TestUsageClientFetch_NonOKReturnsSafeHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "token access-secret leaked body", http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := UsageClient{BaseURL: server.URL, HTTPClient: server.Client()}
	_, err := client.Fetch(context.Background(), UsageRequest{AccessToken: "access-secret"})

	var httpErr *UsageHTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusTooManyRequests, httpErr.StatusCode)
	assert.Equal(t, "rate_limited", httpErr.Reason)
	assert.NotContains(t, err.Error(), "access-secret")
}

func float64Ptr(v float64) *float64 {
	return &v
}

func int64Ptr(v int64) *int64 {
	return &v
}
