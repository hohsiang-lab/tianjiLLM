package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/chatgptcodex"
)

const (
	defaultCodexUsageTTL             = 60 * time.Second
	defaultCodexUsageRefreshInterval = 10 * time.Second
	defaultCodexUsageAsyncTimeout    = 30 * time.Second
	defaultCodexUsageBackoffBase     = 60 * time.Second
)

const codexUsageCacheKeyPrefix = "openai_subscription_codex_usage:"

var errCodexUsageEmptySnapshot = errors.New("empty_snapshot")

type OpenAISubscriptionCodexUsageCache struct {
	mu      sync.Mutex
	entries map[string]openAISubscriptionCodexUsageCacheEntry
}

type openAISubscriptionCodexUsageCacheEntry struct {
	Result         OpenAISubscriptionCodexUsageResult
	ExpiresAt      time.Time
	BackoffUntil   time.Time
	BackoffAttempt int
}

type OpenAISubscriptionCodexUsageResult struct {
	CredentialID    string
	Status          string
	Snapshot        chatgptcodex.UsageSnapshot
	FetchedAt       time.Time
	ExpiresAt       time.Time
	BackoffUntil    time.Time
	LastErrorReason string
	RefreshDeferred bool
}

type OpenAISubscriptionCodexUsagePreloadResult struct {
	CredentialIDs int
	Cached        int
	Refreshed     int
	Failed        int
}

type OpenAISubscriptionCodexUsageResetResult struct {
	CredentialID    string
	Code            string
	WindowsReset    int64
	Usage           OpenAISubscriptionCodexUsageResult
	LastErrorReason string
}

type openAISubscriptionCodexUsageCacheValue struct {
	CredentialID    string                     `json:"credential_id"`
	Status          string                     `json:"status"`
	Snapshot        chatgptcodex.UsageSnapshot `json:"snapshot"`
	FetchedAt       time.Time                  `json:"fetched_at"`
	ExpiresAt       time.Time                  `json:"expires_at"`
	LastErrorReason string                     `json:"last_error_reason,omitempty"`
}

func NewOpenAISubscriptionCodexUsageCache() *OpenAISubscriptionCodexUsageCache {
	return &OpenAISubscriptionCodexUsageCache{entries: make(map[string]openAISubscriptionCodexUsageCacheEntry)}
}

func (h *Handlers) PreloadOpenAISubscriptionCodexUsage(ctx context.Context) OpenAISubscriptionCodexUsagePreloadResult {
	var result OpenAISubscriptionCodexUsagePreloadResult
	if h == nil {
		return result
	}
	credentialIDs := h.openAISubscriptionCodexUsagePreloadCredentialIDs(ctx)
	result.CredentialIDs = len(credentialIDs)
	if len(credentialIDs) == 0 {
		return result
	}

	now := h.openAISubscriptionNowUTC()
	h.hydrateCachedOpenAISubscriptionCodexUsageResults(ctx, credentialIDs, now)
	cache := h.openAISubscriptionCodexUsageCache()
	refreshInterval := h.codexUsageRefreshInterval()
	for _, credentialID := range credentialIDs {
		if _, ok := cache.getFresh(credentialID, now, refreshInterval); ok {
			result.Cached++
			continue
		}
		usageResult := h.OpenAISubscriptionCodexUsageSnapshot(ctx, credentialID, false)
		if usageResult.Status == "fresh" && codexUsageResultHasSnapshot(usageResult) {
			result.Refreshed++
			continue
		}
		result.Failed++
	}
	return result
}

func (h *Handlers) openAISubscriptionCodexUsagePreloadCredentialIDs(ctx context.Context) []string {
	for _, modelCfg := range h.runtimeModelList(ctx) {
		modelName := strings.TrimSpace(modelCfg.TianjiParams.Model)
		if modelName == "" {
			continue
		}
		providerName, _ := provider.ParseModelName(modelName)
		if providerName == "openai" {
			ids, err := h.openAISubscriptionCredentialIDsInRequestScope(ctx)
			if err != nil {
				return nil
			}
			return ids
		}
	}
	return nil
}

