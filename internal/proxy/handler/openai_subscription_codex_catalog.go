package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/cache"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/chatgptcodex"
	"github.com/praxisllmlab/tianjiLLM/internal/wildcard"
)

const (
	defaultCodexCatalogTTL            = time.Minute
	defaultCodexCatalogRefreshTimeout = 30 * time.Second
	defaultCodexCatalogFailureBackoff = time.Minute
	defaultCodexCatalogCacheTTL       = 24 * time.Hour
	defaultCodexCatalogLockTTL        = 30 * time.Second
	codexCatalogCacheKeyPrefix        = "openai_subscription_codex_catalog:"
)

type OpenAISubscriptionCodexCatalogResult struct {
	CredentialID           string
	Catalog                chatgptcodex.Catalog
	Generation             uint64
	InvalidationToken      string
	FetchedAt              time.Time
	ExpiresAt              time.Time
	Degraded               bool
	LastErrorReason        string
	PermanentlyUnavailable bool `json:"permanently_unavailable,omitempty"`
	LastRefreshAttemptAt   time.Time
}

type openAISubscriptionCodexCatalogCacheValue struct {
	CredentialID           string               `json:"credential_id"`
	Catalog                chatgptcodex.Catalog `json:"catalog"`
	Generation             uint64               `json:"generation,omitempty"`
	FetchedAt              time.Time            `json:"fetched_at"`
	ExpiresAt              time.Time            `json:"expires_at"`
	Degraded               bool                 `json:"degraded,omitempty"`
	LastRefreshAttemptAt   time.Time            `json:"last_refresh_attempt_at,omitempty"`
	LastErrorReason        string               `json:"last_error_reason,omitempty"`
	PermanentlyUnavailable bool                 `json:"permanently_unavailable,omitempty"`
	InvalidationToken      string               `json:"invalidation_token,omitempty"`
}

type OpenAISubscriptionCodexCatalogCache struct {
	mu          sync.Mutex
	entries     map[string]OpenAISubscriptionCodexCatalogResult
	generations map[string]uint64
}

var codexCatalogLockSequence atomic.Uint64
var codexCatalogLockOwner = strconv.FormatInt(time.Now().UnixNano(), 10)

func NewOpenAISubscriptionCodexCatalogCache() *OpenAISubscriptionCodexCatalogCache {
	return &OpenAISubscriptionCodexCatalogCache{
		entries:     map[string]OpenAISubscriptionCodexCatalogResult{},
		generations: map[string]uint64{},
	}
}
func (c *OpenAISubscriptionCodexCatalogCache) get(id string) (OpenAISubscriptionCodexCatalogResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.entries[id]
	return v, ok
}
func (c *OpenAISubscriptionCodexCatalogCache) put(v OpenAISubscriptionCodexCatalogResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if currentGeneration := c.generations[v.CredentialID]; v.Generation < currentGeneration {
		return
	}
	if v.Generation > c.generations[v.CredentialID] {
		c.generations[v.CredentialID] = v.Generation
	}
	c.entries[v.CredentialID] = v
}
func (c *OpenAISubscriptionCodexCatalogCache) generation(id string) uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.generations[id]
}
func (c *OpenAISubscriptionCodexCatalogCache) advanceGeneration(id string, generation uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if generation > c.generations[id] {
		c.generations[id] = generation
	}
	delete(c.entries, id)
}
func (c *OpenAISubscriptionCodexCatalogCache) accept(v OpenAISubscriptionCodexCatalogResult) OpenAISubscriptionCodexCatalogResult {
	c.mu.Lock()
	defer c.mu.Unlock()
	currentGeneration := c.generations[v.CredentialID]
	if v.Generation < currentGeneration {
		return OpenAISubscriptionCodexCatalogResult{
			CredentialID:    v.CredentialID,
			Generation:      currentGeneration,
			Degraded:        true,
			LastErrorReason: "invalidated",
		}
	}
	if v.Generation > currentGeneration {
		c.generations[v.CredentialID] = v.Generation
	}
	c.entries[v.CredentialID] = v
	return v
}
func (c *OpenAISubscriptionCodexCatalogCache) Invalidate(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.generations[id]++
	delete(c.entries, id)
}

