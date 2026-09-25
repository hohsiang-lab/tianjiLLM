package openaioauth

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/praxisllmlab/tianjiLLM/internal/cache"
)

func TestDeviceStoreGetNeverMutatesExpiredOrInvalidRecords(t *testing.T) {
	ctx := context.Background()
	c := cache.NewMemoryCache()
	store := NewDeviceStore(c)
	record, err := store.Create(ctx, DeviceAuthRecord{DeviceAuthID: "[REDACTED]", UserCode: "[REDACTED]", SessionBinding: "session", TokenPollURL: "http://127.0.0.1/poll", VerificationURI: "http://127.0.0.1/verify", RedirectURI: "http://127.0.0.1/callback"}, time.Minute)
	require.NoError(t, err)
	key := DeviceAuthCacheKey(record.FlowID)
	before, err := c.Get(ctx, key)
	require.NoError(t, err)
	store.now = func() time.Time { return record.ExpiresAt.Add(time.Second) }
	got, err := store.Get(ctx, record.FlowID)
	require.ErrorIs(t, err, ErrDeviceAuthExpired)
	after, err := c.Get(ctx, key)
	require.NoError(t, err)
	assert.True(t, string(before) == string(after), "expiry must not write outside PG arbitration")
	assert.True(t, got == record, "getter must return the exact CAS snapshot")
	require.NoError(t, c.Set(ctx, key, []byte(`{`), time.Minute))
	_, err = store.Get(ctx, record.FlowID)
	require.ErrorIs(t, err, ErrDeviceAuthInvalid)
	after, err = c.Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, []byte(`{`), after, "invalid reads must not delete a newer record")
}

func TestDeviceStoreExpiryCannotOverwriteConcurrentSuccess(t *testing.T) {
	ctx := context.Background()
	c := &expiryReadCache{MemoryCache: cache.NewMemoryCache()}
	store := NewDeviceStore(c)
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	record, err := store.Create(ctx, DeviceAuthRecord{
		DeviceAuthID: "expiry-race-device", UserCode: "expiry-race-code", SessionBinding: "expiry-race-session",
		TokenPollURL: "http://127.0.0.1/poll", VerificationURI: "http://127.0.0.1/verify", RedirectURI: "http://127.0.0.1/callback",
	}, time.Minute)
	require.NoError(t, err)
	success := record
	success.Status = DeviceAuthStatusSuccess
	success.CredentialID = "expiry-race-credential"
	success.DeviceAuthID, success.UserCode, success.TokenPollURL, success.VerificationURI, success.RedirectURI = "", "", "", "", ""
	body, err := json.Marshal(success)
	require.NoError(t, err)
	now = record.ExpiresAt.Add(time.Second)
	c.afterRead = func() {
		require.NoError(t, c.Set(ctx, DeviceAuthCacheKey(record.FlowID), body, time.Minute-time.Second))
	}
	got, err := store.Get(ctx, record.FlowID)
	require.ErrorIs(t, err, ErrDeviceAuthExpired)
	assert.Equal(t, record.Status, got.Status)
	again, err := store.Get(ctx, record.FlowID)
	require.NoError(t, err)
	assert.Equal(t, DeviceAuthStatusSuccess, again.Status)
}

type expiryReadCache struct {
	*cache.MemoryCache
	afterRead func()
}

func (c *expiryReadCache) GetShared(ctx context.Context, key string) ([]byte, error) {
	body, err := c.Get(ctx, key)
	if f := c.afterRead; f != nil {
		c.afterRead = nil
		f()
	}
	return body, err
}