func (h *Handlers) OpenAISubscriptionCodexUsageSnapshot(ctx context.Context, credentialID string, manual bool) OpenAISubscriptionCodexUsageResult {
	credentialID = strings.TrimSpace(credentialID)
	now := h.openAISubscriptionNowUTC()
	if credentialID == "" {
		return openAISubscriptionCodexUsageError("", "credential_missing", now)
	}
	cache := h.openAISubscriptionCodexUsageCache()
	refreshInterval := h.codexUsageRefreshInterval()
	if !manual {
		if result, ok := cache.getFresh(credentialID, now, refreshInterval); ok {
			return result
		}
	}
	if result, ok := cache.getBackoff(credentialID, now); ok {
		result.RefreshDeferred = manual
		return result
	}

	resolved, err := h.resolveOpenAISubscriptionCredentialForUsageSnapshot(ctx, credentialID)
	if err != nil {
		result := openAISubscriptionCodexUsageError(credentialID, string(openAISubscriptionErrorCodeOrDefault(err)), now)
		cache.putError(credentialID, result)
		return result
	}
	if !manual {
		if result, ok := h.getCachedOpenAISubscriptionCodexUsageResult(ctx, credentialID, now); ok {
			cache.putSuccess(credentialID, result)
			if codexUsageFreshWithin(result, now, refreshInterval) {
				return result
			}
		}
	}

	value, err, _ := h.codexUsageRefreshGroup.Do(credentialID, func() (any, error) {
		return h.fetchOpenAISubscriptionCodexUsage(ctx, credentialID, resolved, now)
	})
	if err != nil {
		result := h.codexUsageErrorResult(credentialID, err, now)
		return cache.putFailure(credentialID, result, now, h.codexUsageBackoffBase(), h.codexUsageBackoffJitter)
	}
	result, ok := value.(OpenAISubscriptionCodexUsageResult)
	if !ok {
		errorResult := openAISubscriptionCodexUsageError(credentialID, "empty_snapshot", now)
		return cache.putFailure(credentialID, errorResult, now, h.codexUsageBackoffBase(), h.codexUsageBackoffJitter)
	}
	cache.putSuccess(credentialID, result)
	h.persistOpenAISubscriptionCodexUsageResult(ctx, result, now)
	return result
}

func (h *Handlers) OpenAISubscriptionCodexUsageReset(ctx context.Context, credentialID string) OpenAISubscriptionCodexUsageResetResult {
	credentialID = strings.TrimSpace(credentialID)
	now := h.openAISubscriptionNowUTC()
	result := OpenAISubscriptionCodexUsageResetResult{CredentialID: credentialID}
	if credentialID == "" {
		result.LastErrorReason = "credential_missing"
		result.Usage = openAISubscriptionCodexUsageError("", result.LastErrorReason, now)
		return result
	}

	resolved, err := h.resolveOpenAISubscriptionCredentialForUsageSnapshot(ctx, credentialID)
	if err != nil {
		result.LastErrorReason = string(openAISubscriptionErrorCodeOrDefault(err))
		result.Usage = openAISubscriptionCodexUsageError(credentialID, result.LastErrorReason, now)
		h.openAISubscriptionCodexUsageCache().putError(credentialID, result.Usage)
		return result
	}

	redeemRequestID := uuid.NewString()
	resetResult, resolved, err := h.consumeOpenAISubscriptionCodexResetCredit(ctx, credentialID, resolved, redeemRequestID)
	if err != nil {
		result.LastErrorReason = h.codexUsageErrorReason(err)
		result.Usage = h.codexUsageErrorResult(credentialID, err, now)
		if cached, ok, _ := h.openAISubscriptionCodexUsageCache().getUsableWithReason(credentialID, now); ok {
			result.Usage.Snapshot = cached.Snapshot
			result.Usage.FetchedAt = cached.FetchedAt
			result.Usage.ExpiresAt = cached.ExpiresAt
		}
		return result
	}

	result.Code = resetResult.Code
	result.WindowsReset = resetResult.WindowsReset
	usage, err := h.fetchOpenAISubscriptionCodexUsage(ctx, credentialID, resolved, now)
	if err != nil {
		result.LastErrorReason = h.codexUsageErrorReason(err)
		result.Usage = h.openAISubscriptionCodexUsageCache().putFailure(credentialID, h.codexUsageErrorResult(credentialID, err, now), now, h.codexUsageBackoffBase(), h.codexUsageBackoffJitter)
		return result
	}
	h.openAISubscriptionCodexUsageCache().putSuccess(credentialID, usage)
	h.persistOpenAISubscriptionCodexUsageResult(ctx, usage, now)
	result.Usage = usage
	return result
}

