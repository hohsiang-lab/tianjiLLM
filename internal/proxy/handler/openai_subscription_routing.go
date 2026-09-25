package handler

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/chatgptcodex"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
	"github.com/praxisllmlab/tianjiLLM/internal/security/redact"
)

const openAISubscriptionProviderKey = "openai-subscription"

const (
	OpenAISubscriptionCredentialRateLimited    OpenAISubscriptionCredentialErrorCode = "rate_limited"
	OpenAISubscriptionCredentialCodexUsageGate OpenAISubscriptionCredentialErrorCode = "codex_usage_gate"
)

type codexSessionIdentity struct {
	SessionID string
	Source    string
}

type openAISubscriptionCandidate struct {
	resolvedOpenAISubscriptionCredential
}

func openAISubscriptionCandidatesFromResolved(resolved []resolvedOpenAISubscriptionCredential) []openAISubscriptionCandidate {
	candidates := make([]openAISubscriptionCandidate, 0, len(resolved))
	for _, credential := range resolved {
		candidates = append(candidates, openAISubscriptionCandidate{resolvedOpenAISubscriptionCredential: credential})
	}
	return candidates
}

type openAISubscriptionAllUnusableError struct {
	failures []OpenAISubscriptionCredentialError
}

func (e *openAISubscriptionAllUnusableError) Error() string {
	if e == nil || len(e.failures) == 0 {
		return "OpenAI subscription credential resolution failed: all configured credentials unusable"
	}
	parts := make([]string, 0, len(e.failures))
	for _, failure := range e.failures {
		parts = append(parts, fmt.Sprintf("%s=%s", failure.CredentialID, failure.Code))
	}
	return "OpenAI subscription credential resolution failed: all configured credentials unusable (" + redact.String(strings.Join(parts, "; ")) + ")"
}

func (h *Handlers) resolveOpenAISubscriptionCandidates(
	ctx context.Context,
	params config.TianjiParams,
	transport openAISubscriptionTransport,
) ([]openAISubscriptionCandidate, []OpenAISubscriptionCredentialError) {
	ids := params.OpenAISubscriptionCredentialIDs
	candidates := make([]openAISubscriptionCandidate, 0, len(ids))
	failures := make([]OpenAISubscriptionCredentialError, 0)
	if len(ids) == 0 {
		return candidates, failures
	}

	for _, credentialID := range ids {
		resolved, err := h.resolveOpenAISubscriptionCredentialByID(ctx, credentialID, transport)
		if err != nil {
			failures = append(failures, openAISubscriptionFailure(credentialID, err))
			continue
		}
		candidates = append(candidates, openAISubscriptionCandidate{resolvedOpenAISubscriptionCredential: resolved})
	}
	return candidates, failures
}

func (h *Handlers) resolveOpenAISubscriptionAttemptOrder(
	ctx context.Context,
	params config.TianjiParams,
	transport openAISubscriptionTransport,
) ([]resolvedOpenAISubscriptionCredential, error) {
	available, err := h.resolveOpenAISubscriptionAvailableCandidates(ctx, params, transport)
	if err != nil {
		return nil, err
	}
	return h.orderOpenAISubscriptionCandidates(ctx, params, available), nil
}

func (h *Handlers) resolveOpenAISubscriptionAvailableCandidates(
	ctx context.Context,
	params config.TianjiParams,
	transport openAISubscriptionTransport,
) ([]openAISubscriptionCandidate, error) {
	if len(params.OpenAISubscriptionCredentialIDs) == 0 {
		return nil, nil
	}
	if h.DB == nil {
		return nil, errors.New("OpenAI subscription credential resolution failed: database not configured")
	}

	candidates, failures := h.resolveOpenAISubscriptionCandidates(ctx, params, transport)
	now := h.openAISubscriptionNowUTC()
	available := make([]openAISubscriptionCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if state, ok := h.openAISubscriptionRateLimitState(candidate.CredentialID); ok && state.Gated(now) {
			failures = append(failures, OpenAISubscriptionCredentialError{
				CredentialID: candidate.CredentialID,
				Code:         OpenAISubscriptionCredentialRateLimited,
				Message:      "credential rate limited",
			})
			continue
		}
		available = append(available, candidate)
	}
	if transport == openAISubscriptionTransportChatGPTCodexBackend {
		var codexUsageFailures []OpenAISubscriptionCredentialError
		available, codexUsageFailures = h.filterOpenAISubscriptionCodexUsageSelectable(ctx, available, now)
		failures = append(failures, codexUsageFailures...)
	}
	if len(available) == 0 {
		if len(failures) == 1 {
			failure := failures[0]
			return nil, &failure
		}
		return nil, &openAISubscriptionAllUnusableError{failures: failures}
	}
	return available, nil
}