func TestDeviceStoreExpiryGraceIsFixed(t *testing.T) {
	ctx := context.Background()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := NewDeviceStore(cache.NewRedisCache(client))
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	record, err := store.Create(ctx, DeviceAuthRecord{
		DeviceAuthID: "grace-device", UserCode: "grace-code", SessionBinding: "grace-session",
		TokenPollURL: "http://127.0.0.1/poll", VerificationURI: "http://127.0.0.1/verify", RedirectURI: "http://127.0.0.1/callback",
	}, time.Minute)
	require.NoError(t, err)
	server.FastForward(time.Minute + 10*time.Second)
	now = record.ExpiresAt.Add(10 * time.Second)
	_, err = store.Get(ctx, record.FlowID)
	require.ErrorIs(t, err, ErrDeviceAuthExpired)
	assert.Equal(t, 50*time.Second, server.TTL(DeviceAuthCacheKey(record.FlowID)))
	server.FastForward(20 * time.Second)
	now = now.Add(20 * time.Second)
	_, err = store.Get(ctx, record.FlowID)
	require.ErrorIs(t, err, ErrDeviceAuthExpired)
	assert.Equal(t, 30*time.Second, server.TTL(DeviceAuthCacheKey(record.FlowID)))
	// The store clock must enforce the bound even if a backend retains old data.
	now = record.ExpiresAt.Add(deviceAuthExpiryGrace)
	_, err = store.Get(ctx, record.FlowID)
	require.ErrorIs(t, err, ErrDeviceAuthNotFound)
}

func TestDeviceStoreTerminalCASRetainsExpiry(t *testing.T) {
	for _, status := range []DeviceAuthStatus{DeviceAuthStatusExpired, DeviceAuthStatusSuccess} {
		for _, remaining := range []time.Duration{500 * time.Microsecond, time.Millisecond, 2 * time.Millisecond} {
			t.Run(string(status)+"/"+remaining.String(), func(t *testing.T) {
				ctx := context.Background()
				server := miniredis.RunT(t)
				client := redis.NewClient(&redis.Options{Addr: server.Addr()})
				t.Cleanup(func() { _ = client.Close() })
				backend := cache.NewRedisCache(client)
				store := NewDeviceStore(backend)
				now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
				store.now = func() time.Time { return now }
				record, err := store.Create(ctx, DeviceAuthRecord{
					DeviceAuthID: "ttl-device", UserCode: "ttl-code", SessionBinding: "ttl-session",
					TokenPollURL: "http://127.0.0.1/poll", VerificationURI: "http://127.0.0.1/verify", RedirectURI: "http://127.0.0.1/callback",
				}, time.Minute)
				require.NoError(t, err)
				key := DeviceAuthCacheKey(record.FlowID)
				before, err := backend.Get(ctx, key)
				require.NoError(t, err)
				server.FastForward(time.Minute + deviceAuthExpiryGrace - remaining)
				now = record.ExpiresAt.Add(deviceAuthExpiryGrace - remaining)
				require.True(t, server.Exists(key))
				require.Equal(t, remaining, server.TTL(key))
				terminal := record
				terminal.Status = status
				terminal.DeviceAuthID, terminal.UserCode, terminal.TokenPollURL, terminal.VerificationURI, terminal.RedirectURI = "", "", "", "", ""
				var applied bool
				if status == DeviceAuthStatusSuccess {
					terminal.CredentialID = "ttl-credential"
					applied, err = store.SaveIfCurrentForCredentialReconciliation(ctx, record, terminal)
				} else {
					applied, err = store.SaveIfCurrent(ctx, record, terminal)
				}
				after, readErr := backend.Get(ctx, key)
				require.NoError(t, readErr)
				if remaining < time.Millisecond {
					assert.ErrorIs(t, err, ErrDeviceAuthExpired)
					assert.False(t, applied)
					assert.True(t, string(before) == string(after), "rejected CAS must preserve exact bytes")
				} else {
					require.NoError(t, err)
					assert.True(t, applied)
					var saved DeviceAuthRecord
					require.NoError(t, json.Unmarshal(after, &saved))
					assert.Equal(t, status, saved.Status)
				}
				assert.Equal(t, remaining, server.TTL(key), "CAS must retain the fixed Redis expiry")
				server.FastForward(remaining)
				assert.False(t, server.Exists(key), "record must physically expire")
			})
		}
	}
}