func (h *Handlers) openAISubscriptionCodexCatalogCache() *OpenAISubscriptionCodexCatalogCache {
	h.codexCatalogCacheMu.Lock()
	defer h.codexCatalogCacheMu.Unlock()
	if h.CodexCatalogCache == nil {
		h.CodexCatalogCache = NewOpenAISubscriptionCodexCatalogCache()
	}
	return h.CodexCatalogCache
}
func (h *Handlers) codexCatalogTTL() time.Duration {
	if h != nil && h.CodexCatalogTTL > 0 {
		return h.CodexCatalogTTL
	}
	return defaultCodexCatalogTTL
}
func (h *Handlers) codexCatalogRefreshTimeout() time.Duration {
	if h != nil && h.CodexCatalogRefreshTimeout > 0 {
		return h.CodexCatalogRefreshTimeout
	}
	return defaultCodexCatalogRefreshTimeout
}
func (h *Handlers) openAISubscriptionCodexCatalogFetcher() chatgptcodex.CatalogFetcher {
	if h.CodexCatalogFetcher != nil {
		return h.CodexCatalogFetcher
	}
	cfg := config.ResolveOpenAIOAuthConfig(h.openAIOAuthConfig())
	return chatgptcodex.CatalogClient{BaseURL: cfg.CodexBackendBaseURL, Originator: cfg.CodexBackendOriginator, ClientVersion: cfg.CodexBackendClientVersion, HTTPClient: h.openAIUpstreamHTTPClient()}
}

func (h *Handlers) OpenAISubscriptionCodexCatalog(ctx context.Context, credentialID string) OpenAISubscriptionCodexCatalogResult {
	credentialID = strings.TrimSpace(credentialID)
	now := h.openAISubscriptionNowUTC()
	if credentialID == "" {
		return OpenAISubscriptionCodexCatalogResult{LastErrorReason: "credential_missing"}
	}
	cache := h.openAISubscriptionCodexCatalogCache()
	sharedCache := h.usesSharedCatalogCache()
	if cached, ok := cache.get(credentialID); ok {
		if !sharedCache && !cached.Degraded && now.Before(cached.ExpiresAt) {
			return cached
		}
		if !sharedCache && cached.Degraded && now.Sub(cached.LastRefreshAttemptAt) < defaultCodexCatalogFailureBackoff {
			return cached
		}
	}
	var expected []byte
	var invalidationToken string
	var sharedCacheToken bool
	cached, raw, cacheHit, sharedErr := h.getCachedOpenAISubscriptionCodexCatalogResultWithRaw(ctx, credentialID)
	if cacheHit {
		expected = raw
		invalidationToken = cached.InvalidationToken
		sharedCacheToken = invalidationToken != ""
		cache.put(cached)
		if cached.Generation >= cache.generation(credentialID) && !cached.Degraded && now.Before(cached.ExpiresAt) {
			return cached
		}
		if cached.Generation >= cache.generation(credentialID) && cached.Degraded && now.Sub(cached.LastRefreshAttemptAt) < defaultCodexCatalogFailureBackoff {
			return cached
		}
	}
	if sharedErr != nil {
		sharedCache = false
	}
	generation := cache.generation(credentialID)
	if cached, ok := cache.get(credentialID); ok && cached.Generation > generation {
		generation = cached.Generation
	}
	value, err, _ := h.codexUsageRefreshGroup.Do("catalog:"+credentialID, func() (any, error) {
		if cached, ok := cache.get(credentialID); ok {
			generation = cached.Generation
			if !sharedCacheToken {
				invalidationToken = cached.InvalidationToken
			}
			if !sharedCache && !cached.Degraded && now.Before(cached.ExpiresAt) {
				return cached, nil
			}
			if cached.Degraded && now.Sub(cached.LastRefreshAttemptAt) < defaultCodexCatalogFailureBackoff {
				return cached, nil
			}
		}
		refreshCtx, cancel := context.WithTimeout(ctx, h.codexCatalogRefreshTimeout())
		defer cancel()

		resolved, err := h.resolveOpenAISubscriptionCredentialForUsageSnapshot(refreshCtx, credentialID)
		if err != nil {
			return nil, err
		}
		catalog, err := h.openAISubscriptionCodexCatalogFetcher().FetchCatalog(refreshCtx, chatgptcodex.CatalogRequest{AccessToken: resolved.BearerToken, AccountID: resolved.AccountID})
		if err != nil {
			return nil, err
		}
		if len(catalog.Models) == 0 {
			return nil, errors.New("codex catalog returned no models")
		}
		result := OpenAISubscriptionCodexCatalogResult{CredentialID: credentialID, Catalog: catalog, Generation: generation, InvalidationToken: invalidationToken, FetchedAt: now, ExpiresAt: now.Add(h.codexCatalogTTL())}
		cache.accept(result)
		return result, nil
	})
	if err != nil {
		lastErrorReason, permanentlyUnavailable := openAISubscriptionCodexCatalogErrorClassification(err)
		if cached, ok := cache.get(credentialID); ok {
			if sharedCacheToken {
				cached.InvalidationToken = invalidationToken
			}
			cached.Degraded = true
			cached.LastErrorReason = lastErrorReason
			cached.PermanentlyUnavailable = permanentlyUnavailable
			cached.LastRefreshAttemptAt = now
			if h.persistOpenAISubscriptionCodexCatalogResult(ctx, cached, expected) {
				return cache.accept(cached)
			}
			if current, ok := h.getCachedOpenAISubscriptionCodexCatalogResult(ctx, credentialID); ok {
				if codexCatalogCacheResultAuthoritative(current, cached, now) {
					return cache.accept(current)
				}
			}
			cached = cache.accept(cached)
			log.Printf("Codex catalog refresh degraded credential_id=%s reason=%s", credentialID, lastErrorReason)
			return cached
		}
		result := OpenAISubscriptionCodexCatalogResult{
			CredentialID:           credentialID,
			Generation:             generation,
			InvalidationToken:      invalidationToken,
			Degraded:               true,
			LastErrorReason:        lastErrorReason,
			PermanentlyUnavailable: permanentlyUnavailable,
			LastRefreshAttemptAt:   now,
		}
		if h.persistOpenAISubscriptionCodexCatalogResult(ctx, result, expected) {
			return cache.accept(result)
		}
		if current, ok := h.getCachedOpenAISubscriptionCodexCatalogResult(ctx, credentialID); ok {
			if codexCatalogCacheResultAuthoritative(current, result, now) {
				return cache.accept(current)
			}
		}
		return cache.accept(result)
	}
	result, ok := value.(OpenAISubscriptionCodexCatalogResult)
	if !ok {
		return OpenAISubscriptionCodexCatalogResult{CredentialID: credentialID, Generation: generation, LastErrorReason: "invalid_catalog_result"}
	}
	if h.persistOpenAISubscriptionCodexCatalogResult(ctx, result, expected) {
		return cache.accept(result)
	}
	if current, ok := h.getCachedOpenAISubscriptionCodexCatalogResult(ctx, credentialID); ok {
		if codexCatalogCacheResultAuthoritative(current, result, now) {
			return cache.accept(current)
		}
	}
	return cache.accept(result)
}

