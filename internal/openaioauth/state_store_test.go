package openaioauth

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/praxisllmlab/tianjiLLM/internal/cache"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stateReadBarrierCache struct {
	*cache.DualCache
	afterRead func()
}

func (c *stateReadBarrierCache) GetShared(ctx context.Context, key string) ([]byte, error) {
	body, err := c.DualCache.GetShared(ctx, key)
	if err == nil && c.afterRead != nil {
		c.afterRead()
	}
	return body, err
}

func TestStateStore_ConsumeRejectsStaleReaderAfterLeaseExpiry(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server := miniredis.RunT(t)
	newCache := func() *cache.DualCache {
		client := redis.NewClient(&redis.Options{Addr: server.Addr()})
		t.Cleanup(func() { _ = client.Close() })
		return cache.NewDualCache(cache.NewMemoryCache(), cache.NewRedisCache(client))
	}
	captured, resume, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	release := sync.OnceFunc(func() { close(resume) })
	first := NewStateStore(&stateReadBarrierCache{DualCache: newCache(), afterRead: sync.OnceFunc(func() {
		close(captured)
		select {
		case <-resume:
		case <-ctx.Done():
		}
	})})
	second := NewStateStore(newCache())
	record, err := first.Create(ctx, "org_state", "http://localhost:1455/auth/callback", time.Minute)
	require.NoError(t, err)
	var firstErr error
	var firstReturnedRecord bool
	go func() {
		defer close(done)
		got, err := first.Consume(ctx, record.State)
		firstErr, firstReturnedRecord = err, got.State != ""
	}()
	t.Cleanup(func() { release(); <-done })
	select {
	case <-captured:
	case <-ctx.Done():
		t.Fatal("consume did not reach the shared-read barrier")
	}
	lockKey := CacheKey(record.State) + ":consume"
	require.True(t, server.Exists(lockKey))
	server.FastForward(stateLockTTL + time.Second)
	require.False(t, server.Exists(lockKey))
	require.True(t, server.Exists(CacheKey(record.State)))
	_, secondErr := second.Consume(ctx, record.State)
	release()
	<-done
	successes := 0
	for _, err := range []error{firstErr, secondErr} {
		if err == nil {
			successes++
		}
	}
	_, replayErr := second.Consume(ctx, record.State)
	t.Logf("CONSUME successes=%d stale_returned=%t replay_denied=%t", successes, firstReturnedRecord, errors.Is(replayErr, ErrInvalidState))
	assert.Equal(t, 1, successes)
	assert.NoError(t, secondErr)
	assert.ErrorIs(t, firstErr, ErrInvalidState)
	assert.False(t, firstReturnedRecord)
	assert.ErrorIs(t, replayErr, ErrInvalidState)
}

func TestStateStore_ConsumeRejectsSubMillisecondLifetime(t *testing.T) {
	ctx := context.Background()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := NewStateStore(cache.NewRedisCache(client))
	record, err := store.Create(ctx, "org_state", "http://localhost:1455/auth/callback", time.Minute)
	require.NoError(t, err)
	store.now = func() time.Time { return record.ExpiresAt.Add(-time.Nanosecond) }
	got, err := store.Consume(ctx, record.State)
	assert.ErrorIs(t, err, ErrExpiredState)
	assert.True(t, got == (StateRecord{}), "near-expired state must not be returned")
	assert.False(t, server.Exists(CacheKey(record.State)), "must not leave an immortal Redis marker")
}

type stateCASFailureCache struct {
	*cache.MemoryCache
	commit bool
	err    error
}

func (c *stateCASFailureCache) CompareAndSet(ctx context.Context, key string, expected, value []byte, ttl time.Duration) (bool, error) {
	if c.commit {
		updated, err := c.MemoryCache.CompareAndSet(ctx, key, expected, value, ttl)
		if err != nil || !updated {
			return updated, err
		}
	}
	return c.commit, c.err
}