func (h *Handlers) fetchOpenAISubscriptionCodexUsage(ctx context.Context, credentialID string, resolved resolvedOpenAISubscriptionCredential, now time.Time) (OpenAISubscriptionCodexUsageResult, error) {
	snapshot, err := h.openAISubscriptionCodexUsageFetcher().Fetch(ctx, chatgptcodex.UsageRequest{
		AccessToken: resolved.BearerToken,
		AccountID:   resolved.AccountID,
	})
	if isCodexUsageHTTPStatus(err, http.StatusUnauthorized) {
		refreshed, refreshErr := h.refreshOpenAISubscriptionCredentialForUsageSnapshot(ctx, credentialID)
		if refreshErr != nil {
			return OpenAISubscriptionCodexUsageResult{}, refreshErr
		}
		snapshot, err = h.openAISubscriptionCodexUsageFetcher().Fetch(ctx, chatgptcodex.UsageRequest{
			AccessToken: refreshed.BearerToken,
			AccountID:   refreshed.AccountID,
		})
	}
	if err != nil {
		return OpenAISubscriptionCodexUsageResult{}, err
	}

	result := OpenAISubscriptionCodexUsageResult{
		CredentialID: credentialID,
		Status:       "fresh",
		Snapshot:     snapshot,
		FetchedAt:    now,
	}
	result.ExpiresAt = h.codexUsageExpiresAt(snapshot, now)
	if !codexUsageResultHasSnapshot(result) {
		return OpenAISubscriptionCodexUsageResult{}, errCodexUsageEmptySnapshot
	}
	return result, nil
}

func (h *Handlers) consumeOpenAISubscriptionCodexResetCredit(ctx context.Context, credentialID string, resolved resolvedOpenAISubscriptionCredential, redeemRequestID string) (chatgptcodex.ResetCreditResult, resolvedOpenAISubscriptionCredential, error) {
	req := chatgptcodex.ResetCreditRequest{
		AccessToken:     resolved.BearerToken,
		AccountID:       resolved.AccountID,
		RedeemRequestID: redeemRequestID,
	}
	result, err := h.openAISubscriptionCodexResetConsumer().ConsumeResetCredit(ctx, req)
	if isCodexUsageHTTPStatus(err, http.StatusUnauthorized) {
		refreshed, refreshErr := h.refreshOpenAISubscriptionCredentialForUsageSnapshot(ctx, credentialID)
		if refreshErr != nil {
			return chatgptcodex.ResetCreditResult{}, resolved, refreshErr
		}
		resolved = refreshed
		result, err = h.openAISubscriptionCodexResetConsumer().ConsumeResetCredit(ctx, chatgptcodex.ResetCreditRequest{
			AccessToken:     refreshed.BearerToken,
			AccountID:       refreshed.AccountID,
			RedeemRequestID: redeemRequestID,
		})
	}
	return result, resolved, err
}

func (h *Handlers) openAISubscriptionCodexUsageFetcher() chatgptcodex.UsageFetcher {
	if h != nil && h.CodexUsageFetcher != nil {
		return h.CodexUsageFetcher
	}
	cfg := config.ResolveOpenAIOAuthConfig(h.openAIOAuthConfig())
	return chatgptcodex.UsageClient{
		BaseURL:    cfg.CodexBackendBaseURL,
		Originator: cfg.CodexBackendOriginator,
		HTTPClient: h.openAIUpstreamHTTPClient(),
	}
}