func codexCatalogCacheResultAuthoritative(current, attempted OpenAISubscriptionCodexCatalogResult, now time.Time) bool {
	if current.Degraded || current.PermanentlyUnavailable {
		return true
	}
	return current.Generation >= attempted.Generation && now.Before(current.ExpiresAt)
}

func openAISubscriptionCodexCatalogErrorClassification(err error) (string, bool) {
	var typed *OpenAISubscriptionCredentialError
	if !errors.As(err, &typed) || typed == nil {
		return "refresh_failed", false
	}

	reason := strings.TrimSpace(typed.NonSelectableReason)
	switch reason {
	case "operator_disabled",
		string(OpenAISubscriptionCredentialReconnect),
		string(OpenAISubscriptionCredentialAuthErr):
		return reason, true
	}

	switch typed.Code {
	case OpenAISubscriptionCredentialDisabled,
		OpenAISubscriptionCredentialReconnect,
		OpenAISubscriptionCredentialAuthErr:
		return string(typed.Code), true
	default:
		return "refresh_failed", false
	}
}

func (h *Handlers) invalidateOpenAISubscriptionCodexCatalog(ctx context.Context, credentialID string) {
	credentialID = strings.TrimSpace(credentialID)
	if credentialID == "" {
		return
	}
	catalogCache := h.openAISubscriptionCodexCatalogCache()
	catalogCache.Invalidate(credentialID)
	if h == nil || h.Cache == nil {
		return
	}
	if _, ok := h.Cache.(cache.SharedCache); !ok {
		if err := h.Cache.Delete(ctx, codexCatalogCacheKey(credentialID)); err != nil {
			log.Printf("warn: codex-catalog-cache: delete %s: %v", credentialID, err)
		}
		return
	}
	unlock, ok := h.acquireCodexCatalogLock(ctx, credentialID)
	if !ok {
		h.tryOpenAISubscriptionCodexCatalogInvalidation(ctx, credentialID, catalogCache.generation(credentialID))
		return
	}
	defer unlock()
	key := codexCatalogCacheKey(credentialID)
	for attempt := 0; attempt < 3; attempt++ {
		current, err := h.getSharedCacheValue(ctx, key)
		if err != nil {
			log.Printf("warn: codex-catalog-cache: read before invalidate %s: %v", credentialID, err)
			h.tryOpenAISubscriptionCodexCatalogInvalidation(ctx, credentialID, catalogCache.generation(credentialID))
			return
		}
		generation := catalogCache.generation(credentialID)
		if len(current) > 0 {
			var cached openAISubscriptionCodexCatalogCacheValue
			if json.Unmarshal(current, &cached) == nil && cached.Generation >= generation {
				generation = cached.Generation + 1
				catalogCache.advanceGeneration(credentialID, generation)
			}
		}
		body, err := codexCatalogInvalidationBody(credentialID, generation)
		if err != nil {
			log.Printf("warn: codex-catalog-cache: marshal invalidation %s: %v", credentialID, err)
			return
		}
		if conditional, ok := h.Cache.(cache.CompareAndSetCache); ok {
			updated, err := conditional.CompareAndSet(ctx, key, current, body, defaultCodexCatalogCacheTTL)
			if err != nil {
				log.Printf("warn: codex-catalog-cache: invalidate %s: %v", credentialID, err)
				h.tryOpenAISubscriptionCodexCatalogInvalidation(ctx, credentialID, generation)
				return
			}
			if updated {
				return
			}
			continue
		}
		if err := h.Cache.Delete(ctx, key); err != nil {
			log.Printf("warn: codex-catalog-cache: delete after unsupported invalidate %s: %v", credentialID, err)
		}
		return
	}
	log.Printf("warn: codex-catalog-cache: invalidate lost race %s", credentialID)
	h.tryOpenAISubscriptionCodexCatalogInvalidation(ctx, credentialID, catalogCache.generation(credentialID))
}