func TestDeviceStoreExpiredSavesRetainReconciledSuccess(t *testing.T) {
	ctx := context.Background()
	store := NewDeviceStore(cache.NewMemoryCache())
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	record, err := store.Create(ctx, DeviceAuthRecord{
		DeviceAuthID: "save-expiry-device", UserCode: "save-expiry-code", SessionBinding: "save-expiry-session",
		TokenPollURL: "http://127.0.0.1/poll", VerificationURI: "http://127.0.0.1/verify", RedirectURI: "http://127.0.0.1/callback",
	}, time.Minute)
	require.NoError(t, err)
	success := record
	success.Status = DeviceAuthStatusSuccess
	success.CredentialID = "save-expiry-credential"
	now = record.ExpiresAt.Add(time.Second)
	applied, err := store.SaveIfCurrentForCredentialReconciliation(ctx, record, success)
	require.NoError(t, err)
	require.True(t, applied)
	require.ErrorIs(t, store.Save(ctx, record), ErrDeviceAuthExpired)
	applied, err = store.SaveIfCurrent(ctx, record, record)
	require.ErrorIs(t, err, ErrDeviceAuthExpired)
	assert.False(t, applied)
	current, err := store.Get(ctx, record.FlowID)
	require.NoError(t, err)
	assert.Equal(t, DeviceAuthStatusSuccess, current.Status)
	now = record.ExpiresAt.Add(deviceAuthExpiryGrace)
	applied, err = store.SaveIfCurrentForCredentialReconciliation(ctx, current, success)
	require.ErrorIs(t, err, ErrDeviceAuthExpired)
	assert.False(t, applied)
	_, err = store.Get(ctx, record.FlowID)
	require.ErrorIs(t, err, ErrDeviceAuthNotFound)
}

func TestDeviceStoreReconciliationCannotChangeFlowIdentity(t *testing.T) {
	for _, field := range []string{"organization", "session", "expiry"} {
		t.Run(field, func(t *testing.T) {
			ctx := context.Background()
			store := NewDeviceStore(cache.NewMemoryCache())
			record, err := store.Create(ctx, DeviceAuthRecord{
				DeviceAuthID: "identity-device", UserCode: "identity-code", SessionBinding: "identity-session", OrgID: "identity-org",
				TokenPollURL: "http://127.0.0.1/poll", VerificationURI: "http://127.0.0.1/verify", RedirectURI: "http://127.0.0.1/callback",
			}, time.Minute)
			require.NoError(t, err)
			success := record
			success.Status = DeviceAuthStatusSuccess
			success.CredentialID = "identity-credential"
			switch field {
			case "organization":
				success.OrgID = "other-org"
			case "session":
				success.SessionBinding = "other-session"
			case "expiry":
				success.ExpiresAt = success.ExpiresAt.Add(time.Minute)
			}
			applied, err := store.SaveIfCurrentForCredentialReconciliation(ctx, record, success)
			require.Error(t, err)
			assert.False(t, applied)
			current, err := store.Get(ctx, record.FlowID)
			require.NoError(t, err)
			assert.Equal(t, DeviceAuthStatusPending, current.Status)
		})
	}
}

func TestDeviceStore_CreateAndGetUsesOpaqueFlowAndTTL(t *testing.T) {
	store := NewDeviceStore(cache.NewMemoryCache())
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }

	record, err := store.Create(context.Background(), DeviceAuthRecord{
		DeviceAuthID:    "device-id",
		UserCode:        "ABCD-EFGH",
		TokenPollURL:    "https://auth.example.test/api/accounts/deviceauth/token",
		VerificationURI: "https://auth.example.test/codex/device",
		RedirectURI:     "https://auth.example.test/deviceauth/callback",
		OrgID:           "org-123",
		SessionBinding:  "session-hash",
		IntervalSeconds: 5,
	}, 15*time.Minute)

	require.NoError(t, err)
	assert.NotEmpty(t, record.FlowID)
	assert.NotEqual(t, record.FlowID, record.DeviceAuthID)
	assert.Equal(t, DeviceAuthStatusPending, record.Status)
	assert.Equal(t, now, record.CreatedAt)
	assert.Equal(t, now.Add(15*time.Minute), record.ExpiresAt)
	assert.Equal(t, now.Add(5*time.Second), record.NextPollAt)

	got, err := store.Get(context.Background(), record.FlowID)
	require.NoError(t, err)
	assert.Equal(t, record, got)
}

