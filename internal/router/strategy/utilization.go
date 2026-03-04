package strategy

import (
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/router"
)

// AlertFunc is the signature for sending token-switch alerts (Discord, etc.).
type AlertFunc func(msg string)

// LowestUtilization picks the deployment whose OAuth token has the lowest
// 5h utilization. It maintains a global active token and switches only
// when the current token exceeds the threshold or is rate-limited.
type LowestUtilization struct {
	mu         sync.RWMutex
	store      callback.RateLimitStore
	threshold  float64 // percentage 0-100
	activeKey  string  // token key currently in use
	lastUsedAt map[string]time.Time
	alertFn    AlertFunc
}

// NewLowestUtilization creates a new LowestUtilization strategy.
// threshold is a percentage (0-100); defaults to 80 if <= 0.
func NewLowestUtilization(store callback.RateLimitStore, threshold float64, alertFn AlertFunc) *LowestUtilization {
	if threshold <= 0 {
		threshold = 80
	}
	return &LowestUtilization{
		store:      store,
		threshold:  threshold,
		lastUsedAt: make(map[string]time.Time),
		alertFn:    alertFn,
	}
}

// PickKey selects the best token key from a list of cache keys using
// lowest-utilization logic. Goroutine-safe. Used by the native proxy path
// to share selection logic without converting to router.Deployment.
func (lu *LowestUtilization) PickKey(allKeys []string) string {
	if len(allKeys) == 0 {
		return ""
	}
	lu.mu.Lock()
	defer lu.mu.Unlock()
	picked := lu.pickTokenKey(allKeys)
	lu.lastUsedAt[picked] = time.Now()
	return picked
}

func (lu *LowestUtilization) Pick(deployments []*router.Deployment) *router.Deployment {
	if len(deployments) == 0 {
		return nil
	}

	// Build token key → deployment(s) mapping.
	keyToDeps := make(map[string][]*router.Deployment)
	var allKeys []string
	seen := make(map[string]bool)
	for _, d := range deployments {
		key := callback.RateLimitCacheKey(d.APIKey())
		keyToDeps[key] = append(keyToDeps[key], d)
		if !seen[key] {
			seen[key] = true
			allKeys = append(allKeys, key)
		}
	}

	lu.mu.Lock()
	defer lu.mu.Unlock()

	picked := lu.pickTokenKey(allKeys)

	// Record lastUsedAt for the picked key.
	lu.lastUsedAt[picked] = time.Now()

	deps := keyToDeps[picked]
	if len(deps) == 1 {
		return deps[0]
	}
	return deps[rand.IntN(len(deps))]
}

// SeedLastUsedAt sets the lastUsedAt timestamp for a given key. Intended for testing.
func (lu *LowestUtilization) SeedLastUsedAt(key string, t time.Time) {
	lu.mu.Lock()
	defer lu.mu.Unlock()
	lu.lastUsedAt[key] = t
}

// pickTokenKey selects the best token key, switching from activeKey only when needed.
func (lu *LowestUtilization) pickTokenKey(allKeys []string) string {
	// Cold start: no active key yet → pick best or shuffle.
	if lu.activeKey == "" {
		picked := lu.pickBestKey(allKeys)
		if picked != "" {
			lu.activeKey = picked
			return picked
		}
		// No utilization data at all → shuffle.
		picked = allKeys[rand.IntN(len(allKeys))]
		lu.activeKey = picked
		return picked
	}

	// Check if activeKey is still in the available set.
	activeAvailable := false
	for _, k := range allKeys {
		if k == lu.activeKey {
			activeAvailable = true
			break
		}
	}
	if !activeAvailable {
		// Active key not in deployment set — pick new.
		return lu.switchToNewKey(allKeys)
	}

	// Check active key's utilization (single store read to avoid TOCTOU).
	if lu.store != nil {
		state, ok := lu.store.Get(lu.activeKey)
		if ok && state.Unified5hUtilization >= 0 {
			util := state.Unified5hUtilization * 100
			isRateLimited := state.Unified5hStatus == "rate_limited"

			if isRateLimited || util >= lu.threshold {
				return lu.switchToNewKey(allKeys)
			}
		}
	}

	return lu.activeKey
}

// switchToNewKey picks the best alternative and updates activeKey.
func (lu *LowestUtilization) switchToNewKey(allKeys []string) string {
	oldKey := lu.activeKey
	picked := lu.pickBestKey(allKeys)
	if picked == "" {
		// All rate-limited or no data → fallback shuffle.
		picked = allKeys[rand.IntN(len(allKeys))]
	}

	lu.activeKey = picked
	if oldKey != "" && picked != oldKey {
		lu.emitSwitchAlert(oldKey, picked)
	}
	return picked
}

// pickBestKey selects the key with the lowest utilization (excluding rate_limited).
// On tie, picks the one with the oldest lastUsedAt (LRU).
// When all tied candidates have zero lastUsedAt (cold start), picks randomly.
func (lu *LowestUtilization) pickBestKey(keys []string) string {
	if lu.store == nil {
		return ""
	}

	type candidate struct {
		key      string
		util     float64
		lastUsed time.Time
	}

	var candidates []candidate
	bestUtil := float64(-1)

	for _, key := range keys {
		state, ok := lu.store.Get(key)
		if !ok || state.Unified5hStatus == "rate_limited" || state.Unified5hUtilization < 0 {
			continue
		}

		util := state.Unified5hUtilization * 100
		lastUsed := lu.lastUsedAt[key]

		if bestUtil < 0 || util < bestUtil {
			bestUtil = util
			candidates = []candidate{{key, util, lastUsed}}
		} else if util == bestUtil {
			candidates = append(candidates, candidate{key, util, lastUsed})
		}
	}

	if len(candidates) == 0 {
		return ""
	}
	if len(candidates) == 1 {
		return candidates[0].key
	}

	// Check if all candidates have zero lastUsedAt (cold start).
	allZero := true
	for _, c := range candidates {
		if !c.lastUsed.IsZero() {
			allZero = false
			break
		}
	}
	if allZero {
		return candidates[rand.IntN(len(candidates))].key
	}

	// LRU: pick the candidate with the oldest lastUsedAt.
	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.lastUsed.Before(best.lastUsed) {
			best = c
		}
	}
	return best.key
}

// emitSwitchAlert logs and optionally sends a Discord alert about a token switch.
func (lu *LowestUtilization) emitSwitchAlert(oldKey, newKey string) {
	oldSuffix := maskKey(oldKey)
	newSuffix := maskKey(newKey)

	msg := "OAuth token switched: ***" + oldSuffix + " → ***" + newSuffix
	slog.Warn(msg, "old_token", oldSuffix, "new_token", newSuffix)

	if lu.alertFn != nil {
		lu.alertFn("⚠️ " + msg)
	}
}

// maskKey returns the last 4 chars of a key for logging.
func maskKey(key string) string {
	if len(key) <= 4 {
		return key
	}
	return key[len(key)-4:]
}