func codexCatalogInvalidationBody(credentialID string, generation uint64) ([]byte, error) {
	return json.Marshal(openAISubscriptionCodexCatalogCacheValue{
		CredentialID:      credentialID,
		Generation:        generation,
		Degraded:          true,
		LastErrorReason:   "invalidated",
		InvalidationToken: codexCatalogLockOwner + ":" + strconv.FormatUint(codexCatalogLockSequence.Add(1), 10),
	})
}

func (h *Handlers) tryOpenAISubscriptionCodexCatalogInvalidation(ctx context.Context, credentialID string, generation uint64) {
	body, err := codexCatalogInvalidationBody(credentialID, generation)
	if err != nil {
		log.Printf("warn: codex-catalog-cache: marshal fallback invalidation %s: %v", credentialID, err)
		return
	}
	// The marker write is atomic for one key. Any stale refresh still compares
	// against the pre-invalidation value and therefore cannot overwrite it.
	if err := h.Cache.Set(ctx, codexCatalogCacheKey(credentialID), body, defaultCodexCatalogCacheTTL); err != nil {
		log.Printf("warn: codex-catalog-cache: fallback invalidation %s: %v", credentialID, err)
	}
}

func (h *Handlers) getCachedOpenAISubscriptionCodexCatalogResult(ctx context.Context, credentialID string) (OpenAISubscriptionCodexCatalogResult, bool) {
	result, _, ok, _ := h.getCachedOpenAISubscriptionCodexCatalogResultWithRaw(ctx, credentialID)
	return result, ok
}