func (h *Handlers) openAISubscriptionCodexResetConsumer() chatgptcodex.ResetCreditConsumer {
	if h != nil && h.CodexResetConsumer != nil {
		return h.CodexResetConsumer
	}
	if h != nil && h.CodexUsageFetcher != nil {
		if consumer, ok := h.CodexUsageFetcher.(chatgptcodex.ResetCreditConsumer); ok {
			return consumer
		}
	}
	cfg := config.ResolveOpenAIOAuthConfig(h.openAIOAuthConfig())
	return chatgptcodex.UsageClient{
		BaseURL:    cfg.CodexBackendBaseURL,
		Originator: cfg.CodexBackendOriginator,
		HTTPClient: h.openAIUpstreamHTTPClient(),
	}
}

func (h *Handlers) openAISubscriptionCodexUsageCache() *OpenAISubscriptionCodexUsageCache {
	if h == nil {
		return nil
	}
	h.codexUsageCacheMu.Lock()
	defer h.codexUsageCacheMu.Unlock()
	if h.CodexUsageCache == nil {
		h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	}
	return h.CodexUsageCache
}

func (h *Handlers) codexUsageTTL() time.Duration {
	if h != nil && h.CodexUsageTTL > 0 {
		return h.CodexUsageTTL
	}
	return defaultCodexUsageTTL
}

func (h *Handlers) codexUsageRefreshInterval() time.Duration {
	if h != nil && h.Config != nil && h.Config.TianjiSettings.CodexUsageRefreshIntervalSeconds != nil && *h.Config.TianjiSettings.CodexUsageRefreshIntervalSeconds > 0 {
		return time.Duration(*h.Config.TianjiSettings.CodexUsageRefreshIntervalSeconds) * time.Second
	}
	return defaultCodexUsageRefreshInterval
}

func (h *Handlers) codexUsageAsyncTimeout() time.Duration {
	if h != nil && h.Config != nil && h.Config.TianjiSettings.CodexUsageAsyncTimeoutSeconds != nil && *h.Config.TianjiSettings.CodexUsageAsyncTimeoutSeconds > 0 {
		return time.Duration(*h.Config.TianjiSettings.CodexUsageAsyncTimeoutSeconds) * time.Second
	}
	return defaultCodexUsageAsyncTimeout
}

func (h *Handlers) codexUsageExpiresAt(snapshot chatgptcodex.UsageSnapshot, now time.Time) time.Time {
	if resetAt, ok := codexUsageEarliestReset(snapshot, now); ok {
		return resetAt
	}
	return now.Add(h.codexUsageTTL())
}

func (h *Handlers) codexUsageBackoffBase() time.Duration {
	if h != nil && h.CodexUsageBackoffBase > 0 {
		return h.CodexUsageBackoffBase
	}
	return defaultCodexUsageBackoffBase
}

func (h *Handlers) codexUsageBackoffJitter(backoff time.Duration) time.Duration {
	if h != nil && h.CodexUsageBackoffJitter != nil {
		return h.CodexUsageBackoffJitter(backoff)
	}
	maxJitter := backoff / 10
	if maxJitter <= 0 {
		return 0
	}
	return time.Duration(rand.Int63n(int64(maxJitter) + 1))
}

func (h *Handlers) codexUsageErrorResult(credentialID string, err error, now time.Time) OpenAISubscriptionCodexUsageResult {
	return openAISubscriptionCodexUsageError(credentialID, h.codexUsageErrorReason(err), now)
}

func (h *Handlers) codexUsageErrorReason(err error) string {
	var httpErr *chatgptcodex.UsageHTTPError
	if errors.As(err, &httpErr) {
		return httpErr.Reason
	}
	if code := openAISubscriptionErrorCode(err); code != "" {
		return string(code)
	}
	if errors.Is(err, errCodexUsageEmptySnapshot) {
		return "empty_snapshot"
	}
	return "upstream_request_failed"
}

func openAISubscriptionCodexUsageError(credentialID, reason string, now time.Time) OpenAISubscriptionCodexUsageResult {
	status := "unavailable"
	switch reason {
	case "rate_limited", "upstream_server_error":
		status = "backoff"
	case "auth_error", string(OpenAISubscriptionCredentialAuthErr):
		status = "auth_error"
	}
	return OpenAISubscriptionCodexUsageResult{
		CredentialID:    credentialID,
		Status:          status,
		FetchedAt:       now,
		LastErrorReason: reason,
	}
}