type failingDeviceCache struct{}

func (failingDeviceCache) Get(context.Context, string) ([]byte, error) {
	return nil, errDeviceCacheSentinel
}
func (failingDeviceCache) Set(context.Context, string, []byte, time.Duration) error { return nil }
func (failingDeviceCache) Delete(context.Context, string) error                     { return nil }
func (failingDeviceCache) MGet(context.Context, ...string) ([][]byte, error)        { return nil, nil }

var errDeviceCacheSentinel = errors.New("cache sentinel")

func TestDeviceStoreGetPreservesCacheError(t *testing.T) {
	_, err := NewDeviceStore(failingDeviceCache{}).Get(context.Background(), "flow-id")
	assert.ErrorIs(t, err, errDeviceCacheSentinel)
}

func TestDeviceStore_MemoryCacheIsNotCoordinated(t *testing.T) {
	assert.False(t, NewDeviceStore(cache.NewMemoryCache()).Coordinated())
}

func TestDeviceStore_RejectsRecordWithoutServerEndpoints(t *testing.T) {
	_, err := NewDeviceStore(cache.NewMemoryCache()).Create(context.Background(), DeviceAuthRecord{
		DeviceAuthID:   "device-id",
		UserCode:       "ABCD-EFGH",
		SessionBinding: "session-hash",
	}, time.Minute)

	require.ErrorIs(t, err, ErrDeviceAuthInvalid)
}

func TestDeviceStore_GetUsesSharedLayer(t *testing.T) {
	now := time.Now().UTC()
	sharedRecord := DeviceAuthRecord{
		FlowID:          "flow-1",
		DeviceAuthID:    "device-id",
		UserCode:        "ABCD-EFGH",
		TokenPollURL:    "https://auth.example.test/api/accounts/deviceauth/token",
		VerificationURI: "https://auth.example.test/codex/device",
		RedirectURI:     "https://auth.example.test/deviceauth/callback",
		SessionBinding:  "session-hash",
		IntervalSeconds: 5,
		Status:          DeviceAuthStatusPending,
		CreatedAt:       now,
		UpdatedAt:       now,
		ExpiresAt:       now.Add(time.Minute),
		NextPollAt:      now,
	}
	sharedBody, err := json.Marshal(sharedRecord)
	require.NoError(t, err)
	c := &sharedDeviceCache{local: []byte(`{"status":"stale"}`), shared: sharedBody}

	got, err := NewDeviceStore(c).Get(context.Background(), sharedRecord.FlowID)

	require.NoError(t, err)
	assert.Equal(t, sharedRecord, got)
	assert.Equal(t, 1, c.sharedReads)
}

func TestDeviceStore_OwnershipAndTerminalStateHelpers(t *testing.T) {
	record := DeviceAuthRecord{SessionBinding: "session-hash", Status: DeviceAuthStatusSuccess}

	assert.True(t, record.OwnedBy("session-hash"))
	assert.False(t, record.OwnedBy("other-session"))
	assert.True(t, record.Terminal())
	assert.False(t, (DeviceAuthRecord{Status: DeviceAuthStatusPending}).Terminal())
}

func TestDeviceStore_RejectsUnknownStatus(t *testing.T) {
	ctx := context.Background()
	cacheStore := cache.NewMemoryCache()
	store := NewDeviceStore(cacheStore)
	record, err := store.Create(ctx, DeviceAuthRecord{
		DeviceAuthID:    "[REDACTED]",
		UserCode:        "ABCD-EFGH",
		TokenPollURL:    "https://auth.example.test/api/accounts/deviceauth/token",
		VerificationURI: "https://auth.example.test/codex/device",
		RedirectURI:     "https://auth.example.test/deviceauth/callback",
		SessionBinding:  "session-hash",
	}, time.Minute)
	require.NoError(t, err)

	record.Status = DeviceAuthStatus("future")
	body, err := json.Marshal(record)
	require.NoError(t, err)
	require.NoError(t, cacheStore.Set(ctx, DeviceAuthCacheKey(record.FlowID), body, time.Minute))

	_, err = store.Get(ctx, record.FlowID)

	require.ErrorIs(t, err, ErrDeviceAuthInvalid)
}