func (h *Handlers) getCachedOpenAISubscriptionCodexCatalogResultWithRaw(ctx context.Context, credentialID string) (OpenAISubscriptionCodexCatalogResult, []byte, bool, error) {
	if h == nil || h.Cache == nil {
		return OpenAISubscriptionCodexCatalogResult{}, nil, false, nil
	}
	raw, err := h.getSharedCacheValue(ctx, codexCatalogCacheKey(credentialID))
	if err != nil {
		return OpenAISubscriptionCodexCatalogResult{}, nil, false, err
	}
	if len(raw) == 0 {
		return OpenAISubscriptionCodexCatalogResult{}, nil, false, nil
	}
	var cached openAISubscriptionCodexCatalogCacheValue
	if err := json.Unmarshal(raw, &cached); err != nil || (len(cached.Catalog.Models) == 0 && !cached.Degraded) {
		if err := h.Cache.Delete(ctx, codexCatalogCacheKey(credentialID)); err != nil {
			log.Printf("warn: codex-catalog-cache: delete invalid %s: %v", credentialID, err)
		}
		return OpenAISubscriptionCodexCatalogResult{}, nil, false, nil
	}
	return OpenAISubscriptionCodexCatalogResult{
		CredentialID:           credentialID,
		Catalog:                cached.Catalog,
		Generation:             cached.Generation,
		InvalidationToken:      cached.InvalidationToken,
		FetchedAt:              cached.FetchedAt,
		ExpiresAt:              cached.ExpiresAt,
		Degraded:               cached.Degraded,
		LastRefreshAttemptAt:   cached.LastRefreshAttemptAt,
		LastErrorReason:        cached.LastErrorReason,
		PermanentlyUnavailable: cached.PermanentlyUnavailable,
	}, append([]byte(nil), raw...), true, nil
}

func (h *Handlers) getSharedCacheValue(ctx context.Context, key string) ([]byte, error) {
	if h == nil || h.Cache == nil {
		return nil, nil
	}
	if shared, ok := h.Cache.(cache.SharedCache); ok {
		return shared.GetShared(ctx, key)
	}
	return h.Cache.Get(ctx, key)
}

func (h *Handlers) usesSharedCatalogCache() bool {
	if h == nil || h.Cache == nil {
		return false
	}
	_, shared := h.Cache.(cache.SharedCache)
	_, locked := h.Cache.(cache.LockCache)
	return shared && locked
}

func codexCatalogLockKey(credentialID string) string {
	return codexCatalogCacheKey(credentialID) + ":lock"
}

func (h *Handlers) acquireCodexCatalogLock(ctx context.Context, credentialID string) (func(), bool) {
	if h == nil || h.Cache == nil {
		return func() {}, true
	}
	locker, ok := h.Cache.(cache.LockCache)
	if !ok {
		if _, shared := h.Cache.(cache.SharedCache); shared {
			log.Printf("warn: codex-catalog-cache: shared cache lacks locking %s", credentialID)
			return nil, false
		}
		return func() {}, true
	}
	key := codexCatalogLockKey(credentialID)
	token := codexCatalogLockOwner + ":" + strconv.FormatUint(codexCatalogLockSequence.Add(1), 10)
	for attempt := 0; attempt < 20; attempt++ {
		acquired, err := locker.AcquireLock(ctx, key, token, defaultCodexCatalogLockTTL)
		if err != nil {
			log.Printf("warn: codex-catalog-cache: acquire lock %s: %v", credentialID, err)
			return nil, false
		}
		if acquired {
			return func() {
				if err := locker.ReleaseLock(context.Background(), key, token); err != nil {
					log.Printf("warn: codex-catalog-cache: release lock %s: %v", credentialID, err)
				}
			}, true
		}
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, false
		case <-timer.C:
		}
	}
	log.Printf("warn: codex-catalog-cache: lock busy %s", credentialID)
	return nil, false
}