func TestStateStore_ConsumeFailsClosedWithoutAtomicWinner(t *testing.T) {
	for _, outcome := range []string{"unsupported", "conflict", "write_error", "committed_reply_lost"} {
		t.Run(outcome, func(t *testing.T) {
			ctx := context.Background()
			backend := cache.NewMemoryCache()
			var c cache.Cache = struct{ cache.Cache }{backend}
			if outcome != "unsupported" {
				fault := &stateCASFailureCache{MemoryCache: backend, commit: outcome == "committed_reply_lost"}
				if outcome != "conflict" {
					fault.err = errors.New("backend diagnostic must not escape")
				}
				c = fault
			}
			store := NewStateStore(c)
			record, err := store.Create(ctx, "org_state", "http://localhost:1455/auth/callback", time.Minute)
			require.NoError(t, err)
			before, err := backend.Get(ctx, CacheKey(record.State))
			require.NoError(t, err)
			got, err := store.Consume(ctx, record.State)
			require.Error(t, err)
			assert.True(t, got == (StateRecord{}), "failed consume must not return sensitive fields")
			if outcome == "unsupported" || outcome == "conflict" {
				assert.ErrorIs(t, err, ErrInvalidState)
			} else {
				assert.Equal(t, "consume openai oauth state: cache write failed", err.Error())
			}
			after, err := backend.Get(ctx, CacheKey(record.State))
			require.NoError(t, err)
			if outcome == "committed_reply_lost" {
				assert.True(t, string(after) == `{}`, "ambiguous commit must stay consumed")
				_, err = NewStateStore(backend).Consume(ctx, record.State)
				assert.ErrorIs(t, err, ErrInvalidState)
			} else {
				assert.True(t, string(before) == string(after), "failed write must not delete or restore state")
			}
		})
	}
}

func TestStateStore_ConsumeClearsRecordWithinRemainingTTL(t *testing.T) {
	ctx := context.Background()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	c := cache.NewRedisCache(client)
	store := NewStateStore(c)
	now := time.Now().UTC()
	store.now = func() time.Time { return now }
	record, err := store.Create(ctx, "org_state", "http://localhost:1455/auth/callback", time.Minute)
	require.NoError(t, err)
	now = now.Add(stateLockTTL + time.Second)
	server.FastForward(stateLockTTL + time.Second)
	got, err := store.Consume(ctx, record.State)
	require.NoError(t, err)
	assert.True(t, got == record, "winner must retain exact exchange and organization fields")
	raw, err := c.GetShared(ctx, CacheKey(record.State))
	require.NoError(t, err)
	assert.True(t, string(raw) == `{}`, "terminal marker must not retain sensitive fields")
	remaining := record.ExpiresAt.Sub(now)
	assert.Equal(t, remaining, server.TTL(CacheKey(record.State)))
	server.FastForward(remaining)
	assert.False(t, server.Exists(CacheKey(record.State)))
	_, err = store.Consume(ctx, record.State)
	assert.ErrorIs(t, err, ErrInvalidState)
}

func TestStateStore_ConsumeRejectsCacheExpiryAfterRead(t *testing.T) {
	ctx := context.Background()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	c := &stateReadBarrierCache{
		DualCache: cache.NewDualCache(cache.NewMemoryCache(), cache.NewRedisCache(client)),
		afterRead: func() { server.FastForward(time.Minute) },
	}
	store := NewStateStore(c)
	record, err := store.Create(ctx, "org_state", "http://localhost:1455/auth/callback", time.Minute)
	require.NoError(t, err)
	got, err := store.Consume(ctx, record.State)
	assert.ErrorIs(t, err, ErrInvalidState)
	assert.True(t, got == (StateRecord{}))
	assert.False(t, server.Exists(CacheKey(record.State)), "CAS must not recreate an expired key")
}

func TestStateStore_CreateAndConsumeOnce(t *testing.T) {
	ctx := context.Background()
	store := NewStateStore(cache.NewMemoryCache())

	record, err := store.Create(ctx, "org_123", "http://localhost:1455/auth/callback", time.Minute)
	require.NoError(t, err)
	require.NotEmpty(t, record.State)
	require.NotEmpty(t, record.CodeVerifier)
	assert.Equal(t, "org_123", record.OrgID)
	assert.Equal(t, "http://localhost:1455/auth/callback", record.RedirectURI)
	assert.True(t, record.ExpiresAt.After(record.CreatedAt))

	consumed, err := store.Consume(ctx, record.State)
	require.NoError(t, err)
	assert.Equal(t, record.State, consumed.State)
	assert.Equal(t, record.CodeVerifier, consumed.CodeVerifier)
	assert.Equal(t, "org_123", consumed.OrgID)
	assert.Equal(t, "http://localhost:1455/auth/callback", consumed.RedirectURI)

	_, err = store.Consume(ctx, record.State)
	assert.ErrorIs(t, err, ErrInvalidState)
}