func (h *Handlers) filterOpenAISubscriptionCodexUsageSelectable(
	ctx context.Context,
	candidates []openAISubscriptionCandidate,
	now time.Time,
) ([]openAISubscriptionCandidate, []OpenAISubscriptionCredentialError) {
	if len(candidates) == 0 {
		return candidates, nil
	}
	h.hydrateCachedOpenAISubscriptionCodexUsageForCandidates(ctx, candidates, now)

	available := make([]openAISubscriptionCandidate, 0, len(candidates))
	failures := make([]OpenAISubscriptionCredentialError, 0)
	for _, candidate := range candidates {
		metadata := h.codexStickyMetadata(candidate.CredentialID, now)
		if metadata.Known && !metadata.Selectable {
			failures = append(failures, OpenAISubscriptionCredentialError{
				CredentialID: candidate.CredentialID,
				Code:         OpenAISubscriptionCredentialCodexUsageGate,
				Message:      "credential exceeds Codex usage selection gate",
			})
			continue
		}
		available = append(available, candidate)
	}
	return available, failures
}

func resolvedOpenAISubscriptionCredentials(candidates []openAISubscriptionCandidate) []resolvedOpenAISubscriptionCredential {
	resolved := make([]resolvedOpenAISubscriptionCredential, 0, len(candidates))
	for _, candidate := range candidates {
		resolved = append(resolved, candidate.resolvedOpenAISubscriptionCredential)
	}
	return resolved
}

func (h *Handlers) refreshStaleOpenAISubscriptionCodexUsage(ctx context.Context, candidates []openAISubscriptionCandidate) {
	now := h.openAISubscriptionNowUTC()
	usageCache := h.openAISubscriptionCodexUsageCache()
	refreshInterval := h.codexUsageRefreshInterval()
	missingCredentialIDs := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if _, ok := usageCache.getFresh(candidate.CredentialID, now, refreshInterval); !ok {
			missingCredentialIDs = append(missingCredentialIDs, candidate.CredentialID)
		}
	}
	h.hydrateCachedOpenAISubscriptionCodexUsageResults(ctx, missingCredentialIDs, now)
	for _, credentialID := range missingCredentialIDs {
		if _, ok := usageCache.getFresh(credentialID, now, refreshInterval); ok {
			continue
		}
		h.OpenAISubscriptionCodexUsageSnapshot(ctx, credentialID, false)
	}
}

func (h *Handlers) orderOpenAISubscriptionCodexCandidates(
	ctx context.Context,
	params config.TianjiParams,
	resolved []resolvedOpenAISubscriptionCredential,
) []resolvedOpenAISubscriptionCredential {
	return h.orderOpenAISubscriptionCodexCandidatesWithIdentity(ctx, params, resolved, codexSessionIdentity{Source: "missing"})
}

func (h *Handlers) orderOpenAISubscriptionCodexCandidatesWithIdentity(
	ctx context.Context,
	params config.TianjiParams,
	resolved []resolvedOpenAISubscriptionCredential,
	identity codexSessionIdentity,
) []resolvedOpenAISubscriptionCredential {
	candidates := openAISubscriptionCandidatesFromResolved(resolved)
	if !h.shouldOrderOpenAISubscriptionCodexCandidates(candidates) {
		return resolved
	}
	routeKey := openAISubscriptionCodexSessionRouteKey(ctx, params, identity)
	if strings.TrimSpace(identity.SessionID) == "" && isChatGPTCodexBackendTransport(params) {
		log.Printf("info: codex-routing: missing session id fallback source=%s track=%s model=%s candidate_count=%d",
			identity.Source, safeOpenAISubscriptionTrackLogValue(routeKey), params.Model, len(candidates))
	}
	return h.orderOpenAISubscriptionCandidatesWithRouteKey(ctx, params, candidates, routeKey, identity)
}