func (h *Handlers) persistOpenAISubscriptionCodexCatalogResult(ctx context.Context, result OpenAISubscriptionCodexCatalogResult, expected []byte) bool {
	if h == nil || h.Cache == nil || (len(result.Catalog.Models) == 0 && !result.Degraded) {
		return true
	}
	unlock, ok := h.acquireCodexCatalogLock(ctx, result.CredentialID)
	if !ok {
		return false
	}
	defer unlock()
	catalogCache := h.openAISubscriptionCodexCatalogCache()
	// ponytail: one lock keeps the local generation fence atomic; use per-credential locks if refresh throughput requires it.
	catalogCache.mu.Lock()
	defer catalogCache.mu.Unlock()
	if currentGeneration := catalogCache.generations[result.CredentialID]; result.Generation < currentGeneration {
		log.Printf("warn: codex-catalog-cache: skip invalidated stale write %s", result.CredentialID)
		return false
	}
	body, err := json.Marshal(openAISubscriptionCodexCatalogCacheValue{
		CredentialID:           result.CredentialID,
		Catalog:                result.Catalog,
		Generation:             result.Generation,
		InvalidationToken:      result.InvalidationToken,
		FetchedAt:              result.FetchedAt,
		ExpiresAt:              result.ExpiresAt,
		Degraded:               result.Degraded,
		LastRefreshAttemptAt:   result.LastRefreshAttemptAt,
		LastErrorReason:        result.LastErrorReason,
		PermanentlyUnavailable: result.PermanentlyUnavailable,
	})
	if err != nil {
		log.Printf("warn: codex-catalog-cache: marshal %s: %v", result.CredentialID, err)
		return false
	}
	conditional, ok := h.Cache.(cache.CompareAndSetCache)
	if !ok {
		if _, shared := h.Cache.(cache.SharedCache); shared {
			log.Printf("warn: codex-catalog-cache: skip non-atomic write %s", result.CredentialID)
			return false
		}
		if setErr := h.Cache.Set(ctx, codexCatalogCacheKey(result.CredentialID), body, defaultCodexCatalogCacheTTL); setErr != nil {
			log.Printf("warn: codex-catalog-cache: set local cache %s: %v", result.CredentialID, setErr)
			return false
		}
		return true
	}
	updated, err := conditional.CompareAndSet(ctx, codexCatalogCacheKey(result.CredentialID), expected, body, defaultCodexCatalogCacheTTL)
	if err != nil {
		log.Printf("warn: codex-catalog-cache: compare-and-set %s: %v", result.CredentialID, err)
		return false
	}
	if !updated {
		current, getErr := h.getSharedCacheValue(ctx, codexCatalogCacheKey(result.CredentialID))
		if getErr == nil && current == nil && expected != nil {
			updated, err = conditional.CompareAndSet(ctx, codexCatalogCacheKey(result.CredentialID), nil, body, defaultCodexCatalogCacheTTL)
			if err != nil {
				log.Printf("warn: codex-catalog-cache: compare-and-set empty retry %s: %v", result.CredentialID, err)
				return false
			}
		} else if !result.Degraded {
			var cached openAISubscriptionCodexCatalogCacheValue
			parsed := getErr == nil && len(current) > 0 && json.Unmarshal(current, &cached) == nil
			// An invalidation marker may have replaced an unknown value after a
			// read error; never let a pre-invalidation refresh replace it.
			if parsed && cached.LastErrorReason != "invalidated" &&
				cached.Generation < result.Generation &&
				cached.InvalidationToken == result.InvalidationToken {
				updated, err = conditional.CompareAndSet(ctx, codexCatalogCacheKey(result.CredentialID), current, body, defaultCodexCatalogCacheTTL)
				if err != nil {
					log.Printf("warn: codex-catalog-cache: compare-and-set generation retry %s: %v", result.CredentialID, err)
					return false
				}
			} else if parsed && cached.Degraded && cached.LastErrorReason == "refresh_failed" &&
				cached.Generation == result.Generation &&
				cached.InvalidationToken == result.InvalidationToken {
				updated, err = conditional.CompareAndSet(ctx, codexCatalogCacheKey(result.CredentialID), current, body, defaultCodexCatalogCacheTTL)
				if err != nil {
					log.Printf("warn: codex-catalog-cache: compare-and-set retry %s: %v", result.CredentialID, err)
					return false
				}
			}
		}
	}
	if !updated {
		log.Printf("warn: codex-catalog-cache: compare-and-set skipped stale write %s", result.CredentialID)
	}
	return updated
}

func codexCatalogCacheKey(credentialID string) string {
	return codexCatalogCacheKeyPrefix + credentialID
}

func (h *Handlers) codexCatalogForModel(ctx context.Context, model config.ModelConfig) (chatgptcodex.CatalogModel, bool) {
	params := model.TianjiParams
	providerName, _ := provider.ParseModelName(params.Model)
	if providerName == "openai" {
		ids, err := h.openAISubscriptionCredentialIDsInRequestScope(ctx)
		if err != nil || len(ids) == 0 {
			return chatgptcodex.CatalogModel{}, false
		}
		params.OpenAISubscriptionCredentialIDs = ids
		params.OpenAISubscriptionTransport = config.OpenAISubscriptionTransportChatGPTCodexBackend
	} else if len(params.OpenAISubscriptionCredentialIDs) > 0 || strings.TrimSpace(params.OpenAISubscriptionTransport) != "" {
		return chatgptcodex.CatalogModel{}, false
	}
	return h.codexCatalogForParams(ctx, params)
}

