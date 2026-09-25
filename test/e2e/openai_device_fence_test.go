//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/praxisllmlab/tianjiLLM/internal/cache"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/openaioauth"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/praxisllmlab/tianjiLLM/internal/testutil/openaitest"
)

// These run in the existing PG16/Playwright E2E job. TestMain is destructive:
// compile-only outside an explicitly owned, isolated E2E database.
func TestOpenAIDeviceFenceNativeRecoveryBeforeInsert(t *testing.T) {
	h, c, provider := newNativeDeviceFence(t)
	record := startNativeDeviceFence(t, h)
	entered, release := make(chan struct{}), make(chan struct{})
	unblock := nativeFenceRelease(release)
	defer unblock()
	base := h.OpenAIOAuthHTTPClient.Transport
	h.OpenAIOAuthHTTPClient = &http.Client{Transport: nativeFenceTransport(func(req *http.Request) (*http.Response, error) {
		resp, err := base.RoundTrip(req)
		if err == nil && req.URL.Path == "/oauth/token" {
			resp.Body = &nativeFenceResponseBarrier{resp.Body, entered, release}
		}
		return resp, err
	})}
	done := make(chan error, 1)
	go func() {
		_, err := h.PollOpenAIDeviceAuth(context.Background(), record.FlowID, record.SessionBinding)
		done <- err
	}()
	nativeFenceWait(t, entered)
	require.Zero(t, nativeFenceRowCount(t, record.FlowID))
	// Model lease expiry, never rewrite immutable expiry/session/org identity.
	current, err := openaioauth.NewDeviceStore(c).Get(context.Background(), record.FlowID)
	require.NoError(t, err)
	current.UpdatedAt = time.Now().UTC().Add(-2*time.Minute - time.Second)
	body, err := json.Marshal(current)
	require.NoError(t, err)
	require.NoError(t, c.Set(context.Background(), openaioauth.DeviceAuthCacheKey(record.FlowID), body, time.Until(record.ExpiresAt)+time.Minute))
	c.expireLease(t, record.FlowID)
	recovered, err := h.PollOpenAIDeviceAuth(context.Background(), record.FlowID, record.SessionBinding)
	require.NoError(t, err)
	require.Equal(t, openaioauth.DeviceAuthStatusFailed, recovered.Status)
	require.Zero(t, nativeFenceRowCount(t, record.FlowID))
	unblock()
	require.NoError(t, nativeFenceResult(t, done))
	final, err := h.PollOpenAIDeviceAuth(context.Background(), record.FlowID, record.SessionBinding)
	require.NoError(t, err)
	assert.Equal(t, openaioauth.DeviceAuthStatusFailed, final.Status)
	assert.Equal(t, "exchange_interrupted", final.ErrorCode)
	assert.Zero(t, nativeFenceRowCount(t, record.FlowID))
	assertNativeFenceProviderCalls(t, provider, 1, 1)
}