func (h *Handlers) shouldOrderOpenAISubscriptionCodexCandidates(candidates []openAISubscriptionCandidate) bool {
	if len(candidates) <= 1 || h == nil || h.Config == nil {
		return false
	}
	return h.Config.NativeUpstreamStrategy == config.StrategySticky || h.Config.NativeUpstreamStrategy == config.StrategyLowestUtilization
}

func (h *Handlers) refreshOpenAISubscriptionCodexUsageAfterResponse(ctx context.Context, resolved []resolvedOpenAISubscriptionCredential) {
	h.refreshStaleOpenAISubscriptionCodexUsageAsync(ctx, openAISubscriptionCandidatesFromResolved(resolved))
}

func (h *Handlers) refreshStaleOpenAISubscriptionCodexUsageAsync(ctx context.Context, candidates []openAISubscriptionCandidate) {
	if len(candidates) == 0 {
		return
	}
	key := codexUsageAsyncRefreshKey(candidates)
	if key == "" {
		return
	}
	candidates = append([]openAISubscriptionCandidate(nil), candidates...)
	_ = h.codexUsageAsyncGroup.DoChan(key, func() (any, error) {
		refreshCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), h.codexUsageAsyncTimeout())
		defer cancel()
		h.refreshStaleOpenAISubscriptionCodexUsage(refreshCtx, candidates)
		return nil, nil
	})
}

func codexUsageAsyncRefreshKey(candidates []openAISubscriptionCandidate) string {
	ids := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.CredentialID) != "" {
			ids = append(ids, strings.TrimSpace(candidate.CredentialID))
		}
	}
	if len(ids) == 0 {
		return ""
	}
	sort.Strings(ids)
	return strings.Join(ids, "\x00")
}

func (h *Handlers) orderOpenAISubscriptionCandidates(
	ctx context.Context,
	params config.TianjiParams,
	candidates []openAISubscriptionCandidate,
) []resolvedOpenAISubscriptionCredential {
	return h.orderOpenAISubscriptionCandidatesWithRouteKey(ctx, params, candidates, openAISubscriptionRouteKey(ctx, params), codexSessionIdentity{Source: "missing"})
}

func (h *Handlers) orderOpenAISubscriptionCandidatesWithRouteKey(
	ctx context.Context,
	params config.TianjiParams,
	candidates []openAISubscriptionCandidate,
	routeKey string,
	identity codexSessionIdentity,
) []resolvedOpenAISubscriptionCredential {
	selected := h.selectOpenAISubscriptionCandidateWithRouteKey(ctx, params, candidates, routeKey, identity)
	h.logDebugOpenAISubscriptionCodexSelection(ctx, params, selected, candidates, routeKey, identity)
	ordered := make([]resolvedOpenAISubscriptionCredential, 0, len(candidates))
	ordered = append(ordered, selected.resolvedOpenAISubscriptionCredential)
	for _, candidate := range candidates {
		if candidate.CredentialID == selected.CredentialID {
			continue
		}
		ordered = append(ordered, candidate.resolvedOpenAISubscriptionCredential)
	}
	return ordered
}

func (h *Handlers) logDebugOpenAISubscriptionCodexSelection(
	ctx context.Context,
	params config.TianjiParams,
	selected openAISubscriptionCandidate,
	candidates []openAISubscriptionCandidate,
	routeKey string,
	identity codexSessionIdentity,
) {
	if os.Getenv("TIANJI_DEBUG_CODEX_ROUTING") != "1" || !isChatGPTCodexBackendTransport(params) {
		return
	}
	now := h.openAISubscriptionNowUTC()
	metadata := h.codexStickyMetadata(selected.CredentialID, now)
	log.Printf("info: codex-routing-debug: track=%s model=%s selected_credential=%s candidate_count=%d session_source=%s session_present=%t primary_reset=%s secondary_reset_score=%d selection_score=%.6f known=%t available=%t selectable=%t",
		safeOpenAISubscriptionTrackLogValue(routeKey), params.Model, selected.CredentialID, len(candidates), identity.Source, strings.TrimSpace(identity.SessionID) != "", metadata.PrimaryResetAt, metadata.SecondaryResetScore, float64(metadata.Score)/1_000_000, metadata.Known, metadata.Available, metadata.Selectable)
}