func (h *Handlers) codexCatalogForParams(ctx context.Context, params config.TianjiParams) (chatgptcodex.CatalogModel, bool) {
	return h.codexCatalogForParamsWithLookup(params, func(credentialID string) OpenAISubscriptionCodexCatalogResult {
		return h.OpenAISubscriptionCodexCatalog(ctx, credentialID)
	})
}

func (h *Handlers) codexResponsesLiteForParams(ctx context.Context, params config.TianjiParams) (bool, bool) {
	if h == nil || h.CodexCatalogFetcher == nil {
		return false, false
	}
	return codexResponsesLiteForParamsWithLookup(params, func(credentialID string) OpenAISubscriptionCodexCatalogResult {
		return h.OpenAISubscriptionCodexCatalog(ctx, credentialID)
	})
}

func (h *Handlers) codexResponsesLiteForParamsCached(ctx context.Context, params config.TianjiParams) (bool, bool) {
	now := h.openAISubscriptionNowUTC()
	cache := h.openAISubscriptionCodexCatalogCache()
	return codexResponsesLiteForParamsWithLookup(params, func(credentialID string) OpenAISubscriptionCodexCatalogResult {
		if !h.usesSharedCatalogCache() {
			if cached, ok := cache.get(credentialID); ok {
				cached = cache.accept(cached)
				if !cached.Degraded && now.Before(cached.ExpiresAt) {
					return cached
				}
				return OpenAISubscriptionCodexCatalogResult{}
			}
		}
		if cached, ok := h.getCachedOpenAISubscriptionCodexCatalogResult(ctx, credentialID); ok {
			cached = cache.accept(cached)
			if !cached.Degraded && now.Before(cached.ExpiresAt) {
				return cached
			}
		}
		return OpenAISubscriptionCodexCatalogResult{}
	})
}

func codexResponsesLiteForParamsWithLookup(params config.TianjiParams, lookup func(string) OpenAISubscriptionCodexCatalogResult) (bool, bool) {
	if !isChatGPTCodexBackendTransport(params) {
		return false, false
	}
	if len(params.OpenAISubscriptionCredentialIDs) == 0 {
		return false, false
	}
	if strings.Contains(params.Model, "*") {
		if codexWildcardSlugPattern(params.Model) == "" {
			return false, false
		}
		catalogs := codexCatalogResultsConcurrently(params.OpenAISubscriptionCredentialIDs, lookup)
		lookup = func(credentialID string) OpenAISubscriptionCodexCatalogResult {
			return catalogs[strings.TrimSpace(credentialID)]
		}
		return codexResponsesLiteForWildcardWithLookup(params, lookup)
	}
	candidates := codexCatalogModelCandidates(params.Model)
	if len(candidates) == 0 {
		return false, false
	}
	catalogs := codexCatalogResultsConcurrently(params.OpenAISubscriptionCredentialIDs, lookup)
	lookup = func(credentialID string) OpenAISubscriptionCodexCatalogResult {
		return catalogs[strings.TrimSpace(credentialID)]
	}
	for _, candidate := range candidates {
		found := true
		useResponsesLite := true
		for _, credentialID := range params.OpenAISubscriptionCredentialIDs {
			entry, ok := lookup(credentialID).Catalog.Model(candidate)
			if !ok {
				found = false
				break
			}
			if !entry.UseResponsesLite {
				useResponsesLite = false
			}
		}
		if found {
			return useResponsesLite, true
		}
	}
	return false, false
}

func codexResponsesLiteForWildcardWithLookup(params config.TianjiParams, lookup func(string) OpenAISubscriptionCodexCatalogResult) (bool, bool) {
	pattern := codexWildcardSlugPattern(params.Model)
	if pattern == "" || len(params.OpenAISubscriptionCredentialIDs) == 0 {
		return false, false
	}
	found := false
	useResponsesLite := true
	for _, credentialID := range params.OpenAISubscriptionCredentialIDs {
		catalog := lookup(credentialID)
		credentialFound := false
		for _, entry := range catalog.Catalog.Models {
			if wildcard.Match(pattern, strings.TrimPrefix(entry.Slug, "openai/")) == nil {
				continue
			}
			found = true
			credentialFound = true
			useResponsesLite = useResponsesLite && entry.UseResponsesLite
		}
		if !credentialFound {
			if len(catalog.Catalog.Models) == 0 {
				return false, false
			}
			useResponsesLite = false
		}
	}
	return useResponsesLite, found
}

