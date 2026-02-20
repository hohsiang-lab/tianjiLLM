package scheduler

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/policy"
)

// BudgetResetJob resets spend for all keys whose budget_reset_at has passed.
// Uses a single batch UPDATE for efficiency.
type BudgetResetJob struct {
	DB *db.Queries
}

func (j *BudgetResetJob) Name() string { return "budget_reset" }

func (j *BudgetResetJob) Run(ctx context.Context) error {
	return j.DB.ResetBudgetForExpiredTokens(ctx)
}

// SpendLogCleanupJob deletes spend log entries older than the retention period.
type SpendLogCleanupJob struct {
	DB        *db.Queries
	Retention time.Duration // e.g., 90 days
}

func (j *SpendLogCleanupJob) Name() string { return "spend_log_cleanup" }

func (j *SpendLogCleanupJob) Run(ctx context.Context) error {
	cutoff := time.Now().Add(-j.Retention)
	return j.DB.DeleteOldSpendLogs(ctx, pgtype.Timestamptz{Time: cutoff, Valid: true})
}

// PolicyHotReloadJob reloads policies from DB into the in-memory engine.
type PolicyHotReloadJob struct {
	Engine *policy.Engine
}

func (j *PolicyHotReloadJob) Name() string { return "policy_hot_reload" }

func (j *PolicyHotReloadJob) Run(ctx context.Context) error {
	return j.Engine.Load(ctx)
}

// SpendArchivalJob archives old spend logs to cold storage.
type SpendArchivalJob struct {
	Archiver  SpendArchiver
	Retention time.Duration // e.g., 90 days — archive logs older than this
}

// SpendArchiver is the interface the spend.Archiver satisfies.
type SpendArchiver interface {
	Archive(ctx context.Context, from, to time.Time) error
}

func (j *SpendArchivalJob) Name() string { return "spend_archival" }

func (j *SpendArchivalJob) Run(ctx context.Context) error {
	to := time.Now().Add(-j.Retention)
	from := to.AddDate(0, -1, 0) // archive one month at a time
	return j.Archiver.Archive(ctx, from, to)
}

// SpendBatchWriteJob flushes buffered spend logs to the database.
type SpendBatchWriteJob struct {
	Flusher SpendFlusher
}

// SpendFlusher is satisfied by spend.RedisBuffer or any buffered writer.
type SpendFlusher interface {
	Flush()
}

func (j *SpendBatchWriteJob) Name() string { return "spend_batch_write" }

func (j *SpendBatchWriteJob) Run(_ context.Context) error {
	j.Flusher.Flush()
	return nil
}

// CredentialRefreshJob reloads provider credentials from DB.
type CredentialRefreshJob struct {
	DB *db.Queries
}

func (j *CredentialRefreshJob) Name() string { return "credential_refresh" }

func (j *CredentialRefreshJob) Run(ctx context.Context) error {
	creds, err := j.DB.ListCredentials(ctx)
	if err != nil {
		return err
	}
	log.Printf("scheduler: credential_refresh: loaded %d credentials", len(creds))
	return nil
}

// KeyRotationJob checks for keys that need rotation based on expiry.
type KeyRotationJob struct {
	DB *db.Queries
}

func (j *KeyRotationJob) Name() string { return "key_rotation" }

func (j *KeyRotationJob) Run(ctx context.Context) error {
	expired, err := j.DB.ListExpiredTokens(ctx)
	if err != nil {
		return err
	}
	if len(expired) > 0 {
		log.Printf("scheduler: key_rotation: %d expired keys found", len(expired))
	}
	return nil
}

// KeyFetcher fetches a new API key for a given provider/credential name.
type KeyFetcher interface {
	FetchKey(ctx context.Context, credentialName string) (string, error)
}

// KeySwapper atomically swaps an API key for a provider.
type KeySwapper interface {
	SwapKey(credentialName, newKey string)
}

// ProviderKeyRotationJob periodically fetches fresh API keys from an external source
// (e.g., vault, secrets manager) and swaps them atomically in the provider config.
type ProviderKeyRotationJob struct {
	Fetcher     KeyFetcher
	Swapper     KeySwapper
	Credentials []string // credential names to rotate
}

func (j *ProviderKeyRotationJob) Name() string { return "provider_key_rotation" }

func (j *ProviderKeyRotationJob) Run(ctx context.Context) error {
	for _, cred := range j.Credentials {
		newKey, err := j.Fetcher.FetchKey(ctx, cred)
		if err != nil {
			log.Printf("scheduler: provider_key_rotation: failed to fetch key for %s: %v", cred, err)
			continue
		}
		j.Swapper.SwapKey(cred, newKey)
		log.Printf("scheduler: provider_key_rotation: rotated key for %s", cred)
	}
	return nil
}

// HealthCheckJob probes deployment endpoints and logs failures.
type HealthCheckJob struct {
	Endpoints []string
	Client    *http.Client
}

func (j *HealthCheckJob) Name() string { return "health_check" }

func (j *HealthCheckJob) Run(ctx context.Context) error {
	for _, endpoint := range j.Endpoints {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			log.Printf("scheduler: health_check: bad endpoint %s: %v", endpoint, err)
			continue
		}

		resp, err := j.Client.Do(req)
		if err != nil {
			log.Printf("scheduler: health_check: %s unreachable: %v", endpoint, err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 500 {
			log.Printf("scheduler: health_check: %s returned %d", endpoint, resp.StatusCode)
		}
	}
	return nil
}