func (h *Handlers) selectOpenAISubscriptionCandidateWithRouteKey(
	ctx context.Context,
	params config.TianjiParams,
	candidates []openAISubscriptionCandidate,
	routeKey string,
	identity codexSessionIdentity,
) openAISubscriptionCandidate {
	if len(candidates) <= 1 {
		return candidates[0]
	}
	if h.Config != nil {
		switch h.Config.NativeUpstreamStrategy {
		case config.StrategySticky:
			return h.stickyOpenAISubscriptionSelectWithKey(ctx, params, candidates, routeKey, identity)
		case config.StrategyLowestUtilization:
			if isChatGPTCodexBackendTransport(params) {
				return h.lowestCodexUsageOpenAISubscriptionSelect(candidates)
			}
			return h.lowestUtilizationOpenAISubscriptionSelect(candidates)
		}
	}
	return h.roundRobinOpenAISubscriptionSelect(routeKey, candidates)
}

func (h *Handlers) roundRobinOpenAISubscriptionSelect(key string, candidates []openAISubscriptionCandidate) openAISubscriptionCandidate {
	if len(candidates) == 1 {
		return candidates[0]
	}
	h.openAISubscriptionRoutingMu.Lock()
	defer h.openAISubscriptionRoutingMu.Unlock()
	if h.openAISubscriptionRR == nil {
		h.openAISubscriptionRR = make(map[string]uint64)
	}
	idx := h.openAISubscriptionRR[key]
	h.openAISubscriptionRR[key] = idx + 1
	return candidates[idx%uint64(len(candidates))]
}

func (h *Handlers) stickyOpenAISubscriptionSelectWithKey(
	ctx context.Context,
	params config.TianjiParams,
	candidates []openAISubscriptionCandidate,
	key string,
	identity codexSessionIdentity,
) openAISubscriptionCandidate {
	h.openAISubscriptionRoutingMu.Lock()
	if h.openAISubscriptionSticky == nil {
		h.openAISubscriptionSticky = make(map[string]string)
	}
	stickyID := h.openAISubscriptionSticky[key]
	h.openAISubscriptionRoutingMu.Unlock()
	if stickyID != "" {
		h.stickyCore.SeedIfAbsent(key, stickyStrategyEntry{SelectedID: stickyID})
	}

	now := h.openAISubscriptionNowUTC()
	h.hydrateCachedOpenAISubscriptionCodexUsageForCandidates(ctx, candidates, now)

	strategyCandidates := make([]stickyStrategyCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		metadata := h.codexStickyMetadata(candidate.CredentialID, now)
		strategyCandidates = append(strategyCandidates, stickyStrategyCandidate{
			ID:            candidate.CredentialID,
			Payload:       candidate,
			Metadata:      metadata,
			MetadataScore: metadata.SecondaryResetScore,
		})
	}

	selected, entry, ok := h.stickyCore.Select(key, strategyCandidates, stickyStrategyPolicy{
		CanReuse: codexStickyCanReuseForTrack(key),
		Select: func(strategyCandidates []stickyStrategyCandidate) stickyStrategyCandidate {
			if strings.TrimSpace(identity.SessionID) != "" && isChatGPTCodexBackendTransport(params) {
				return codexRendezvousSelectStrategyCandidates(identity.SessionID, strategyCandidates)
			}
			return selectLowestStickyScore(strategyCandidates, func(candidate stickyStrategyCandidate) (int64, bool) {
				metadata, _ := candidate.Metadata.(codexStickyMetadata)
				return candidate.MetadataScore, !metadata.Known || metadata.Selectable
			})
		},
	})
	if !ok {
		return candidates[0]
	}
	if stickyID != entry.SelectedID {
		metadata, _ := entry.Metadata.(codexStickyMetadata)
		log.Printf("info: codex-sticky: track=%s switched from credential=%s to credential=%s session_source=%s session_present=%t primary_reset=%s secondary_reset_score=%d selection_score=%.6f known=%t available=%t selectable=%t",
			safeOpenAISubscriptionTrackLogValue(key), stickyID, entry.SelectedID, identity.Source, strings.TrimSpace(identity.SessionID) != "", metadata.PrimaryResetAt, metadata.SecondaryResetScore, float64(metadata.Score)/1_000_000, metadata.Known, metadata.Available, metadata.Selectable)
	}
	routed, ok := selected.Payload.(openAISubscriptionCandidate)
	if !ok {
		return candidates[0]
	}

	h.openAISubscriptionRoutingMu.Lock()
	h.openAISubscriptionSticky[key] = entry.SelectedID
	h.openAISubscriptionRoutingMu.Unlock()
	return routed
}