func codexCatalogResultsConcurrently(credentialIDs []string, lookup func(string) OpenAISubscriptionCodexCatalogResult) map[string]OpenAISubscriptionCodexCatalogResult {
	results := make(map[string]OpenAISubscriptionCodexCatalogResult, len(credentialIDs))
	seen := make(map[string]struct{}, len(credentialIDs))
	var mu sync.Mutex
	var wait sync.WaitGroup
	for _, credentialID := range credentialIDs {
		credentialID = strings.TrimSpace(credentialID)
		if credentialID == "" {
			continue
		}
		if _, ok := seen[credentialID]; ok {
			continue
		}
		seen[credentialID] = struct{}{}
		wait.Add(1)
		go func(credentialID string) {
			defer wait.Done()
			result := lookup(credentialID)
			mu.Lock()
			results[credentialID] = result
			mu.Unlock()
		}(credentialID)
	}
	wait.Wait()
	return results
}

func (h *Handlers) codexCatalogForParamsWithLookup(params config.TianjiParams, lookup func(string) OpenAISubscriptionCodexCatalogResult) (chatgptcodex.CatalogModel, bool) {
	if !isChatGPTCodexBackendTransport(params) {
		return chatgptcodex.CatalogModel{}, false
	}
	candidates := codexCatalogModelCandidates(params.Model)
	if len(candidates) == 0 {
		return chatgptcodex.CatalogModel{}, false
	}
	credentialIDs := params.OpenAISubscriptionCredentialIDs
	if len(credentialIDs) == 0 {
		return chatgptcodex.CatalogModel{}, false
	}
	catalogs := make(map[string]OpenAISubscriptionCodexCatalogResult, len(credentialIDs))
	eligibleCredentialIDs := make([]string, 0, len(credentialIDs))
	for _, credentialID := range credentialIDs {
		result := lookup(credentialID)
		catalogs[credentialID] = result
		if result.PermanentlyUnavailable {
			continue
		}
		if result.Degraded {
			return chatgptcodex.CatalogModel{}, false
		}
		eligibleCredentialIDs = append(eligibleCredentialIDs, credentialID)
	}
	if len(eligibleCredentialIDs) == 0 {
		return chatgptcodex.CatalogModel{}, false
	}
	credentialIDs = eligibleCredentialIDs
	for _, candidate := range candidates {
		var selected chatgptcodex.CatalogModel
		availableToAll := true
		for index, credentialID := range credentialIDs {
			entry, ok := catalogs[credentialID].Catalog.Model(candidate)
			if !ok {
				availableToAll = false
				break
			}
			if index == 0 {
				selected = entry
				continue
			}
			if !reflect.DeepEqual(selected, entry) {
				availableToAll = false
				break
			}
		}
		if availableToAll {
			return selected, true
		}
	}
	if strings.Contains(params.Model, "*") {
		pattern := codexWildcardSlugPattern(params.Model)
		if pattern == "" {
			return chatgptcodex.CatalogModel{}, false
		}
		var wildcardModel chatgptcodex.CatalogModel
		found := false
		useResponsesLite := true
		for _, credentialID := range credentialIDs {
			credentialFound := false
			for _, entry := range catalogs[credentialID].Catalog.Models {
				if wildcard.Match(pattern, strings.TrimPrefix(entry.Slug, "openai/")) == nil {
					continue
				}
				found = true
				credentialFound = true
				useResponsesLite = useResponsesLite && entry.UseResponsesLite
			}
			if !credentialFound {
				useResponsesLite = false
			}
		}
		if found {
			wildcardModel.UseResponsesLite = useResponsesLite
			return wildcardModel, true
		}
	}
	return chatgptcodex.CatalogModel{}, false
}

func codexWildcardSlugPattern(upstream string) string {
	_, pattern, ok := strings.Cut(strings.TrimSpace(upstream), "/")
	if !ok {
		return ""
	}
	return strings.TrimSpace(pattern)
}

func codexCatalogModelCandidates(upstream string) []string {
	upstream = strings.TrimSpace(upstream)
	if upstream == "" {
		return nil
	}
	candidates := []string{upstream}
	providerName, modelSlug, ok := strings.Cut(upstream, "/")
	if ok && providerName == "openai" {
		modelSlug = strings.TrimSpace(modelSlug)
		if modelSlug != "" {
			candidates = append(candidates, modelSlug)
		}
	}
	return candidates
}