func TestStateStore_ConsumeHonorsPerStateLock(t *testing.T) {
	ctx := context.Background()
	c := cache.NewMemoryCache()
	store := NewStateStore(c)
	record, err := store.Create(ctx, "org_123", "http://localhost:1455/auth/callback", time.Minute)
	require.NoError(t, err)

	lockKey := CacheKey(record.State) + ":consume"
	acquired, err := c.AcquireLock(ctx, lockKey, "[REDACTED]", time.Minute)
	require.NoError(t, err)
	require.True(t, acquired)

	_, err = store.Consume(ctx, record.State)
	assert.ErrorIs(t, err, ErrInvalidState)
}

func TestStateStore_ConsumeReadsSharedLayerAfterLock(t *testing.T) {
	ctx := context.Background()
	c := &sharedDeviceCache{}
	store := NewStateStore(c)
	record, err := store.Create(ctx, "org_123", "http://localhost:1455/auth/callback", time.Minute)
	require.NoError(t, err)
	c.mu.Lock()
	c.shared = nil
	c.mu.Unlock()

	_, err = store.Consume(ctx, record.State)
	require.ErrorIs(t, err, ErrInvalidState)
	assert.Greater(t, c.sharedReads, 0)
}
func TestStateStore_GlobalRecordCanBeConsumedWithoutOrganization(t *testing.T) {
	ctx := context.Background()
	store := NewStateStore(cache.NewMemoryCache())

	record, err := store.Create(ctx, "", "http://localhost:1455/auth/callback", time.Minute)
	require.NoError(t, err)
	assert.Empty(t, record.OrgID)

	consumed, err := store.Consume(ctx, record.State)
	require.NoError(t, err)
	assert.Empty(t, consumed.OrgID)
}

func TestStateStore_CreateStoresRedirectURI(t *testing.T) {
	ctx := context.Background()
	store := NewStateStore(cache.NewMemoryCache())

	record, err := store.Create(ctx, "org_123", "http://127.0.0.1:1455/auth/callback", time.Minute)
	require.NoError(t, err)

	raw, err := store.cache.Get(ctx, CacheKey(record.State))
	require.NoError(t, err)
	var stored StateRecord
	require.NoError(t, json.Unmarshal(raw, &stored))
	assert.Equal(t, "http://127.0.0.1:1455/auth/callback", stored.RedirectURI)
}

func TestStateStore_ExpiredRecordRejectedAndDeleted(t *testing.T) {
	ctx := context.Background()
	c := cache.NewMemoryCache()
	store := NewStateStore(c)
	record := StateRecord{
		State:        "expired-state",
		CodeVerifier: "verifier",
		OrgID:        "org_123",
		RedirectURI:  "http://localhost:1455/auth/callback",
		CreatedAt:    time.Now().Add(-20 * time.Minute),
		ExpiresAt:    time.Now().Add(-10 * time.Minute),
	}
	body, err := json.Marshal(record)
	require.NoError(t, err)
	require.NoError(t, c.Set(ctx, CacheKey(record.State), body, time.Hour))

	_, err = store.Consume(ctx, record.State)
	assert.ErrorIs(t, err, ErrExpiredState)

	raw, err := c.Get(ctx, CacheKey(record.State))
	require.NoError(t, err)
	assert.Nil(t, raw)
}

func TestStateStore_MalformedRecordRejectedAndDeleted(t *testing.T) {
	ctx := context.Background()
	c := cache.NewMemoryCache()
	store := NewStateStore(c)
	require.NoError(t, c.Set(ctx, CacheKey("bad-state"), []byte(`{"state":"other","org_id":""}`), time.Hour))

	_, err := store.Consume(ctx, "bad-state")
	assert.True(t, errors.Is(err, ErrInvalidState))

	raw, err := c.Get(ctx, CacheKey("bad-state"))
	require.NoError(t, err)
	assert.Nil(t, raw)
}