func (h *Handlers) hydrateCachedOpenAISubscriptionCodexUsageForCandidates(
	ctx context.Context,
	candidates []openAISubscriptionCandidate,
	now time.Time,
) {
	usageCache := h.openAISubscriptionCodexUsageCache()
	refreshInterval := h.codexUsageRefreshInterval()
	missingCredentialIDs := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if _, ok := usageCache.getFresh(candidate.CredentialID, now, refreshInterval); !ok {
			missingCredentialIDs = append(missingCredentialIDs, candidate.CredentialID)
		}
	}
	h.hydrateCachedOpenAISubscriptionCodexUsageResults(ctx, missingCredentialIDs, now)
}

func (h *Handlers) lowestCodexUsageOpenAISubscriptionSelect(candidates []openAISubscriptionCandidate) openAISubscriptionCandidate {
	now := h.openAISubscriptionNowUTC()
	var selected openAISubscriptionCandidate
	selectedSet := false
	var bestScore int64
	for _, candidate := range candidates {
		metadata := h.codexStickyMetadata(candidate.CredentialID, now)
		if !metadata.Known || !metadata.Available {
			continue
		}
		if !selectedSet || metadata.Score < bestScore {
			selected = candidate
			selectedSet = true
			bestScore = metadata.Score
		}
	}
	if selectedSet {
		return selected
	}
	return h.lowestUtilizationOpenAISubscriptionSelect(candidates)
}

type codexStickyMetadata struct {
	Known               bool
	Available           bool
	Selectable          bool
	PrimaryResetAt      string
	Score               int64
	SecondaryResetScore int64
}

func (h *Handlers) codexStickyMetadata(credentialID string, now time.Time) codexStickyMetadata {
	cache := h.openAISubscriptionCodexUsageCache()
	result, ok, missReason := cache.getUsableWithReason(credentialID, now)
	if !ok {
		log.Printf("info: codex-sticky: credential=%s usage_cache_miss reason=%s",
			credentialID, missReason)
		return codexUnknownStickyMetadata()
	}
	primaryGate := 0.0
	if h != nil && h.Config != nil {
		primaryGate = h.Config.RatelimitAlertThreshold
	}
	secondaryGate := h.codexUsageWeeklyGate()
	return codexStickyMetadataFromUsageResult(result, primaryGate, secondaryGate, now)
}

func (h *Handlers) codexUsageWeeklyGate() float64 {
	if h != nil && h.Config != nil && h.Config.CodexUsageWeeklyThreshold > 0 {
		return h.Config.CodexUsageWeeklyThreshold
	}
	return chatgptcodex.DefaultSecondaryGate
}

func codexStickyMetadataFromUsageResult(result OpenAISubscriptionCodexUsageResult, primaryGate, secondaryGate float64, now time.Time) codexStickyMetadata {
	score, available, ok := chatgptcodex.SelectionScoreAtWithGates(result.Snapshot, primaryGate, secondaryGate, result.FetchedAt, now)
	if !ok || result.Snapshot.RateLimit == nil {
		return codexUnknownStickyMetadata()
	}
	if primaryGate <= 0 {
		primaryGate = chatgptcodex.DefaultPrimaryGate
	}
	if secondaryGate <= 0 {
		secondaryGate = chatgptcodex.DefaultSecondaryGate
	}
	selectable := available &&
		codexWindowBelowGate(result.Snapshot.RateLimit.PrimaryWindow, primaryGate) &&
		codexWindowBelowGate(result.Snapshot.RateLimit.SecondaryWindow, secondaryGate)
	return codexStickyMetadata{
		Known:               true,
		Available:           available,
		Selectable:          selectable,
		PrimaryResetAt:      codexWindowResetKey(result.Snapshot.RateLimit.PrimaryWindow, result.FetchedAt),
		Score:               int64(score * 1_000_000),
		SecondaryResetScore: codexWindowResetScore(result.Snapshot.RateLimit.SecondaryWindow, result.FetchedAt),
	}
}