func openAISubscriptionErrorCodeOrDefault(err error) OpenAISubscriptionCredentialErrorCode {
	code := openAISubscriptionErrorCode(err)
	if code == "" {
		return OpenAISubscriptionCredentialLookupErr
	}
	return code
}

func isCodexUsageHTTPStatus(err error, statusCode int) bool {
	var httpErr *chatgptcodex.UsageHTTPError
	return errors.As(err, &httpErr) && httpErr.StatusCode == statusCode
}

func (c *OpenAISubscriptionCodexUsageCache) getFresh(credentialID string, now time.Time, refreshInterval time.Duration) (OpenAISubscriptionCodexUsageResult, bool) {
	result, ok, _ := c.getFreshWithReason(credentialID, now, refreshInterval)
	return result, ok
}

func (c *OpenAISubscriptionCodexUsageCache) getFreshWithReason(credentialID string, now time.Time, refreshInterval time.Duration) (OpenAISubscriptionCodexUsageResult, bool, string) {
	result, ok, reason := c.getUsableWithReason(credentialID, now)
	if !ok {
		return result, ok, reason
	}
	if !codexUsageFreshWithin(result, now, refreshInterval) {
		return OpenAISubscriptionCodexUsageResult{}, false, "refresh_interval_stale"
	}
	return result, true, "fresh"
}

func (c *OpenAISubscriptionCodexUsageCache) getUsableWithReason(credentialID string, now time.Time) (OpenAISubscriptionCodexUsageResult, bool, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[credentialID]
	if !ok {
		return OpenAISubscriptionCodexUsageResult{}, false, "missing_entry"
	}
	if entry.Result.Status != "fresh" {
		return OpenAISubscriptionCodexUsageResult{}, false, "status_not_fresh"
	}
	if !entry.ExpiresAt.After(now) {
		return OpenAISubscriptionCodexUsageResult{}, false, "expired"
	}
	return entry.Result, true, "usable"
}

func (c *OpenAISubscriptionCodexUsageCache) getBackoff(credentialID string, now time.Time) (OpenAISubscriptionCodexUsageResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[credentialID]
	if !ok || entry.BackoffUntil.IsZero() || !entry.BackoffUntil.After(now) {
		return OpenAISubscriptionCodexUsageResult{}, false
	}
	if !codexUsageResultHasSnapshot(entry.Result) {
		return OpenAISubscriptionCodexUsageResult{}, false
	}
	result := entry.Result
	result.Status = "backoff"
	result.BackoffUntil = entry.BackoffUntil
	return result, true
}

func (c *OpenAISubscriptionCodexUsageCache) putSuccess(credentialID string, result OpenAISubscriptionCodexUsageResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[credentialID] = openAISubscriptionCodexUsageCacheEntry{
		Result:    result,
		ExpiresAt: result.ExpiresAt,
	}
}

func (c *OpenAISubscriptionCodexUsageCache) putError(credentialID string, result OpenAISubscriptionCodexUsageResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[credentialID] = openAISubscriptionCodexUsageCacheEntry{Result: result}
}

func (c *OpenAISubscriptionCodexUsageCache) Invalidate(credentialID string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, strings.TrimSpace(credentialID))
}

func (c *OpenAISubscriptionCodexUsageCache) putFailure(credentialID string, result OpenAISubscriptionCodexUsageResult, now time.Time, base time.Duration, jitter func(time.Duration) time.Duration) OpenAISubscriptionCodexUsageResult {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := c.entries[credentialID]
	entry.BackoffAttempt++
	if base <= 0 {
		base = defaultCodexUsageBackoffBase
	}
	backoff := base
	for i := 1; i < entry.BackoffAttempt && i < 6; i++ {
		backoff *= 2
	}
	if jitter != nil {
		backoff += jitter(backoff)
	}
	result.BackoffUntil = now.Add(backoff)
	if codexUsageResultHasSnapshot(entry.Result) {
		result.Snapshot = entry.Result.Snapshot
		result.FetchedAt = entry.Result.FetchedAt
	}
	entry.Result = result
	entry.BackoffUntil = result.BackoffUntil
	c.entries[credentialID] = entry
	return result
}