func TestOpenAIDeviceFenceNativeCommitArbitration(t *testing.T) {
	for _, mode := range []string{"commit_across_expiry", "rollback", "lost_commit_ack"} {
		t.Run(mode, func(t *testing.T) {
			h, c, provider := newNativeDeviceFence(t)
			record := startNativeDeviceFence(t, h)
			if mode == "commit_across_expiry" {
				record.ExpiresAt = time.Now().UTC().Add(time.Second)
				body, err := json.Marshal(record)
				require.NoError(t, err)
				require.NoError(t, c.Set(context.Background(), openaioauth.DeviceAuthCacheKey(record.FlowID), body, time.Minute+time.Second))
			}
			entered, release := make(chan struct{}), make(chan struct{})
			unblock := nativeFenceRelease(release)
			defer unblock()
			lockAttempts := make(chan struct{}, 16)
			h.DB = db.New(&nativeFenceDB{Pool: testPool, wrap: func(tx pgx.Tx) pgx.Tx {
				return &nativeFenceTx{Tx: tx, beforeLock: func() { lockAttempts <- struct{}{} }, commit: func(ctx context.Context, tx pgx.Tx) error {
					close(entered)
					<-release
					if mode == "rollback" {
						if err := tx.Rollback(ctx); err != nil {
							return err
						}
						return context.Canceled
					}
					if err := tx.Commit(ctx); err != nil {
						return err
					}
					if mode == "lost_commit_ack" {
						return context.DeadlineExceeded
					}
					return nil
				}}
			}})
			writer := make(chan error, 1)
			go func() {
				_, err := h.PollOpenAIDeviceAuth(context.Background(), record.FlowID, record.SessionBinding)
				writer <- err
			}()
			nativeFenceWait(t, entered)
			// INSERT already ran in the owning tx, but is not visible or successful yet.
			require.Zero(t, nativeFenceRowCount(t, record.FlowID))
			var available bool
			err := testPool.QueryRow(context.Background(), "SELECT pg_try_advisory_xact_lock(hashtextextended($1, 0))", "tianji/openai/device/"+record.FlowID).Scan(&available)
			require.NoError(t, err)
			require.False(t, available, "INSERT must retain the same transaction's flow lock")
			for len(lockAttempts) > 0 {
				<-lockAttempts
			}
			observer := make(chan error, 1)
			go func() {
				_, err := h.PollOpenAIDeviceAuth(context.Background(), record.FlowID, record.SessionBinding)
				observer <- err
			}()
			nativeFenceWait(t, lockAttempts)
			select {
			case <-observer:
				t.Fatal("reconciliation escaped the uncommitted writer's flow lock")
			case <-time.After(30 * time.Millisecond):
			}
			if mode == "commit_across_expiry" {
				time.Sleep(max(0, time.Until(record.ExpiresAt)) + time.Millisecond)
			}
			unblock()
			require.NoError(t, nativeFenceResult(t, writer))
			require.NoError(t, nativeFenceResult(t, observer))
			for range 2 {
				status, err := h.PollOpenAIDeviceAuth(context.Background(), record.FlowID, record.SessionBinding)
				require.NoError(t, err)
				if mode == "rollback" {
					assert.Equal(t, openaioauth.DeviceAuthStatusFailed, status.Status)
					assert.Zero(t, nativeFenceRowCount(t, record.FlowID))
				} else {
					assert.Equal(t, openaioauth.DeviceAuthStatusSuccess, status.Status)
					assert.Equal(t, 1, nativeFenceRowCount(t, record.FlowID))
				}
			}
			assertNativeFenceProviderCalls(t, provider, 1, 1)
		})
	}
}

func TestOpenAIDeviceFenceNativeLostTransactionDelayedAdmission(t *testing.T) {
	h, c, provider := newNativeDeviceFence(t)
	record := startNativeDeviceFence(t, h)
	var active atomic.Pointer[nativeFenceTx]
	h.DB = db.New(&nativeFenceDB{Pool: testPool, wrap: func(tx pgx.Tx) pgx.Tx {
		wrapped := &nativeFenceTx{Tx: tx}
		active.Store(wrapped)
		return wrapped
	}})
	var original openaioauth.DeviceAuthRecord
	c.beforeCAS = func(before, after openaioauth.DeviceAuthRecord) {
		if strings.HasPrefix(after.Generation, "admitted:") && after.Generation != before.Generation {
			original = before
			// Terminate only this fixture's exact transaction connection; PG16 waits
			// for its rollback/lock release before the delayed cache CAS is delivered.
			pid := active.Load().Conn().PgConn().PID()
			var terminated bool
			err := testPool.QueryRow(context.Background(), "SELECT pg_terminate_backend($1, 5000)", pid).Scan(&terminated)
			require.NoError(t, err)
			require.True(t, terminated)
		}
	}
	status, err := h.PollOpenAIDeviceAuth(context.Background(), record.FlowID, record.SessionBinding)
	require.NoError(t, err)
	assert.Equal(t, openaioauth.DeviceAuthStatusFailed, status.Status)
	require.NotEmpty(t, original.Generation)
	_, err = h.SaveOpenAISubscriptionCredentialForFlow(context.Background(), &original, "OpenAI Subscription", nativeFenceBundle(), handler.OpenAISubscriptionCredentialInfo{})
	require.Error(t, err, "the original owner must not reopen an INSERT after transaction loss")
	assert.Zero(t, nativeFenceRowCount(t, record.FlowID))
	assertNativeFenceProviderCalls(t, provider, 1, 1)
}