func codexUnknownStickyMetadata() codexStickyMetadata {
	return codexStickyMetadata{SecondaryResetScore: math.MaxInt64}
}

func codexWindowBelowGate(window chatgptcodex.UsageWindow, gate float64) bool {
	if gate <= 0 {
		return true
	}
	if window.UsedPercent == nil {
		return false
	}
	return *window.UsedPercent < gate
}

func codexWindowResetKey(window chatgptcodex.UsageWindow, fetchedAt time.Time) string {
	if reset, ok := codexWindowResetUnix(window, fetchedAt); ok {
		return strconv.FormatInt(reset, 10)
	}
	return strings.TrimSpace(window.ResetAt)
}

func codexWindowResetScore(window chatgptcodex.UsageWindow, fetchedAt time.Time) int64 {
	if reset, ok := codexWindowResetUnix(window, fetchedAt); ok {
		return reset
	}
	return math.MaxInt64
}

func codexWindowResetUnix(window chatgptcodex.UsageWindow, fetchedAt time.Time) (int64, bool) {
	resetAt := strings.TrimSpace(window.ResetAt)
	if resetAt != "" {
		if ts, err := strconv.ParseInt(resetAt, 10, 64); err == nil {
			return ts, true
		}
		if ts, err := time.Parse(time.RFC3339, resetAt); err == nil {
			return ts.Unix(), true
		}
	}
	if window.ResetAfterSeconds != nil && !fetchedAt.IsZero() {
		return fetchedAt.Add(time.Duration(*window.ResetAfterSeconds) * time.Second).Unix(), true
	}
	return 0, false
}

func codexStickyCanReuseForTrack(trackKey string) func(stickyStrategyEntry, stickyStrategyCandidate) bool {
	return func(entry stickyStrategyEntry, candidate stickyStrategyCandidate) bool {
		return codexStickyCanReuseWithTrack(trackKey, entry, candidate)
	}
}

func codexStickyCanReuseWithTrack(trackKey string, entry stickyStrategyEntry, candidate stickyStrategyCandidate) bool {
	current, _ := candidate.Metadata.(codexStickyMetadata)
	if !current.Known {
		previous, _ := entry.Metadata.(codexStickyMetadata)
		reuse := previous.PrimaryResetAt == ""
		if !reuse && trackKey != "" {
			log.Printf("info: codex-sticky: track=%s re-evaluating credential=%s reason=usage_unknown previous_primary_reset=%s",
				safeOpenAISubscriptionTrackLogValue(trackKey), candidate.ID, previous.PrimaryResetAt)
		}
		return reuse
	}
	if !current.Selectable {
		if trackKey != "" {
			log.Printf("info: codex-sticky: track=%s re-evaluating credential=%s reason=not_selectable primary_reset=%s secondary_reset_score=%d selection_score=%.6f known=%t available=%t selectable=%t",
				safeOpenAISubscriptionTrackLogValue(trackKey), candidate.ID, current.PrimaryResetAt, current.SecondaryResetScore, float64(current.Score)/1_000_000, current.Known, current.Available, current.Selectable)
		}
		return false
	}
	previous, _ := entry.Metadata.(codexStickyMetadata)
	if previous.PrimaryResetAt == "" {
		return true
	}
	if isOpenAISubscriptionCodexSessionRouteKey(trackKey) {
		return true
	}
	reuse := previous.PrimaryResetAt == current.PrimaryResetAt
	if !reuse && trackKey != "" {
		log.Printf("info: codex-sticky: track=%s re-evaluating credential=%s reason=primary_reset_changed previous_primary_reset=%s current_primary_reset=%s secondary_reset_score=%d selection_score=%.6f",
			safeOpenAISubscriptionTrackLogValue(trackKey), candidate.ID, previous.PrimaryResetAt, current.PrimaryResetAt, current.SecondaryResetScore, float64(current.Score)/1_000_000)
	}
	return reuse
}