func codexUsageResultHasSnapshot(result OpenAISubscriptionCodexUsageResult) bool {
	if result.Snapshot.Email != "" ||
		result.Snapshot.PlanType != "" ||
		codexUsageRateLimitHasSnapshot(result.Snapshot.RateLimit) ||
		codexUsageResetCreditsHasSnapshot(result.Snapshot.RateLimitResetCredits) ||
		result.Snapshot.PrimaryWindow.UsedPercent != nil ||
		result.Snapshot.WeeklyWindow.UsedPercent != nil {
		return true
	}
	for _, bucket := range result.Snapshot.AdditionalBuckets {
		if bucket.PrimaryWindow.UsedPercent != nil || bucket.WeeklyWindow.UsedPercent != nil {
			return true
		}
	}
	return false
}

func codexUsageRateLimitHasSnapshot(rateLimit *chatgptcodex.UsageRateLimit) bool {
	if rateLimit == nil {
		return false
	}
	return rateLimit.Allowed != nil ||
		rateLimit.LimitReached != nil ||
		rateLimit.PrimaryWindow.UsedPercent != nil ||
		rateLimit.PrimaryWindow.ResetAfterSeconds != nil ||
		rateLimit.PrimaryWindow.LimitWindowSeconds != nil ||
		rateLimit.SecondaryWindow.UsedPercent != nil ||
		rateLimit.SecondaryWindow.ResetAfterSeconds != nil ||
		rateLimit.SecondaryWindow.LimitWindowSeconds != nil
}

func codexUsageResetCreditsHasSnapshot(resetCredits *chatgptcodex.UsageResetCredits) bool {
	return resetCredits != nil && resetCredits.AvailableCount != nil
}

func (h *Handlers) getCachedOpenAISubscriptionCodexUsageResult(ctx context.Context, credentialID string, now time.Time) (OpenAISubscriptionCodexUsageResult, bool) {
	if h == nil || h.Cache == nil {
		return OpenAISubscriptionCodexUsageResult{}, false
	}
	raw, err := h.Cache.Get(ctx, codexUsageCacheKey(credentialID))
	if err != nil {
		log.Printf("warn: codex-usage-cache: get %s: %v", credentialID, err)
		return OpenAISubscriptionCodexUsageResult{}, false
	}
	if len(raw) == 0 {
		return OpenAISubscriptionCodexUsageResult{}, false
	}
	return h.decodeCachedOpenAISubscriptionCodexUsageResult(ctx, credentialID, raw, now)
}

func (h *Handlers) hydrateCachedOpenAISubscriptionCodexUsageResults(ctx context.Context, credentialIDs []string, now time.Time) {
	if h == nil || h.Cache == nil || len(credentialIDs) == 0 {
		return
	}
	keys := make([]string, 0, len(credentialIDs))
	ids := make([]string, 0, len(credentialIDs))
	seen := make(map[string]struct{}, len(credentialIDs))
	for _, credentialID := range credentialIDs {
		credentialID = strings.TrimSpace(credentialID)
		if credentialID == "" {
			continue
		}
		if _, ok := seen[credentialID]; ok {
			continue
		}
		seen[credentialID] = struct{}{}
		keys = append(keys, codexUsageCacheKey(credentialID))
		ids = append(ids, credentialID)
	}
	if len(keys) == 0 {
		return
	}
	rawValues, err := h.Cache.MGet(ctx, keys...)
	if err != nil {
		log.Printf("warn: codex-usage-cache: mget: %v", err)
		return
	}
	if len(rawValues) != len(ids) {
		log.Printf("warn: codex-usage-cache: mget returned %d values for %d keys", len(rawValues), len(ids))
		return
	}
	cache := h.openAISubscriptionCodexUsageCache()
	refreshInterval := h.codexUsageRefreshInterval()
	for i, raw := range rawValues {
		if len(raw) == 0 {
			continue
		}
		result, ok := h.decodeCachedOpenAISubscriptionCodexUsageResult(ctx, ids[i], raw, now)
		if ok && codexUsageFreshWithin(result, now, refreshInterval) {
			cache.putSuccess(ids[i], result)
		}
	}
}