func TestDeviceStore_WithLockSerializesConcurrentPollers(t *testing.T) {
	store := NewDeviceStore(cache.NewMemoryCache())
	record, err := store.Create(context.Background(), DeviceAuthRecord{
		DeviceAuthID:    "device-id",
		UserCode:        "ABCD-EFGH",
		TokenPollURL:    "https://auth.example.test/api/accounts/deviceauth/token",
		VerificationURI: "https://auth.example.test/codex/device",
		RedirectURI:     "https://auth.example.test/deviceauth/callback",
		SessionBinding:  "session-hash",
	}, time.Minute)
	require.NoError(t, err)

	entered := make(chan struct{})
	release := make(chan struct{})
	firstDone := make(chan struct{})
	secondStarted := make(chan struct{})
	secondDone := make(chan struct{})
	var firstErr error
	var secondErr error
	var secondAcquired bool
	go func() {
		defer close(firstDone)
		_, firstErr = store.WithLock(context.Background(), record.FlowID, func() error {
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered

	go func() {
		defer close(secondDone)
		close(secondStarted)
		secondAcquired, secondErr = store.WithLock(context.Background(), record.FlowID, func() error { return nil })
	}()
	<-secondStarted
	<-secondDone
	close(release)
	<-firstDone

	require.NoError(t, firstErr)
	require.NoError(t, secondErr)
	assert.False(t, secondAcquired)
}

func TestDeviceStore_LockLeaseCoversTwoBoundedProviderRequests(t *testing.T) {
	assert.GreaterOrEqual(t, deviceAuthLockTTL, 2*time.Minute)
}

func TestDeviceStore_RecordDoesNotContainTokenMaterial(t *testing.T) {
	record := DeviceAuthRecord{
		FlowID:         "flow-1",
		DeviceAuthID:   "device-id",
		UserCode:       "ABCD-EFGH",
		SessionBinding: "session-hash",
		Status:         DeviceAuthStatusSuccess,
		CredentialID:   "credential-id",
	}
	body, err := json.Marshal(record)
	require.NoError(t, err)
	assert.NotContains(t, string(body), "access_token")
	assert.NotContains(t, string(body), "refresh_token")
	assert.NotContains(t, string(body), "id_token")
	assert.NotContains(t, string(body), "credential_value")
}

func TestDeviceStore_ExpiredRecordIsRejected(t *testing.T) {
	store := NewDeviceStore(cache.NewMemoryCache())
	store.now = func() time.Time { return time.Unix(100, 0).UTC() }
	record, err := store.Create(context.Background(), DeviceAuthRecord{
		DeviceAuthID:    "device-id",
		UserCode:        "ABCD-EFGH",
		TokenPollURL:    "https://auth.example.test/api/accounts/deviceauth/token",
		VerificationURI: "https://auth.example.test/codex/device",
		RedirectURI:     "https://auth.example.test/deviceauth/callback",
		SessionBinding:  "session-hash",
	}, time.Second)
	require.NoError(t, err)

	store.now = func() time.Time { return record.ExpiresAt.Add(time.Nanosecond) }
	got, err := store.Get(context.Background(), record.FlowID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrDeviceAuthExpired))
	assert.Equal(t, record.FlowID, got.FlowID)
	assert.True(t, got == record, "read-only expiry must retain the exact CAS snapshot")

	again, err := store.Get(context.Background(), record.FlowID)
	require.ErrorIs(t, err, ErrDeviceAuthExpired)
	assert.Equal(t, got, again)
}

func TestDeviceStore_SaveIfCurrentRejectsExpiredExchangeLease(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	store := NewDeviceStore(cache.NewMemoryCache())
	store.now = func() time.Time { return now }
	record, err := store.Create(ctx, DeviceAuthRecord{
		DeviceAuthID:    "device-id",
		UserCode:        "ABCD-EFGH",
		TokenPollURL:    "https://auth.example.test/api/accounts/deviceauth/token",
		VerificationURI: "https://auth.example.test/codex/device",
		RedirectURI:     "https://auth.example.test/deviceauth/callback",
		SessionBinding:  "session-hash",
	}, time.Hour)
	require.NoError(t, err)

	expected := record
	expected.Status = DeviceAuthStatusExchanging
	expected.UpdatedAt = now.Add(-deviceAuthLockTTL - time.Second)
	expectedBody, err := json.Marshal(expected)
	require.NoError(t, err)
	require.NoError(t, store.cache.Set(ctx, DeviceAuthCacheKey(record.FlowID), expectedBody, time.Hour))

	staleSuccess := expected
	staleSuccess.Status = DeviceAuthStatusSuccess
	staleSuccess.CredentialID = "credential-id"
	applied, err := store.SaveIfCurrent(ctx, expected, staleSuccess)

	require.NoError(t, err)
	assert.False(t, applied)
	current, err := store.Get(ctx, record.FlowID)
	require.NoError(t, err)
	assert.Equal(t, DeviceAuthStatusExchanging, current.Status)
}

func TestDeviceStore_RedisInstancesShareStateAndLock(t *testing.T) {
	server := miniredis.RunT(t)
	client1 := redis.NewClient(&redis.Options{Addr: server.Addr()})
	client2 := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		_ = client1.Close()
		_ = client2.Close()
	})

	store1 := NewDeviceStore(cache.NewRedisCache(client1))
	store2 := NewDeviceStore(cache.NewRedisCache(client2))
	ctx := context.Background()
	record, err := store1.Create(ctx, DeviceAuthRecord{
		DeviceAuthID:    "device-id",
		UserCode:        "ABCD-EFGH",
		TokenPollURL:    "https://auth.example.test/api/accounts/deviceauth/token",
		VerificationURI: "https://auth.example.test/codex/device",
		RedirectURI:     "https://auth.example.test/deviceauth/callback",
		SessionBinding:  "session-hash",
	}, time.Minute)
	require.NoError(t, err)
	require.True(t, store1.Coordinated())
	require.True(t, store2.Coordinated())

	got, err := store2.Get(ctx, record.FlowID)
	require.NoError(t, err)
	assert.Equal(t, record, got)

	entered := make(chan struct{})
	release := make(chan struct{})
	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		_, _ = store1.WithLock(ctx, record.FlowID, func() error {
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered

	acquired, err := store2.WithLock(ctx, record.FlowID, func() error { return nil })
	require.NoError(t, err)
	assert.False(t, acquired)
	close(release)
	<-firstDone
}

type sharedDeviceCache struct {
	mu          sync.Mutex
	local       []byte
	shared      []byte
	sharedReads int
}

func (c *sharedDeviceCache) Get(context.Context, string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]byte(nil), c.local...), nil
}

func (c *sharedDeviceCache) Set(_ context.Context, _ string, value []byte, _ time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.local = append([]byte(nil), value...)
	c.shared = append([]byte(nil), value...)
	return nil
}

func (c *sharedDeviceCache) Delete(context.Context, string) error              { return nil }
func (c *sharedDeviceCache) MGet(context.Context, ...string) ([][]byte, error) { return nil, nil }

func (c *sharedDeviceCache) GetShared(context.Context, string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sharedReads++
	return append([]byte(nil), c.shared...), nil
}

func (c *sharedDeviceCache) CompareAndSet(context.Context, string, []byte, []byte, time.Duration) (bool, error) {
	return false, nil
}

func (c *sharedDeviceCache) AcquireLock(context.Context, string, string, time.Duration) (bool, error) {
	return true, nil
}

func (c *sharedDeviceCache) ReleaseLock(context.Context, string, string) error { return nil }

var _ cache.Cache = (*sharedDeviceCache)(nil)
var _ cache.SharedCache = (*sharedDeviceCache)(nil)
var _ cache.LockCache = (*sharedDeviceCache)(nil)