func TestOpenAIDeviceFenceNativeLostRevokerDelayedCAS(t *testing.T) {
	h, c, provider := newNativeDeviceFence(t)
	record := startNativeDeviceFence(t, h)
	inserted, finishInsert := make(chan struct{}), make(chan struct{})
	unblockInsert := nativeFenceRelease(finishInsert)
	defer unblockInsert()
	var active atomic.Pointer[nativeFenceTx]
	h.DB = db.New(&nativeFenceDB{Pool: testPool, wrap: func(tx pgx.Tx) pgx.Tx {
		wrapped := &nativeFenceTx{Tx: tx, commit: func(ctx context.Context, tx pgx.Tx) error { close(inserted); <-finishInsert; return tx.Commit(ctx) }}
		active.Store(wrapped)
		return wrapped
	}})
	entered, release := make(chan struct{}), make(chan struct{})
	unblock := nativeFenceRelease(release)
	defer unblock()
	c.beforeCAS = func(_, after openaioauth.DeviceAuthRecord) {
		if after.Status == openaioauth.DeviceAuthStatusCancelled {
			require.NoError(t, active.Load().Rollback(context.Background()))
			close(entered)
			<-release
		}
	}
	cancelled := make(chan error, 1)
	go func() {
		cancelled <- h.CancelOpenAIDeviceAuth(context.Background(), record.FlowID, record.SessionBinding)
	}()
	nativeFenceWait(t, entered)
	// A lost Redis lease must not be mistaken for the durable fence.
	c.expireLease(t, record.FlowID)
	writer := make(chan error, 1)
	go func() {
		_, err := h.PollOpenAIDeviceAuth(context.Background(), record.FlowID, record.SessionBinding)
		writer <- err
	}()
	nativeFenceWait(t, inserted)
	require.Zero(t, nativeFenceRowCount(t, record.FlowID), "admitted row must still be uncommitted")
	unblock()
	assert.Error(t, nativeFenceResult(t, cancelled))
	admitted, err := openaioauth.NewDeviceStore(c).Get(context.Background(), record.FlowID)
	require.NoError(t, err)
	assert.Equal(t, openaioauth.DeviceAuthStatusExchanging, admitted.Status)
	assert.True(t, strings.HasPrefix(admitted.Generation, "admitted:"))
	unblockInsert()
	require.NoError(t, nativeFenceResult(t, writer))
	final, err := h.PollOpenAIDeviceAuth(context.Background(), record.FlowID, record.SessionBinding)
	require.NoError(t, err)
	assert.Equal(t, openaioauth.DeviceAuthStatusSuccess, final.Status)
	assert.Equal(t, 1, nativeFenceRowCount(t, record.FlowID))
	assertNativeFenceProviderCalls(t, provider, 1, 1)
}

func newNativeDeviceFence(t *testing.T) (*handler.Handlers, *nativeFenceCache, *openaitest.OAuthServer) {
	t.Helper()
	provider := configureOpenAIDeviceOAuthMock(t, openaitest.DeviceAuthOptions{PollResponses: []openaitest.DevicePollResponse{{Response: map[string]any{
		"authorization_code": "[REDACTED]", "code_verifier": "[REDACTED]", "code_challenge": openai.ChallengeFromVerifier("[REDACTED]"),
	}}}}, []openaitest.TokenFixture{{Code: "[REDACTED]", Response: map[string]any{
		"access_token": "[REDACTED]", "refresh_token": "[REDACTED]", "account_id": "fixture-account", "expires_in": 3600,
	}}})
	c := &nativeFenceCache{e2eSharedMemoryCache: e2eSharedMemoryCache{cache.NewMemoryCache()}}
	h := &handler.Handlers{Config: cfg, DB: testDB, Cache: c, OpenAIOAuthHTTPClient: proxyHandler.OpenAIOAuthHTTPClient}
	return h, c, provider
}

func startNativeDeviceFence(t *testing.T, h *handler.Handlers) openaioauth.DeviceAuthRecord {
	t.Helper()
	started, err := h.StartOpenAIDeviceAuth(context.Background(), "", "fixture-session")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, testDB.DeleteCredential(context.Background(), nativeFenceCredentialID(started.FlowID)))
	})
	store := openaioauth.NewDeviceStore(h.Cache)
	record, err := store.Get(context.Background(), started.FlowID)
	require.NoError(t, err)
	record.NextPollAt = time.Now().UTC().Add(-time.Second)
	require.NoError(t, store.Save(context.Background(), record))
	return record
}