func (h *Handlers) lowestUtilizationOpenAISubscriptionSelect(candidates []openAISubscriptionCandidate) openAISubscriptionCandidate {
	var selected openAISubscriptionCandidate
	selectedSet := false
	bestScore := -1
	for _, candidate := range candidates {
		state, ok := h.openAISubscriptionRateLimitState(candidate.CredentialID)
		if !ok {
			if !selectedSet {
				selected = candidate
				selectedSet = true
			}
			continue
		}
		score := openAIQuotaCapacityScore(state)
		if !selectedSet || score > bestScore {
			selected = candidate
			selectedSet = true
			bestScore = score
		}
	}
	if selectedSet {
		return selected
	}
	return candidates[0]
}

func openAIQuotaCapacityScore(state callback.OpenAIQuotaState) int {
	score := 0
	if state.Requests.RemainingKnown {
		score += state.Requests.Remaining
	}
	if state.Tokens.RemainingKnown {
		score += state.Tokens.Remaining
	}
	if state.QuotaUtilizationKnown {
		score += int((1 - state.QuotaUtilization) * 1000)
	}
	return score
}

func openAISubscriptionRouteKey(ctx context.Context, params config.TianjiParams) string {
	orgScope := "__master__"
	if ctx != nil {
		if orgID, ok := ctx.Value(middleware.ContextKeyOrgID).(string); ok && orgID != "" {
			orgScope = "org:" + orgID
		}
	}
	return openAISubscriptionProviderKey + ":" + orgScope + ":" + openAISubscriptionRouteClass(params.Model)
}

func openAISubscriptionCodexSessionRouteKey(ctx context.Context, params config.TianjiParams, identity codexSessionIdentity) string {
	sessionID := strings.TrimSpace(identity.SessionID)
	if sessionID == "" {
		return openAISubscriptionRouteKey(ctx, params)
	}
	orgScope := "__master__"
	if ctx != nil {
		if orgID, ok := ctx.Value(middleware.ContextKeyOrgID).(string); ok && orgID != "" {
			orgScope = "org:" + orgID
		}
	}
	return openAISubscriptionProviderKey + ":" + orgScope + ":codex-session:" + sessionID + ":" + openAISubscriptionRouteClass(params.Model)
}

func isOpenAISubscriptionCodexSessionRouteKey(trackKey string) bool {
	const (
		providerPrefix = openAISubscriptionProviderKey + ":"
		sessionPrefix  = "codex-session:"
		routeClassMark = ":model:"
	)
	rest, ok := strings.CutPrefix(trackKey, providerPrefix)
	if !ok {
		return false
	}
	switch {
	case strings.HasPrefix(rest, "__master__:"):
		rest = strings.TrimPrefix(rest, "__master__:")
	case strings.HasPrefix(rest, "org:"):
		_, afterOrg, ok := strings.Cut(rest[len("org:"):], ":")
		if !ok {
			return false
		}
		rest = afterOrg
	default:
		return false
	}
	if !strings.HasPrefix(rest, sessionPrefix) {
		return false
	}
	return strings.LastIndex(rest, routeClassMark) > len(sessionPrefix)
}

func safeOpenAISubscriptionTrackLogValue(trackKey string) string {
	const marker = ":codex-session:"
	idx := strings.Index(trackKey, marker)
	if idx < 0 {
		return trackKey
	}
	sessionStart := idx + len(marker)
	routeClassStart := strings.LastIndex(trackKey, ":model:")
	if routeClassStart <= sessionStart {
		return trackKey[:sessionStart] + "redacted"
	}
	sessionID := trackKey[sessionStart:routeClassStart]
	sum := sha256.Sum256([]byte(sessionID))
	return trackKey[:sessionStart] + "sha256:" + fmt.Sprintf("%x", sum[:8]) + trackKey[routeClassStart:]
}

func openAISubscriptionRouteClass(model string) string {
	if strings.TrimSpace(model) == "" {
		return "model:unknown"
	}
	return "model:" + model
}

