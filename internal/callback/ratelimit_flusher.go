package callback

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

// ErrRateLimitStateNotFound is returned by RateLimitDB.GetOAuthTokenRateLimitState
// when no row exists for the given token key. Callers must not import pgx directly.
var ErrRateLimitStateNotFound = errors.New("rate limit state not found")

// RateLimitDB is the subset of db.Queries needed by the flusher/preload.
type RateLimitDB interface {
	UpsertOAuthTokenRateLimitState(ctx context.Context, arg db.UpsertOAuthTokenRateLimitStateParams) error
	GetActiveOAuthTokenRateLimitStates(ctx context.Context) ([]db.OAuthTokenRateLimitState, error)
	// GetOAuthTokenRateLimitState fetches a single token's state by key.
	// Returns ErrRateLimitStateNotFound when no row exists.
	// Used by lowestUtilizationSelect to recover state after PruneExpired.
	GetOAuthTokenRateLimitState(ctx context.Context, tokenKey string) (db.OAuthTokenRateLimitState, error)
}

// DBWithRateLimitOps is the subset of db.Store needed to construct a RateLimitDB adapter.
type DBWithRateLimitOps interface {
	UpsertOAuthTokenRateLimitState(ctx context.Context, arg db.UpsertOAuthTokenRateLimitStateParams) error
	GetActiveOAuthTokenRateLimitStates(ctx context.Context) ([]db.OAuthTokenRateLimitState, error)
	GetOAuthTokenRateLimitState(ctx context.Context, tokenKey string) (db.OAuthTokenRateLimitState, error)
}

// rateLimitDBAdapter wraps a DBWithRateLimitOps and translates pgx.ErrNoRows
// to ErrRateLimitStateNotFound, keeping pgx out of the handler layer.
type rateLimitDBAdapter struct {
	q DBWithRateLimitOps
}

// NewRateLimitDB wraps a DBWithRateLimitOps so callers never need to import pgx
// to distinguish "not found" from other DB errors.
// Returns nil if q is nil, preserving the nil-check semantics callers rely on.
func NewRateLimitDB(q DBWithRateLimitOps) RateLimitDB {
	if q == nil {
		return nil
	}
	return &rateLimitDBAdapter{q: q}
}

func (a *rateLimitDBAdapter) UpsertOAuthTokenRateLimitState(ctx context.Context, arg db.UpsertOAuthTokenRateLimitStateParams) error {
	return a.q.UpsertOAuthTokenRateLimitState(ctx, arg)
}

func (a *rateLimitDBAdapter) GetActiveOAuthTokenRateLimitStates(ctx context.Context) ([]db.OAuthTokenRateLimitState, error) {
	return a.q.GetActiveOAuthTokenRateLimitStates(ctx)
}

func (a *rateLimitDBAdapter) GetOAuthTokenRateLimitState(ctx context.Context, tokenKey string) (db.OAuthTokenRateLimitState, error) {
	row, err := a.q.GetOAuthTokenRateLimitState(ctx, tokenKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.OAuthTokenRateLimitState{}, ErrRateLimitStateNotFound
	}
	return row, err
}

// PreloadRateLimitStore loads rate limit states with active 7d windows from DB
// into the in-memory store. A token's data is "active" when its 7d reset time
// is still in the future — meaning Anthropic has not yet zeroed the counters.
// Called once at startup. Errors are logged but do not prevent the server from starting.
func PreloadRateLimitStore(ctx context.Context, rdb RateLimitDB, store *InMemoryRateLimitStore) {
	rows, err := rdb.GetActiveOAuthTokenRateLimitStates(ctx)
	if err != nil {
		log.Printf("warn: ratelimit-preload: %v", err)
		return
	}

	for _, row := range rows {
		state := dbRowToState(row)
		store.SetClean(row.TokenKey, state)
	}

	log.Printf("ratelimit-preload: loaded %d token states from DB", len(rows))
}

// RateLimitFlusher periodically writes dirty rate limit state to the database.
type RateLimitFlusher struct {
	store    *InMemoryRateLimitStore
	db       RateLimitDB
	interval time.Duration
}

// NewRateLimitFlusher creates a flusher that writes dirty entries every interval.
func NewRateLimitFlusher(store *InMemoryRateLimitStore, rdb RateLimitDB, interval time.Duration) *RateLimitFlusher {
	return &RateLimitFlusher{store: store, db: rdb, interval: interval}
}

// Start runs the flush loop. Blocks until ctx is cancelled, then performs a final flush.
func (f *RateLimitFlusher) Start(ctx context.Context) {
	ticker := time.NewTicker(f.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			f.flush(ctx)
		case <-ctx.Done():
			shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			f.flush(shutCtx)
			cancel()
			return
		}
	}
}