func nativeFenceBundle() handler.OpenAISubscriptionTokenBundle {
	return handler.OpenAISubscriptionTokenBundle{AccessToken: "[REDACTED]", RefreshToken: "[REDACTED]", AccountID: "fixture-account", ExpiresAt: time.Now().UTC().Add(time.Hour)}
}
func nativeFenceCredentialID(flowID string) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("tianji/openai/device/"+flowID)).String()
}
func nativeFenceRowCount(t *testing.T, flowID string) int {
	t.Helper()
	var count int
	require.NoError(t, testPool.QueryRow(context.Background(), `SELECT count(*) FROM "CredentialTable" WHERE credential_id = $1`, nativeFenceCredentialID(flowID)).Scan(&count))
	return count
}
func assertNativeFenceProviderCalls(t *testing.T, provider *openaitest.OAuthServer, polls, exchanges int) {
	t.Helper()
	actualExchanges := 0
	for _, request := range provider.Requests() {
		if request.Path == "/oauth/token" {
			actualExchanges++
		}
	}
	assert.Equal(t, polls, countDevicePollRequests(provider.Requests()))
	assert.Equal(t, exchanges, actualExchanges)
}

// Intercept only native transaction boundaries, not queries or result rows.
type nativeFenceDB struct {
	*pgxpool.Pool
	wrap func(pgx.Tx) pgx.Tx
}

func (d *nativeFenceDB) BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
	if opts.IsoLevel != pgx.ReadCommitted {
		return nil, errors.New("expected ReadCommitted")
	}
	tx, err := d.Pool.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return d.wrap(tx), nil
}

type nativeFenceTx struct {
	pgx.Tx
	inserted   bool
	beforeLock func()
	commit     func(context.Context, pgx.Tx) error
}

func (tx *nativeFenceTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "pg_advisory_xact_lock") && tx.beforeLock != nil {
		tx.beforeLock()
	}
	return tx.Tx.Exec(ctx, sql, args...)
}
func (tx *nativeFenceTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.Contains(sql, `INSERT INTO "CredentialTable"`) {
		tx.inserted = true
	}
	return tx.Tx.QueryRow(ctx, sql, args...)
}
func (tx *nativeFenceTx) Commit(ctx context.Context) error {
	if tx.inserted && tx.commit != nil {
		return tx.commit(ctx, tx.Tx)
	}
	return tx.Tx.Commit(ctx)
}

type nativeFenceCache struct {
	e2eSharedMemoryCache
	beforeCAS func(openaioauth.DeviceAuthRecord, openaioauth.DeviceAuthRecord)
	mu        sync.Mutex
	lease     string
}

func (c *nativeFenceCache) CompareAndSet(ctx context.Context, key string, expected, value []byte, ttl time.Duration) (bool, error) {
	var before, after openaioauth.DeviceAuthRecord
	if json.Unmarshal(expected, &before) != nil || json.Unmarshal(value, &after) != nil {
		return false, errors.New("invalid fixture record")
	}
	if c.beforeCAS != nil {
		c.beforeCAS(before, after)
	}
	return c.MemoryCache.CompareAndSet(ctx, key, expected, value, ttl)
}
func (c *nativeFenceCache) AcquireLock(ctx context.Context, key, token string, ttl time.Duration) (bool, error) {
	ok, err := c.MemoryCache.AcquireLock(ctx, key, token, ttl)
	if ok {
		c.mu.Lock()
		c.lease = token
		c.mu.Unlock()
	}
	return ok, err
}
func (c *nativeFenceCache) expireLease(t *testing.T, flowID string) {
	t.Helper()
	c.mu.Lock()
	token := c.lease
	c.mu.Unlock()
	require.NoError(t, c.ReleaseLock(context.Background(), openaioauth.DeviceAuthLockKey(flowID), token))
}

type nativeFenceTransport func(*http.Request) (*http.Response, error)

func (f nativeFenceTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type nativeFenceResponseBarrier struct {
	io.ReadCloser
	entered, release chan struct{}
}

func (b *nativeFenceResponseBarrier) Close() error {
	err := b.ReadCloser.Close()
	close(b.entered)
	<-b.release
	return err
}
func nativeFenceRelease(ch chan struct{}) func() {
	var once sync.Once
	return func() { once.Do(func() { close(ch) }) }
}
func nativeFenceWait(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("native fence barrier was not reached")
	}
}
func nativeFenceResult(t *testing.T, ch <-chan error) error {
	t.Helper()
	select {
	case err := <-ch:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("native fence worker did not finish")
		return nil
	}
}