func extractCodexSessionIdentity(payload map[string]any) codexSessionIdentity {
	metadata := codexClientMetadata(payload)
	if len(metadata) == 0 {
		return codexSessionIdentity{Source: "missing"}
	}
	if sessionID, ok := stringMapValue(metadata, "session_id"); ok {
		if normalized := strings.TrimSpace(sessionID); normalized != "" {
			return codexSessionIdentity{SessionID: normalized, Source: "direct"}
		}
	}
	if rawTurnMetadata, ok := stringMapValue(metadata, "x-codex-turn-metadata"); ok && strings.TrimSpace(rawTurnMetadata) != "" {
		var turnMetadata map[string]any
		if err := json.Unmarshal([]byte(rawTurnMetadata), &turnMetadata); err != nil {
			return codexSessionIdentity{Source: "parse_error"}
		}
		if sessionID, ok := stringMapValue(turnMetadata, "session_id"); ok {
			if normalized := strings.TrimSpace(sessionID); normalized != "" {
				return codexSessionIdentity{SessionID: normalized, Source: "x-codex-turn-metadata"}
			}
		}
	}
	return codexSessionIdentity{Source: "missing"}
}

func codexClientMetadata(payload map[string]any) map[string]any {
	if payload == nil {
		return nil
	}
	if raw, ok := payload["client_metadata"]; ok {
		if metadata, ok := raw.(map[string]any); ok {
			return metadata
		}
		if rawJSON, ok := raw.(json.RawMessage); ok {
			var metadata map[string]any
			if json.Unmarshal(rawJSON, &metadata) == nil {
				return metadata
			}
		}
	}
	return payload
}

func stringMapValue(values map[string]any, key string) (string, bool) {
	if values == nil {
		return "", false
	}
	raw, ok := values[key]
	if !ok {
		return "", false
	}
	value, ok := raw.(string)
	return value, ok
}

func codexRendezvousSelect(sessionID string, candidates []openAISubscriptionCandidate) openAISubscriptionCandidate {
	if len(candidates) == 0 {
		return openAISubscriptionCandidate{}
	}
	selected := candidates[0]
	var bestScore uint64
	bestSet := false
	for _, candidate := range candidates {
		score := codexRendezvousScore(sessionID, candidate.CredentialID)
		if !bestSet || score > bestScore || (score == bestScore && candidate.CredentialID < selected.CredentialID) {
			selected = candidate
			bestScore = score
			bestSet = true
		}
	}
	return selected
}

func codexRendezvousSelectStrategyCandidates(sessionID string, candidates []stickyStrategyCandidate) stickyStrategyCandidate {
	selectable := make([]stickyStrategyCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		metadata, _ := candidate.Metadata.(codexStickyMetadata)
		if !metadata.Known || metadata.Selectable {
			selectable = append(selectable, candidate)
		}
	}
	if len(selectable) == 0 {
		return candidates[0]
	}
	selected := selectable[0]
	var bestScore uint64
	bestSet := false
	for _, candidate := range selectable {
		score := codexRendezvousScore(sessionID, candidate.ID)
		if !bestSet || score > bestScore || (score == bestScore && candidate.ID < selected.ID) {
			selected = candidate
			bestScore = score
			bestSet = true
		}
	}
	return selected
}

func codexRendezvousScore(sessionID, credentialID string) uint64 {
	sum := sha256.Sum256([]byte(strings.TrimSpace(sessionID) + "\x00" + strings.TrimSpace(credentialID)))
	return binary.BigEndian.Uint64(sum[:8])
}

func openAISubscriptionFailure(credentialID string, err error) OpenAISubscriptionCredentialError {
	var typed *OpenAISubscriptionCredentialError
	if errors.As(err, &typed) {
		return *typed
	}
	return OpenAISubscriptionCredentialError{
		CredentialID: credentialID,
		Code:         OpenAISubscriptionCredentialLookupErr,
		Message:      redact.String(err.Error()),
	}
}

func openAISubscriptionRetryableStatus(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || statusCode >= http.StatusInternalServerError
}

func openAISubscriptionHTTPReasonCode(statusCode int) string {
	switch statusCode {
	case http.StatusTooManyRequests:
		return "rate_limited"
	case http.StatusUnauthorized:
		return string(OpenAISubscriptionCredentialAuthErr)
	default:
		if statusCode >= http.StatusInternalServerError {
			return "upstream_server_error"
		}
		if statusCode >= http.StatusBadRequest {
			return "upstream_client_error"
		}
		return ""
	}
}