// flush writes all dirty entries to DB. On failure, dirty keys are retained for next attempt.
// Note: if the process crashes between SnapshotDirty (which clears the dirty set) and
// MarkDirty (which re-marks failed keys), those keys' state is lost. This is an inherent
// trade-off of non-transactional memory-to-DB sync; the next preload will recover the
// most recent successfully flushed state.
func (f *RateLimitFlusher) flush(ctx context.Context) {
	snap := f.store.SnapshotDirty()
	if len(snap) == 0 {
		return
	}

	var failed []string
	var firstErr error
	for key, state := range snap {
		// Skip remaining upserts if context is already done — further calls will fail
		// instantly anyway, so avoid the overhead of N more failed DB round-trips.
		if ctx.Err() != nil {
			failed = append(failed, key)
			continue
		}
		if err := f.db.UpsertOAuthTokenRateLimitState(ctx, stateToUpsertParams(key, state)); err != nil {
			failed = append(failed, key)
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	if len(failed) > 0 {
		f.store.MarkDirty(failed)
		// If the context expired (shutdown timeout), some keys are permanently lost
		// since the process will exit after flush returns. Log at ERROR to alert operators.
		if ctx.Err() != nil {
			log.Printf("ERROR: ratelimit-flush: shutdown timeout: %d/%d keys NOT persisted to DB (lost on exit), first error: %v", len(failed), len(snap), firstErr)
		} else {
			log.Printf("warn: ratelimit-flush: %d/%d keys failed (first error: %v), will retry", len(failed), len(snap), firstErr)
		}
	}

	if flushed := len(snap) - len(failed); flushed > 0 {
		log.Printf("ratelimit-flush: wrote %d token states to DB", flushed)
	}
}

func stateToUpsertParams(key string, s AnthropicOAuthRateLimitState) db.UpsertOAuthTokenRateLimitStateParams {
	return db.UpsertOAuthTokenRateLimitStateParams{
		TokenKey:                   key,
		UnifiedStatus:              s.UnifiedStatus,
		UnifiedReset:               s.UnifiedReset,
		Unified5hStatus:            s.Unified5hStatus,
		Unified5hUtilization:       s.Unified5hUtilization,
		Unified5hReset:             s.Unified5hReset,
		Unified7dStatus:            s.Unified7dStatus,
		Unified7dUtilization:       s.Unified7dUtilization,
		Unified7dReset:             s.Unified7dReset,
		Unified7dSonnetStatus:      s.Unified7dSonnetStatus,
		Unified7dSonnetUtilization: s.Unified7dSonnetUtilization,
		Unified7dSonnetReset:       s.Unified7dSonnetReset,
		OrgID:                      s.OrganizationID,
	}
}

// DBRowToState converts a DB row into an in-memory rate limit state.
// Exported so lowestUtilizationSelect can reuse it when loading from DB.
func DBRowToState(row db.OAuthTokenRateLimitState) AnthropicOAuthRateLimitState {
	return dbRowToState(row)
}

func dbRowToState(row db.OAuthTokenRateLimitState) AnthropicOAuthRateLimitState {
	clampField := func(fieldName string, v float64) float64 {
		clamped := clampUtilization(v)
		if v != -1 && clamped != v {
			log.Printf("warn: ratelimit: DB value for %s out of range (got %.4f), clamped to [0,1]", fieldName, v)
		}
		return clamped
	}
	return AnthropicOAuthRateLimitState{
		TokenKey:                   row.TokenKey,
		UnifiedStatus:              row.UnifiedStatus,
		UnifiedReset:               row.UnifiedReset,
		Unified5hStatus:            row.Unified5hStatus,
		Unified5hUtilization:       clampField("unified_5h_utilization", row.Unified5hUtilization),
		Unified5hReset:             row.Unified5hReset,
		Unified7dStatus:            row.Unified7dStatus,
		Unified7dUtilization:       clampField("unified_7d_utilization", row.Unified7dUtilization),
		Unified7dReset:             row.Unified7dReset,
		Unified7dSonnetStatus:      row.Unified7dSonnetStatus,
		Unified7dSonnetUtilization: clampField("unified_7d_sonnet_utilization", row.Unified7dSonnetUtilization),
		Unified7dSonnetReset:       row.Unified7dSonnetReset,
		OrganizationID:             row.OrgID,
		ParsedAt:                   row.UpdatedAt.Time,
	}
}

// PreloadOAuthUsageStore populates the OAuthUsageStore from the DB at startup,
// so the Claude Code UI tab shows utilization data without waiting for a live request.
// Only writes entries that are not already present (live data from a racing request wins).
func PreloadOAuthUsageStore(ctx context.Context, rdb RateLimitDB, store OAuthUsageStore) {
	rows, err := rdb.GetActiveOAuthTokenRateLimitStates(ctx)
	if err != nil {
		log.Printf("warn: oauth-usage-preload: %v", err)
		return
	}
	loaded := 0
	for _, row := range rows {
		if _, ok := store.Get(row.TokenKey); ok {
			// Live data already present — don't overwrite with stale DB value.
			continue
		}
		state := dbRowToState(row)
		store.SetFromHeaders(row.TokenKey, state)
		loaded++
	}
	log.Printf("oauth-usage-preload: fetched %d rows from DB, loaded %d (skipped %d live entries)",
		len(rows), loaded, len(rows)-loaded)
}