func (h *Handlers) decodeCachedOpenAISubscriptionCodexUsageResult(ctx context.Context, credentialID string, raw []byte, now time.Time) (OpenAISubscriptionCodexUsageResult, bool) {
	var cached openAISubscriptionCodexUsageCacheValue
	if err := json.Unmarshal(raw, &cached); err != nil {
		log.Printf("warn: codex-usage-cache: unmarshal %s: %v", credentialID, err)
		_ = h.Cache.Delete(ctx, codexUsageCacheKey(credentialID))
		return OpenAISubscriptionCodexUsageResult{}, false
	}
	result := OpenAISubscriptionCodexUsageResult{
		CredentialID:    credentialID,
		Status:          cached.Status,
		Snapshot:        cached.Snapshot,
		FetchedAt:       cached.FetchedAt,
		ExpiresAt:       cached.ExpiresAt,
		LastErrorReason: cached.LastErrorReason,
	}
	if result.Status == "" {
		result.Status = "fresh"
	}
	if result.ExpiresAt.IsZero() || !result.ExpiresAt.After(now) || !codexUsageResultHasSnapshot(result) {
		_ = h.Cache.Delete(ctx, codexUsageCacheKey(credentialID))
		return OpenAISubscriptionCodexUsageResult{}, false
	}
	return result, true
}

func (h *Handlers) persistOpenAISubscriptionCodexUsageResult(ctx context.Context, result OpenAISubscriptionCodexUsageResult, now time.Time) {
	if h == nil || h.Cache == nil || !codexUsageResultHasSnapshot(result) {
		return
	}
	ttl := result.ExpiresAt.Sub(now)
	if ttl <= 0 {
		return
	}
	body, err := json.Marshal(openAISubscriptionCodexUsageCacheValue{
		CredentialID:    result.CredentialID,
		Status:          result.Status,
		Snapshot:        result.Snapshot,
		FetchedAt:       result.FetchedAt,
		ExpiresAt:       result.ExpiresAt,
		LastErrorReason: result.LastErrorReason,
	})
	if err != nil {
		log.Printf("warn: codex-usage-cache: marshal %s: %v", result.CredentialID, err)
		return
	}
	if err := h.Cache.Set(ctx, codexUsageCacheKey(result.CredentialID), body, ttl); err != nil {
		log.Printf("warn: codex-usage-cache: set %s: %v", result.CredentialID, err)
	}
}

func codexUsageCacheKey(credentialID string) string {
	return codexUsageCacheKeyPrefix + credentialID
}

func codexUsageFreshWithin(result OpenAISubscriptionCodexUsageResult, now time.Time, refreshInterval time.Duration) bool {
	if refreshInterval <= 0 {
		refreshInterval = defaultCodexUsageRefreshInterval
	}
	return !result.FetchedAt.IsZero() && now.Sub(result.FetchedAt) <= refreshInterval
}

func codexUsageEarliestReset(snapshot chatgptcodex.UsageSnapshot, now time.Time) (time.Time, bool) {
	var earliest time.Time
	knownWindows := 0
	consider := func(raw string) bool {
		resetAt, ok := parseCodexUsageResetAt(raw)
		if !ok || !resetAt.After(now) {
			return false
		}
		if earliest.IsZero() || resetAt.Before(earliest) {
			earliest = resetAt
		}
		knownWindows++
		return true
	}
	considerWindow := func(window chatgptcodex.UsageWindow) bool {
		return window.UsedPercent == nil || consider(window.ResetAt)
	}
	if !considerWindow(snapshot.PrimaryWindow) {
		return time.Time{}, false
	}
	if !considerWindow(snapshot.WeeklyWindow) {
		return time.Time{}, false
	}
	for _, bucket := range snapshot.AdditionalBuckets {
		if !considerWindow(bucket.PrimaryWindow) || !considerWindow(bucket.WeeklyWindow) {
			return time.Time{}, false
		}
	}
	if knownWindows == 0 || earliest.IsZero() {
		return time.Time{}, false
	}
	return earliest, true
}

func parseCodexUsageResetAt(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	if resetAt, err := time.Parse(time.RFC3339, raw); err == nil {
		return resetAt, true
	}
	sec, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || sec <= 0 {
		return time.Time{}, false
	}
	return time.Unix(sec, 0), true
}
