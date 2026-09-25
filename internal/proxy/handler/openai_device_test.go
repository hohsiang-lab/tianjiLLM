package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/iotest"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/praxisllmlab/tianjiLLM/internal/cache"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/openaioauth"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

func TestStartOpenAIDeviceAuthDisabledMakesNoProviderRequests(t *testing.T) {
	server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{})
	h := newDeviceAuthHandlers(t, server)
	h.Config.GeneralSettings.OpenAIOAuth.Enabled = false
	calls := 0
	h.OpenAIOAuthHTTPClient = &http.Client{Transport: requestContextTransport(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("unexpected provider request")
	})}
	_, err := h.StartOpenAIDeviceAuth(context.Background(), "org-disabled", "session-disabled")
	assert.Zero(t, calls)
	require.EqualError(t, err, "OpenAI Connect is disabled")
}

func TestPollOpenAIDeviceAuthDisabledPreservesStatusAndCancel(t *testing.T) {
	server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{})
	h := newDeviceAuthHandlers(t, server)
	started, err := h.StartOpenAIDeviceAuth(context.Background(), "org-disabled", "session-disabled")
	require.NoError(t, err)
	makeDeviceFlowEligible(t, h.Cache, started.FlowID)
	h.Config.GeneralSettings.OpenAIOAuth.Enabled = false
	status, err := h.PollOpenAIDeviceAuth(context.Background(), started.FlowID, "session-disabled")
	require.NoError(t, err)
	assert.Equal(t, openaioauth.DeviceAuthStatusPending, status.Status)
	assert.Zero(t, server.pollCalls())
	assert.Zero(t, server.tokenCalls())
	require.NoError(t, h.CancelOpenAIDeviceAuth(context.Background(), started.FlowID, "session-disabled"))
	status, err = h.PollOpenAIDeviceAuth(context.Background(), started.FlowID, "session-disabled")
	require.NoError(t, err)
	assert.Equal(t, openaioauth.DeviceAuthStatusCancelled, status.Status)
}

func TestPollOpenAIDeviceAuthUnreadableSuccessFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name     string
		err      error
		wantCode string
	}{
		{"cancelled", context.Canceled, "exchange_failed"},
		{"deadline", context.DeadlineExceeded, "exchange_failed"},
		{"unexpected_eof", io.ErrUnexpectedEOF, "provider_error"},
		{"wrapped_cancelled", fmt.Errorf("private reader diagnostic: %w", context.Canceled), "exchange_failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{pollSuccess: true})
			h := newDeviceAuthHandlers(t, server)
			started, err := h.StartOpenAIDeviceAuth(context.Background(), "org-unreadable", "session-unreadable")
			require.NoError(t, err)
			makeDeviceFlowEligible(t, h.Cache, started.FlowID)
			saves := 0
			dbStore, ok := h.DB.(*mockStore)
			require.True(t, ok)
			dbStore.createCredentialFn = func(context.Context, db.CreateCredentialParams) (db.CredentialTable, error) {
				saves++
				return db.CredentialTable{}, errors.New("unexpected insert")
			}
			transport := server.Client().Transport
			h.OpenAIOAuthHTTPClient = &http.Client{Transport: requestContextTransport(func(req *http.Request) (*http.Response, error) {
				resp, roundTripErr := transport.RoundTrip(req)
				if roundTripErr == nil && req.URL.Path == "/api/accounts/deviceauth/token" {
					require.Equal(t, http.StatusOK, resp.StatusCode)
					_ = resp.Body.Close()
					resp.Body = io.NopCloser(iotest.ErrReader(tc.err))
				}
				return resp, roundTripErr
			})}
			for i := 0; i < 2; i++ {
				status, pollErr := h.PollOpenAIDeviceAuth(context.Background(), started.FlowID, "session-unreadable")
				require.NoError(t, pollErr)
				assert.Equal(t, openaioauth.DeviceAuthStatusFailed, status.Status)
				assert.Equal(t, tc.wantCode, status.ErrorCode)
			}
			record, err := openaioauth.NewDeviceStore(h.Cache).Get(context.Background(), started.FlowID)
			require.NoError(t, err)
			assert.True(t, record.DeviceAuthID == "" && record.UserCode == "" && record.TokenPollURL == "" && record.RedirectURI == "" && record.VerificationURI == "", "provider state must be cleared")
			assert.Equal(t, 1, server.pollCalls())
			assert.Zero(t, server.tokenCalls())
			assert.Zero(t, saves)
		})
	}
}

func TestPollOpenAIDeviceAuthLostPollResponseDoesNotReplay(t *testing.T) {
	server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{pollSuccess: true})
	h := newDeviceAuthHandlers(t, server)
	started, err := h.StartOpenAIDeviceAuth(context.Background(), "org-lost-response", "session-lost-response")
	require.NoError(t, err)
	saves := 0
	dbStore, ok := h.DB.(*mockStore)
	require.True(t, ok)
	dbStore.createCredentialFn = func(context.Context, db.CreateCredentialParams) (db.CredentialTable, error) {
		saves++
		return db.CredentialTable{}, errors.New("unexpected insert")
	}
	transport := server.Client().Transport
	h.OpenAIOAuthHTTPClient = &http.Client{Transport: requestContextTransport(func(req *http.Request) (*http.Response, error) {
		resp, roundTripErr := transport.RoundTrip(req)
		if roundTripErr == nil && req.URL.Path == "/api/accounts/deviceauth/token" {
			require.Equal(t, http.StatusOK, resp.StatusCode)
			_ = resp.Body.Close()
			return nil, fmt.Errorf("private transport diagnostic: %w", io.ErrUnexpectedEOF)
		}
		return resp, roundTripErr
	})}

	for i := 0; i < 2; i++ {
		makeDeviceFlowEligible(t, h.Cache, started.FlowID)
		status, pollErr := h.PollOpenAIDeviceAuth(context.Background(), started.FlowID, "session-lost-response")
		require.NoError(t, pollErr)
		assert.Equal(t, openaioauth.DeviceAuthStatusFailed, status.Status)
		assert.Equal(t, "provider_error", status.ErrorCode)
	}
	record, err := openaioauth.NewDeviceStore(h.Cache).Get(context.Background(), started.FlowID)
	require.NoError(t, err)
	assert.Equal(t, openaioauth.DeviceAuthStatusFailed, record.Status)
	assert.Equal(t, "provider_error", record.ErrorCode)
	assert.True(t, record.DeviceAuthID == "" && record.UserCode == "" && record.TokenPollURL == "" && record.RedirectURI == "" && record.VerificationURI == "", "provider state must be cleared")
	assert.Equal(t, 1, server.pollCalls())
	assert.Zero(t, server.tokenCalls())
	assert.Zero(t, saves)
}

func TestPollOpenAIDeviceAuthReconcilesCredentialDuringExpiryGrace(t *testing.T) {
	server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{})
	h := newDeviceAuthHandlers(t, server)
	started, err := h.StartOpenAIDeviceAuth(context.Background(), "org-grace", "session-grace")
	require.NoError(t, err)
	store := openaioauth.NewDeviceStore(h.Cache)
	record, err := store.Get(context.Background(), started.FlowID)
	require.NoError(t, err)
	record.CreatedAt = time.Now().UTC().Add(-2 * time.Minute)
	record.ExpiresAt = time.Now().UTC().Add(-time.Second)
	record.Status = openaioauth.DeviceAuthStatusExchanging
	record.Generation = "admitted:grace-fixture"
	body, err := json.Marshal(record)
	require.NoError(t, err)
	require.NoError(t, h.Cache.Set(context.Background(), openaioauth.DeviceAuthCacheKey(record.FlowID), body, time.Minute))
	credentialID := openAIDeviceCredentialID(record.FlowID)
	dbStore, ok := h.DB.(*mockStore)
	require.True(t, ok)
	dbStore.getCredentialFn = func(_ context.Context, id string) (db.CredentialTable, error) {
		require.True(t, id == credentialID, "must look up deterministic credential identity")
		return db.CredentialTable{CredentialID: credentialID, CredentialType: CredentialTypeOpenAISubscription, OrganizationID: &record.OrgID}, nil
	}
	inserts := 0
	dbStore.createCredentialFn = func(context.Context, db.CreateCredentialParams) (db.CredentialTable, error) {
		inserts++
		return db.CredentialTable{}, errors.New("unexpected insert")
	}
	h.Config.GeneralSettings.OpenAIOAuth.Enabled = false
	for i := 0; i < 2; i++ {
		status, pollErr := h.PollOpenAIDeviceAuth(context.Background(), record.FlowID, record.SessionBinding)
		require.NoError(t, pollErr)
		assert.Equal(t, openaioauth.DeviceAuthStatusSuccess, status.Status)
		assert.Equal(t, credentialID, status.CredentialID)
	}
	current, err := store.Get(context.Background(), record.FlowID)
	require.NoError(t, err)
	assert.Equal(t, openaioauth.DeviceAuthStatusSuccess, current.Status)
	assert.True(t, current.DeviceAuthID == "" && current.UserCode == "" && current.TokenPollURL == "", "provider state must be cleared")
	assert.Zero(t, inserts)
	assert.Zero(t, server.pollCalls())
	assert.Zero(t, server.tokenCalls())
}

func TestPollOpenAIDeviceAuthExpiryReconciliationRequiresIdentity(t *testing.T) {
	for _, name := range []string{"wrong_session", "wrong_org", "global_credential", "missing", "wrong_type", "wrong_id"} {
		t.Run(name, func(t *testing.T) {
			server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{})
			h := newDeviceAuthHandlers(t, server)
			started, err := h.StartOpenAIDeviceAuth(context.Background(), "org-identity", "session-identity")
			require.NoError(t, err)
			record := expireDeviceFlow(t, h.Cache, started.FlowID)
			credential := db.CredentialTable{CredentialID: openAIDeviceCredentialID(record.FlowID), CredentialType: CredentialTypeOpenAISubscription, OrganizationID: &record.OrgID}
			session := record.SessionBinding
			switch name {
			case "wrong_session":
				session = "other-session-identity"
			case "wrong_org":
				org := "other-org-identity"
				credential.OrganizationID = &org
			case "global_credential":
				credential.OrganizationID = nil
			case "wrong_type":
				credential.CredentialType = "api_key"
			case "wrong_id":
				credential.CredentialID = "unrelated-credential"
			}
			lookups := 0
			dbStore, ok := h.DB.(*mockStore)
			require.True(t, ok)
			dbStore.getCredentialFn = func(context.Context, string) (db.CredentialTable, error) {
				lookups++
				if name == "missing" {
					return db.CredentialTable{}, pgx.ErrNoRows
				}
				return credential, nil
			}
			status, err := h.PollOpenAIDeviceAuth(context.Background(), record.FlowID, session)
			if name != "missing" {
				require.ErrorIs(t, err, openaioauth.ErrDeviceAuthOwnership)
			} else {
				require.NoError(t, err)
				assert.Equal(t, openaioauth.DeviceAuthStatusExpired, status.Status)
				assert.Empty(t, status.CredentialID)
			}
			if name == "wrong_session" {
				assert.Zero(t, lookups)
			}
			assert.Zero(t, server.pollCalls())
			assert.Zero(t, server.tokenCalls())
		})
	}
}

func deviceFlowExpiryBarrier(t *testing.T, c cache.Cache, flowID string) func() {
	t.Helper()
	store := openaioauth.NewDeviceStore(c)
	record, err := store.Get(context.Background(), flowID)
	require.NoError(t, err)
	record.ExpiresAt = time.Now().UTC().Add(100 * time.Millisecond)
	body, err := json.Marshal(record)
	require.NoError(t, err)
	require.NoError(t, c.Set(context.Background(), openaioauth.DeviceAuthCacheKey(flowID), body, time.Minute+100*time.Millisecond))
	return func() { time.Sleep(max(0, time.Until(record.ExpiresAt)) + time.Millisecond) }
}

func expireDeviceFlow(t *testing.T, c cache.Cache, flowID string) openaioauth.DeviceAuthRecord {
	t.Helper()
	record, err := openaioauth.NewDeviceStore(c).Get(context.Background(), flowID)
	require.NoError(t, err)
	now := time.Now().UTC()
	record.CreatedAt = now.Add(-2 * time.Minute)
	record.ExpiresAt = now.Add(-time.Second)
	body, err := json.Marshal(record)
	require.NoError(t, err)
	require.NoError(t, c.Set(context.Background(), openaioauth.DeviceAuthCacheKey(flowID), body, time.Minute-time.Second))
	return record
}

func TestPollOpenAIDeviceAuthExpiryAtReReadReconciles(t *testing.T) {
	for _, path := range []string{"poll_locked", "poll_busy", "stale_locked", "stale_busy"} {
		t.Run(path, func(t *testing.T) {
			ctx := context.Background()
			server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{})
			h := newDeviceAuthHandlers(t, server)
			c := &deviceAuthReadHookCache{deviceTestSharedMemoryCache: deviceTestSharedMemoryCache{cache.NewMemoryCache()}}
			h.Cache = c
			started, err := h.StartOpenAIDeviceAuth(ctx, "org-reread", "session-reread")
			require.NoError(t, err)
			makeDeviceFlowEligible(t, c, started.FlowID)
			waitForExpiry := deviceFlowExpiryBarrier(t, c, started.FlowID)
			store := openaioauth.NewDeviceStore(c)
			record, err := store.Get(ctx, started.FlowID)
			require.NoError(t, err)
			record.Generation = "admitted:reread-fixture"
			seeded, marshalErr := json.Marshal(record)
			require.NoError(t, marshalErr)
			require.NoError(t, c.Set(ctx, openaioauth.DeviceAuthCacheKey(record.FlowID), seeded, time.Minute))
			if strings.HasPrefix(path, "stale") {
				record.Status = openaioauth.DeviceAuthStatusExchanging
				record.UpdatedAt = time.Now().UTC().Add(-deviceAuthExchangeLease - time.Second)
				body, marshalErr := json.Marshal(record)
				require.NoError(t, marshalErr)
				require.NoError(t, c.Set(ctx, openaioauth.DeviceAuthCacheKey(record.FlowID), body, time.Minute))
			}
			if strings.HasSuffix(path, "busy") {
				acquired, lockErr := c.AcquireLock(ctx, openaioauth.DeviceAuthLockKey(record.FlowID), "reread-lock", time.Minute)
				require.NoError(t, lockErr)
				require.True(t, acquired)
			}
			durable := false
			dbStore, ok := h.DB.(*mockStore)
			require.True(t, ok)
			dbStore.getCredentialFn = func(context.Context, string) (db.CredentialTable, error) {
				if !durable {
					return db.CredentialTable{}, pgx.ErrNoRows
				}
				return db.CredentialTable{CredentialID: openAIDeviceCredentialID(record.FlowID), CredentialType: CredentialTypeOpenAISubscription, OrganizationID: &record.OrgID}, nil
			}
			reads := 0
			c.beforeRead = func() {
				reads++
				if reads == 2 {
					c.beforeRead = nil
					waitForExpiry()
					durable = true
				}
			}
			status, err := h.PollOpenAIDeviceAuth(ctx, record.FlowID, record.SessionBinding)
			require.NoError(t, err)
			assert.Equal(t, openaioauth.DeviceAuthStatusSuccess, status.Status)
			assert.Zero(t, server.pollCalls())
			assert.Zero(t, server.tokenCalls())
		})
	}
}

type deviceAuthReadHookCache struct {
	deviceTestSharedMemoryCache
	beforeRead func()
}

func (c *deviceAuthReadHookCache) GetShared(ctx context.Context, key string) ([]byte, error) {
	if c.beforeRead != nil {
		c.beforeRead()
	}
	return c.Get(ctx, key)
}

func TestPollOpenAIDeviceAuthExpiryAfterPollStopsExchange(t *testing.T) {
	server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{pollSuccess: true})
	h := newDeviceAuthHandlers(t, server)
	started, err := h.StartOpenAIDeviceAuth(context.Background(), "org-expiry-poll", "session-expiry-poll")
	require.NoError(t, err)
	waitForExpiry := deviceFlowExpiryBarrier(t, h.Cache, started.FlowID)
	makeDeviceFlowEligible(t, h.Cache, started.FlowID)
	transport := server.Client().Transport
	h.OpenAIOAuthHTTPClient = &http.Client{Transport: requestContextTransport(func(req *http.Request) (*http.Response, error) {
		resp, roundTripErr := transport.RoundTrip(req)
		if roundTripErr == nil && req.URL.Path == "/api/accounts/deviceauth/token" {
			waitForExpiry()
		}
		return resp, roundTripErr
	})}
	status, err := h.PollOpenAIDeviceAuth(context.Background(), started.FlowID, "session-expiry-poll")
	require.NoError(t, err)
	assert.Equal(t, openaioauth.DeviceAuthStatusExpired, status.Status)
	assert.Zero(t, server.tokenCalls())
	assert.Equal(t, 1, server.pollCalls())
}

func TestPollOpenAIDeviceAuthExpiryDuringInsertReconciles(t *testing.T) {
	for _, cacheFailure := range []bool{false, true} {
		t.Run(fmt.Sprint(cacheFailure), func(t *testing.T) {
			server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{pollSuccess: true})
			h := newDeviceAuthHandlers(t, server)
			h.Cache = &failingDeviceAuthCache{MemoryCache: cache.NewMemoryCache(), failTerminalSave: cacheFailure}
			started, err := h.StartOpenAIDeviceAuth(context.Background(), "org-insert-expiry", "session-insert-expiry")
			require.NoError(t, err)
			waitForExpiry := deviceFlowExpiryBarrier(t, h.Cache, started.FlowID)
			makeDeviceFlowEligible(t, h.Cache, started.FlowID)
			var durable db.CredentialTable
			inserts := 0
			dbStore, ok := h.DB.(*mockStore)
			require.True(t, ok)
			dbStore.createCredentialFn = func(_ context.Context, arg db.CreateCredentialParams) (db.CredentialTable, error) {
				inserts++
				// Deterministic insert barrier: the flow expires while the DB write completes.
				waitForExpiry()
				durable = db.CredentialTable{CredentialID: arg.CredentialID, CredentialType: arg.CredentialType, OrganizationID: arg.OrganizationID}
				return durable, nil
			}
			dbStore.getCredentialFn = func(_ context.Context, id string) (db.CredentialTable, error) {
				if durable.CredentialID == "" {
					return db.CredentialTable{}, pgx.ErrNoRows
				}
				require.True(t, id == durable.CredentialID, "reconciliation must use durable identity")
				return durable, nil
			}
			status, err := h.PollOpenAIDeviceAuth(context.Background(), started.FlowID, "session-insert-expiry")
			if cacheFailure {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, openaioauth.DeviceAuthStatusSuccess, status.Status)
			}
			for i := 0; i < 2; i++ {
				status, err = h.PollOpenAIDeviceAuth(context.Background(), started.FlowID, "session-insert-expiry")
				require.NoError(t, err)
				assert.Equal(t, openaioauth.DeviceAuthStatusSuccess, status.Status)
				assert.True(t, status.CredentialID == durable.CredentialID, "credential identity must be preserved")
			}
			assert.Equal(t, 1, inserts)
			assert.Equal(t, 1, server.pollCalls())
			assert.Equal(t, 1, server.tokenCalls())
		})
	}
}

func TestStartOpenAIDeviceAuthIntervalFitsLifetime(t *testing.T) {
	for _, tc := range []struct {
		name     string
		interval any
		want     int64
	}{
		{"numeric_90", 90, 90},
		{"string_120", "120", 120},
		{"default", nil, 5},
		{"below_ttl", int64(openaioauth.DefaultDeviceAuthTTL/time.Second) - 1, int64(openaioauth.DefaultDeviceAuthTTL/time.Second) - 1},
		{"at_ttl", int64(openaioauth.DefaultDeviceAuthTTL / time.Second), 0},
		{"numeric_above_ttl", 1000, 0},
		{"string_above_ttl", "1000", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{pollSuccess: true})
			h := newDeviceAuthHandlers(t, server)
			redisServer := miniredis.RunT(t)
			client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
			t.Cleanup(func() { _ = client.Close() })
			h.Cache = cache.NewRedisCache(client)
			setDeviceAuthTestInterval(t, h, tc.interval)
			m, ok := h.DB.(*mockStore)
			require.True(t, ok)
			createCalls := 0
			create := m.createCredentialFn
			m.createCredentialFn = func(ctx context.Context, arg db.CreateCredentialParams) (db.CredentialTable, error) {
				createCalls++
				return create(ctx, arg)
			}
			result, err := h.StartOpenAIDeviceAuth(context.Background(), "org-interval", "session-interval")
			if tc.want == 0 {
				assert.True(t, errors.Is(err, openai.ErrDeviceAuthProvider), "interval_admission_safe_error")
				assert.True(t, result == (OpenAIDeviceAuthStart{}), "interval_admission_empty_result")
				assert.Zero(t, len(redisServer.Keys()), "interval_admission_no_flow")
			} else {
				require.NoError(t, err)
				record, getErr := openaioauth.NewDeviceStore(h.Cache).Get(context.Background(), result.FlowID)
				require.NoError(t, getErr)
				assert.Equal(t, tc.want, result.IntervalSeconds)
				assert.Equal(t, tc.want, record.IntervalSeconds)
				assert.True(t, record.NextPollAt.Before(record.ExpiresAt))
				assert.Equal(t, openaioauth.DefaultDeviceAuthTTL, record.ExpiresAt.Sub(record.CreatedAt))
			}
			assert.Zero(t, server.pollCalls())
			assert.Zero(t, server.tokenCalls())
			assert.Zero(t, createCalls)
		})
	}
}

func setDeviceAuthTestInterval(t *testing.T, h *Handlers, interval any) {
	t.Helper()
	transport := h.OpenAIOAuthHTTPClient.Transport
	h.OpenAIOAuthHTTPClient = &http.Client{Transport: requestContextTransport(func(r *http.Request) (*http.Response, error) {
		response, err := transport.RoundTrip(r)
		if err != nil || r.URL.Path != "/api/accounts/deviceauth/usercode" {
			return response, err
		}
		defer response.Body.Close()
		var body map[string]any
		if decodeErr := json.NewDecoder(response.Body).Decode(&body); decodeErr != nil {
			return nil, decodeErr
		}
		body["interval"] = interval
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		response.Body = io.NopCloser(strings.NewReader(string(encoded)))
		response.ContentLength = int64(len(encoded))
		return response, nil
	})}
}

func TestStartOpenAIDeviceAuthStoresSessionBoundFlow(t *testing.T) {
	server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{})
	h := newDeviceAuthHandlers(t, server)

	result, err := h.StartOpenAIDeviceAuth(context.Background(), "org-state", "session-hash")

	require.NoError(t, err)
	assert.NotEmpty(t, result.FlowID)
	assert.Equal(t, server.URL()+"/codex/device", result.VerificationURI)
	assert.Equal(t, "ABCD-EFGH", result.UserCode)
	assert.NotContains(t, result.FlowID, "[REDACTED]")

	record, err := openaioauth.NewDeviceStore(h.Cache).Get(context.Background(), result.FlowID)
	require.NoError(t, err)
	assert.Equal(t, "org-state", record.OrgID)
	assert.Equal(t, "session-hash", record.SessionBinding)
	assert.Equal(t, "[REDACTED]", record.DeviceAuthID)
}

func TestStartOpenAIDeviceAuthBoundsProviderRequest(t *testing.T) {
	server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{})
	h := newDeviceAuthHandlers(t, server)
	h.OpenAIOAuthHTTPClient = &http.Client{Transport: requestContextTransport(func(r *http.Request) (*http.Response, error) {
		if _, ok := r.Context().Deadline(); !ok {
			return nil, errors.New("missing timeout")
		}
		return nil, context.Canceled
	})}

	_, err := h.StartOpenAIDeviceAuth(context.Background(), "org-state", "session-hash")

	require.ErrorIs(t, err, context.Canceled)
}

func TestStartOpenAIDeviceAuthRequiresPersistence(t *testing.T) {
	server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{})
	h := newDeviceAuthHandlers(t, server)
	h.DB = nil

	_, err := h.StartOpenAIDeviceAuth(context.Background(), "org-state", "session-hash")

	require.ErrorIs(t, err, ErrOpenAIDeviceAuthPersistenceUnavailable)
}

func TestStartOpenAIDeviceAuthRequiresSessionBinding(t *testing.T) {
	server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{})
	h := newDeviceAuthHandlers(t, server)

	_, err := h.StartOpenAIDeviceAuth(context.Background(), "org-state", "")
	require.ErrorIs(t, err, ErrOpenAIDeviceAuthSessionRequired)
	assert.Equal(t, 0, server.pollCalls())
}

func TestPollOpenAIDeviceAuthRejectsMissingAccountIDBeforePersistence(t *testing.T) {
	server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{pollSuccess: true, omitAccountID: true})
	h := newDeviceAuthHandlers(t, server)
	started, err := h.StartOpenAIDeviceAuth(context.Background(), "org-state", "session-hash")
	require.NoError(t, err)
	makeDeviceFlowEligible(t, h.Cache, started.FlowID)

	status, err := h.PollOpenAIDeviceAuth(context.Background(), started.FlowID, "session-hash")

	require.NoError(t, err)
	assert.Equal(t, openaioauth.DeviceAuthStatusFailed, status.Status)
	assert.Equal(t, "missing_account_id", status.ErrorCode)
	assert.Equal(t, 1, server.tokenCalls())
}

func TestPollOpenAIDeviceAuthExpiredFlowRejectsWrongSession(t *testing.T) {
	server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{})
	h := newDeviceAuthHandlers(t, server)
	started, err := h.StartOpenAIDeviceAuth(context.Background(), "org-state", "session-hash")
	require.NoError(t, err)

	store := openaioauth.NewDeviceStore(h.Cache)
	record, err := store.Get(context.Background(), started.FlowID)
	require.NoError(t, err)
	now := time.Now().UTC()
	record.CreatedAt = now.Add(-2 * time.Minute)
	record.UpdatedAt = now.Add(-time.Minute)
	record.ExpiresAt = now.Add(-time.Second)
	body, err := json.Marshal(record)
	require.NoError(t, err)
	require.NoError(t, h.Cache.Set(context.Background(), openaioauth.DeviceAuthCacheKey(started.FlowID), body, time.Minute))

	_, err = h.PollOpenAIDeviceAuth(context.Background(), started.FlowID, "other-session")
	require.ErrorIs(t, err, openaioauth.ErrDeviceAuthOwnership)
}

func TestPollOpenAIDeviceAuthRejectsPKCEMismatchBeforeExchange(t *testing.T) {
	server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{pollSuccess: true, mismatchPKCE: true})
	h := newDeviceAuthHandlers(t, server)
	started, err := h.StartOpenAIDeviceAuth(context.Background(), "org-state", "session-hash")
	require.NoError(t, err)
	makeDeviceFlowEligible(t, h.Cache, started.FlowID)

	status, err := h.PollOpenAIDeviceAuth(context.Background(), started.FlowID, "session-hash")

	require.NoError(t, err)
	assert.Equal(t, openaioauth.DeviceAuthStatusFailed, status.Status)
	assert.Equal(t, "provider_error", status.ErrorCode)
	assert.Zero(t, server.tokenCalls())
}

func TestPollOpenAIDeviceAuthSuccessUsesStoredOrganizationAndPersists(t *testing.T) {
	server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{pollSuccess: true})
	h := newDeviceAuthHandlers(t, server)

	started, err := h.StartOpenAIDeviceAuth(context.Background(), "org-state", "session-hash")
	require.NoError(t, err)
	makeDeviceFlowEligible(t, h.Cache, started.FlowID)

	result, err := h.PollOpenAIDeviceAuth(context.Background(), started.FlowID, "session-hash")

	require.NoError(t, err)
	assert.Equal(t, openaioauth.DeviceAuthStatusSuccess, result.Status, "device status error=%s", result.ErrorCode)
	assert.NotEmpty(t, result.CredentialID)
	assert.Equal(t, 1, server.pollCalls())
	assert.Equal(t, 1, server.tokenCalls())
	tokenForm := server.tokenFormValues()
	assert.Equal(t, "[REDACTED]", tokenForm.Get("code"))
	assert.Equal(t, "[REDACTED]", tokenForm.Get("code_verifier"))
	assert.Equal(t, server.URL()+"/deviceauth/callback", tokenForm.Get("redirect_uri"))
	record, err := openaioauth.NewDeviceStore(h.Cache).Get(context.Background(), started.FlowID)
	require.NoError(t, err)
	assert.Equal(t, openaioauth.DeviceAuthStatusSuccess, record.Status)
	assert.Equal(t, result.CredentialID, record.CredentialID)
	assert.Empty(t, record.DeviceAuthID)
	assert.Empty(t, record.UserCode)
	assert.Empty(t, record.TokenPollURL)
	assert.Empty(t, record.VerificationURI)
	assert.Empty(t, record.RedirectURI)
}

func TestPollAndCompleteDeviceAuthDoesNotOverwriteNewerTerminalState(t *testing.T) {
	server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{pollSuccess: true})
	h := newDeviceAuthHandlers(t, server)
	store := openaioauth.NewDeviceStore(h.Cache)
	started, err := h.StartOpenAIDeviceAuth(context.Background(), "org-state", "session-hash")
	require.NoError(t, err)
	record, err := store.Get(context.Background(), started.FlowID)
	require.NoError(t, err)
	record.Status = openaioauth.DeviceAuthStatusExchanging
	record.Generation = "attempt:terminal-fixture"
	require.NoError(t, store.Save(context.Background(), record))

	dbStore, ok := h.DB.(*mockStore)
	require.True(t, ok)
	credentialID := openAIDeviceCredentialID(started.FlowID)
	dbStore.createCredentialFn = func(_ context.Context, arg db.CreateCredentialParams) (db.CredentialTable, error) {
		newer, getErr := store.Get(context.Background(), started.FlowID)
		require.NoError(t, getErr)
		newer.Status = openaioauth.DeviceAuthStatusFailed
		newer.ErrorCode = "save_failed"
		clearDeviceAuthProviderState(&newer)
		require.NoError(t, store.Save(context.Background(), newer))
		return db.CredentialTable{CredentialID: arg.CredentialID, CredentialType: CredentialTypeOpenAISubscription, OrganizationID: arg.OrganizationID}, nil
	}

	result, err := h.pollAndCompleteDeviceAuth(context.Background(), store, record)

	require.NoError(t, err)
	assert.Equal(t, openaioauth.DeviceAuthStatusFailed, result.Status)
	current, err := store.Get(context.Background(), started.FlowID)
	require.NoError(t, err)
	assert.Equal(t, openaioauth.DeviceAuthStatusFailed, current.Status)
	assert.Empty(t, current.DeviceAuthID)
	assert.Empty(t, current.UserCode)

	reconciled, err := h.PollOpenAIDeviceAuth(context.Background(), started.FlowID, "session-hash")
	require.NoError(t, err)
	assert.Equal(t, openaioauth.DeviceAuthStatusSuccess, reconciled.Status)
	assert.Equal(t, credentialID, reconciled.CredentialID)
}

func TestPollOpenAIDeviceAuthStaleExchangeRecoveryWinsOverOldWorker(t *testing.T) {
	server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{pollSuccess: true})
	h := newDeviceAuthHandlers(t, server)
	started, err := h.StartOpenAIDeviceAuth(context.Background(), "org-state", "session-hash")
	require.NoError(t, err)

	store := openaioauth.NewDeviceStore(h.Cache)
	record, err := store.Get(context.Background(), started.FlowID)
	require.NoError(t, err)
	record.Status = openaioauth.DeviceAuthStatusExchanging
	record.UpdatedAt = time.Now().UTC().Add(-deviceAuthExchangeLease - time.Second)
	staleBody, err := json.Marshal(record)
	require.NoError(t, err)
	require.NoError(t, h.Cache.Set(context.Background(), openaioauth.DeviceAuthCacheKey(started.FlowID), staleBody, time.Minute))
	staleWorkerRecord := record

	result, err := h.PollOpenAIDeviceAuth(context.Background(), started.FlowID, "session-hash")
	require.NoError(t, err)
	assert.Equal(t, openaioauth.DeviceAuthStatusFailed, result.Status)
	assert.Equal(t, "exchange_interrupted", result.ErrorCode)
	assert.Zero(t, server.pollCalls())

	staleSuccess := staleWorkerRecord
	staleSuccess.Status = openaioauth.DeviceAuthStatusSuccess
	staleSuccess.CredentialID = "credential-id"
	clearDeviceAuthProviderState(&staleSuccess)
	applied, err := store.SaveIfCurrent(context.Background(), staleWorkerRecord, staleSuccess)
	require.NoError(t, err)
	assert.False(t, applied)
	current, err := store.Get(context.Background(), started.FlowID)
	require.NoError(t, err)
	assert.Equal(t, openaioauth.DeviceAuthStatusFailed, current.Status)
	assert.Equal(t, "exchange_interrupted", current.ErrorCode)
}

func TestPollOpenAIDeviceAuthRecoversCredentialWhenTerminalSaveFails(t *testing.T) {
	server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{pollSuccess: true})
	h := newDeviceAuthHandlers(t, server)
	h.Cache = &failingDeviceAuthCache{MemoryCache: cache.NewMemoryCache(), failTerminalSave: true}
	dbStore, ok := h.DB.(*mockStore)
	require.True(t, ok)
	dbStore.createCredentialFn = func(_ context.Context, arg db.CreateCredentialParams) (db.CredentialTable, error) {
		return db.CredentialTable{CredentialID: arg.CredentialID, CredentialType: CredentialTypeOpenAISubscription, OrganizationID: arg.OrganizationID}, nil
	}

	started, err := h.StartOpenAIDeviceAuth(context.Background(), "org-state", "session-hash")
	require.NoError(t, err)
	makeDeviceFlowEligible(t, h.Cache, started.FlowID)

	_, err = h.PollOpenAIDeviceAuth(context.Background(), started.FlowID, "session-hash")
	require.Error(t, err)
	record, getErr := openaioauth.NewDeviceStore(h.Cache).Get(context.Background(), started.FlowID)
	require.NoError(t, getErr)
	assert.Equal(t, openaioauth.DeviceAuthStatusExchanging, record.Status)
	assert.Empty(t, record.CredentialID)

	result, err := h.PollOpenAIDeviceAuth(context.Background(), started.FlowID, "session-hash")

	require.NoError(t, err)
	assert.Equal(t, openaioauth.DeviceAuthStatusSuccess, result.Status)
	assert.NotEmpty(t, result.CredentialID)
	assert.Equal(t, 1, server.pollCalls())
	assert.Equal(t, 1, server.tokenCalls())
	finalRecord, getErr := openaioauth.NewDeviceStore(h.Cache).Get(context.Background(), started.FlowID)
	require.NoError(t, getErr)
	assert.Equal(t, openaioauth.DeviceAuthStatusSuccess, finalRecord.Status)
	assert.Equal(t, result.CredentialID, finalRecord.CredentialID)
	assert.Empty(t, finalRecord.DeviceAuthID)
	assert.Empty(t, finalRecord.UserCode)
	assert.Empty(t, finalRecord.TokenPollURL)
	assert.Empty(t, finalRecord.VerificationURI)
	assert.Empty(t, finalRecord.RedirectURI)
}

func TestPollOpenAIDeviceAuthRejectsWrongSession(t *testing.T) {
	server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{})
	h := newDeviceAuthHandlers(t, server)
	started, err := h.StartOpenAIDeviceAuth(context.Background(), "org-state", "session-hash")
	require.NoError(t, err)

	_, err = h.PollOpenAIDeviceAuth(context.Background(), started.FlowID, "other-session")

	require.Error(t, err)
	assert.ErrorIs(t, err, openaioauth.ErrDeviceAuthOwnership)
	assert.Zero(t, server.pollCalls())
}

func TestPollOpenAIDeviceAuthRespectsProviderInterval(t *testing.T) {
	server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{})
	h := newDeviceAuthHandlers(t, server)
	started, err := h.StartOpenAIDeviceAuth(context.Background(), "org-state", "session-hash")
	require.NoError(t, err)

	result, err := h.PollOpenAIDeviceAuth(context.Background(), started.FlowID, "session-hash")

	require.NoError(t, err)
	assert.Equal(t, openaioauth.DeviceAuthStatusPending, result.Status)
	assert.Zero(t, server.pollCalls())
}

func TestPollOpenAIDeviceAuthMapsSlowDownAndDenial(t *testing.T) {
	for _, tc := range []struct {
		name       string
		pollStatus int
		pollBody   string
		wantStatus openaioauth.DeviceAuthStatus
		wantError  string
	}{
		{name: "slow_down", pollStatus: http.StatusBadRequest, pollBody: `{"error":"slow_down"}`, wantStatus: openaioauth.DeviceAuthStatusPending, wantError: "slow_down"},
		{name: "denied", pollStatus: http.StatusBadRequest, pollBody: `{"error":"access_denied"}`, wantStatus: openaioauth.DeviceAuthStatusDenied, wantError: "access_denied"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{pollStatus: tc.pollStatus, pollBody: tc.pollBody})
			h := newDeviceAuthHandlers(t, server)
			started, err := h.StartOpenAIDeviceAuth(context.Background(), "org-state", "session-hash")
			require.NoError(t, err)
			makeDeviceFlowEligible(t, h.Cache, started.FlowID)

			result, err := h.PollOpenAIDeviceAuth(context.Background(), started.FlowID, "session-hash")

			require.NoError(t, err)
			assert.Equal(t, tc.wantStatus, result.Status)
			assert.Equal(t, tc.wantError, result.ErrorCode)
		})
	}
}

func TestPollOpenAIDeviceAuthPollCancellationFailsTerminallyAndDoesNotReplay(t *testing.T) {
	server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{pollSuccess: true})
	h := newDeviceAuthHandlers(t, server)
	started, err := h.StartOpenAIDeviceAuth(context.Background(), "org-state", "session-hash")
	require.NoError(t, err)
	makeDeviceFlowEligible(t, h.Cache, started.FlowID)

	baseClient := server.Client()
	h.OpenAIOAuthHTTPClient = &http.Client{Transport: requestContextTransport(func(r *http.Request) (*http.Response, error) {
		response, roundTripErr := baseClient.Transport.RoundTrip(r)
		if roundTripErr != nil {
			return nil, roundTripErr
		}
		if r.URL.Path == "/api/accounts/deviceauth/token" {
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
			return nil, context.Canceled
		}
		return response, nil
	})}

	status, err := h.PollOpenAIDeviceAuth(context.Background(), started.FlowID, "session-hash")
	require.NoError(t, err)
	assert.Equal(t, openaioauth.DeviceAuthStatusFailed, status.Status)
	assert.Equal(t, "exchange_failed", status.ErrorCode)
	assert.Equal(t, 1, server.pollCalls())

	makeDeviceFlowEligible(t, h.Cache, started.FlowID)
	status, err = h.PollOpenAIDeviceAuth(context.Background(), started.FlowID, "session-hash")
	require.NoError(t, err)
	assert.Equal(t, openaioauth.DeviceAuthStatusFailed, status.Status)
	assert.Equal(t, "exchange_failed", status.ErrorCode)
	assert.Equal(t, 1, server.pollCalls())

	record, getErr := openaioauth.NewDeviceStore(h.Cache).Get(context.Background(), started.FlowID)
	require.NoError(t, getErr)
	assert.Empty(t, record.DeviceAuthID)
	assert.Empty(t, record.UserCode)
	assert.Empty(t, record.TokenPollURL)
	assert.Empty(t, record.VerificationURI)
	assert.Empty(t, record.RedirectURI)
}

func TestPollOpenAIDeviceAuthDeadlineDuringPollFailsTerminally(t *testing.T) {
	server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{pollSuccess: true})
	h := newDeviceAuthHandlers(t, server)
	started, err := h.StartOpenAIDeviceAuth(context.Background(), "org-state", "session-hash")
	require.NoError(t, err)
	makeDeviceFlowEligible(t, h.Cache, started.FlowID)
	h.OpenAIOAuthHTTPClient = &http.Client{Transport: requestContextTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/api/accounts/deviceauth/token" {
			return nil, context.DeadlineExceeded
		}
		return server.Client().Transport.RoundTrip(r)
	})}

	status, err := h.PollOpenAIDeviceAuth(context.Background(), started.FlowID, "session-hash")
	require.NoError(t, err)
	assert.Equal(t, openaioauth.DeviceAuthStatusFailed, status.Status)
	assert.Equal(t, "exchange_failed", status.ErrorCode)
}

func TestPollOpenAIDeviceAuthCancelledExchangeFailsTerminally(t *testing.T) {
	server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{pollSuccess: true})
	h := newDeviceAuthHandlers(t, server)
	started, err := h.StartOpenAIDeviceAuth(context.Background(), "org-state", "session-hash")
	require.NoError(t, err)
	makeDeviceFlowEligible(t, h.Cache, started.FlowID)

	baseClient := server.Client()
	h.OpenAIOAuthHTTPClient = &http.Client{Transport: requestContextTransport(func(r *http.Request) (*http.Response, error) {
		response, roundTripErr := baseClient.Transport.RoundTrip(r)
		if roundTripErr != nil {
			return nil, roundTripErr
		}
		if r.URL.Path == "/oauth/token" {
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
			return nil, context.Canceled
		}
		return response, nil
	})}

	status, err := h.PollOpenAIDeviceAuth(context.Background(), started.FlowID, "session-hash")
	require.NoError(t, err)
	assert.Equal(t, openaioauth.DeviceAuthStatusFailed, status.Status)
	assert.Equal(t, "exchange_failed", status.ErrorCode)
	assert.Equal(t, 1, server.pollCalls())
	assert.Equal(t, 1, server.tokenCalls())

	record, getErr := openaioauth.NewDeviceStore(h.Cache).Get(context.Background(), started.FlowID)
	require.NoError(t, getErr)
	assert.Equal(t, openaioauth.DeviceAuthStatusFailed, record.Status)
	assert.Empty(t, record.DeviceAuthID)
	assert.Empty(t, record.UserCode)
	assert.Empty(t, record.RedirectURI)
}

func TestDeviceAuthBackoffRetainsProviderFloor(t *testing.T) {
	for _, tc := range []struct {
		interval int64
		retry    int
		want     time.Duration
	}{
		{5, 1, 5 * time.Second}, {5, 2, 10 * time.Second}, {5, 5, time.Minute},
		{30, 2, time.Minute}, {90, 1, 90 * time.Second}, {90, 2, 90 * time.Second},
		{120, 1, 120 * time.Second}, {120, 2, 120 * time.Second},
		{95, 1, 95 * time.Second}, {95, 2, 95 * time.Second},
	} {
		t.Run(fmt.Sprintf("%d_%d", tc.interval, tc.retry), func(t *testing.T) {
			assert.Equal(t, tc.want, deviceAuthBackoff(tc.interval, tc.retry), "interval_backoff_floor")
		})
	}
}

func TestPollOpenAIDeviceAuthTransientRetainsProviderFloor(t *testing.T) {
	for _, interval := range []int64{90, 120} {
		t.Run(fmt.Sprint(interval), func(t *testing.T) {
			server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{pollStatus: http.StatusServiceUnavailable, pollBody: `{"error":"temporary"}`})
			h := newDeviceAuthHandlers(t, server)
			setDeviceAuthTestInterval(t, h, interval)
			// Exercise slow_down before the explicit transient response as well.
			transport := h.OpenAIOAuthHTTPClient.Transport
			firstPoll := true
			h.OpenAIOAuthHTTPClient = &http.Client{Transport: requestContextTransport(func(r *http.Request) (*http.Response, error) {
				response, err := transport.RoundTrip(r)
				if err == nil && r.URL.Path == "/api/accounts/deviceauth/token" && firstPoll {
					firstPoll = false
					_ = response.Body.Close()
					response.StatusCode = http.StatusBadRequest
					response.Body = io.NopCloser(strings.NewReader(`{"error":"slow_down"}`))
				}
				return response, err
			})}
			m, ok := h.DB.(*mockStore)
			require.True(t, ok)
			createCalls := 0
			create := m.createCredentialFn
			m.createCredentialFn = func(ctx context.Context, arg db.CreateCredentialParams) (db.CredentialTable, error) {
				createCalls++
				return create(ctx, arg)
			}
			ctx := context.Background()
			started, err := h.StartOpenAIDeviceAuth(ctx, "org-interval", "session-interval")
			require.NoError(t, err)
			makeDeviceFlowEligible(t, h.Cache, started.FlowID)
			slow, err := h.PollOpenAIDeviceAuth(ctx, started.FlowID, "session-interval")
			require.NoError(t, err)
			require.Equal(t, "slow_down", slow.ErrorCode)
			require.Equal(t, interval+5, slow.IntervalSeconds)
			for retry := 1; retry <= 2; retry++ {
				// Controlled due state, not a production clock or a minute-long sleep.
				makeDeviceFlowEligible(t, h.Cache, started.FlowID)
				before := time.Now()
				status, err := h.PollOpenAIDeviceAuth(ctx, started.FlowID, "session-interval")
				require.NoError(t, err)
				assert.Equal(t, retry+1, server.pollCalls(), "due positive control reaches provider")
				assert.Equal(t, openaioauth.DeviceAuthStatusPending, status.Status)
				assert.Equal(t, "temporarily_unavailable", status.ErrorCode)
				assert.False(t, status.NextPollAt.Before(before.Add(time.Duration(interval+5)*time.Second)), "interval_transient_status_floor")
				record, err := openaioauth.NewDeviceStore(h.Cache).Get(ctx, started.FlowID)
				require.NoError(t, err)
				assert.True(t, record.NextPollAt.Equal(status.NextPollAt), "interval_transient_stored_floor")
				assert.Equal(t, retry, record.RetryCount)
				early, err := h.PollOpenAIDeviceAuth(ctx, started.FlowID, "session-interval")
				require.NoError(t, err)
				assert.True(t, early.NextPollAt.Equal(status.NextPollAt))
				assert.Equal(t, retry+1, server.pollCalls(), "interval_transient_no_early_poll")
			}
			assert.Zero(t, server.tokenCalls())
			assert.Zero(t, createCalls)
		})
	}
}

func TestPollOpenAIDeviceAuthRetriesTransientFailureWithBound(t *testing.T) {
	server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{pollStatus: http.StatusInternalServerError, pollBody: `{"error":"temporary"}`})
	h := newDeviceAuthHandlers(t, server)
	started, err := h.StartOpenAIDeviceAuth(context.Background(), "org-state", "session-hash")
	require.NoError(t, err)

	for attempt := 1; attempt <= openaioauth.DeviceAuthRetryLimit; attempt++ {
		makeDeviceFlowEligible(t, h.Cache, started.FlowID)
		result, pollErr := h.PollOpenAIDeviceAuth(context.Background(), started.FlowID, "session-hash")
		require.NoError(t, pollErr)
		if attempt < openaioauth.DeviceAuthRetryLimit {
			assert.Equal(t, openaioauth.DeviceAuthStatusPending, result.Status)
		} else {
			assert.Equal(t, openaioauth.DeviceAuthStatusFailed, result.Status)
			assert.Equal(t, "provider_unavailable", result.ErrorCode)
		}
	}
	assert.Equal(t, openaioauth.DeviceAuthRetryLimit, server.pollCalls())
	assert.Zero(t, server.tokenCalls())
}

func TestCancelOpenAIDeviceAuthIsOwnedAndIdempotent(t *testing.T) {
	server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{})
	h := newDeviceAuthHandlers(t, server)
	started, err := h.StartOpenAIDeviceAuth(context.Background(), "org-state", "session-hash")
	require.NoError(t, err)

	require.ErrorIs(t, h.CancelOpenAIDeviceAuth(context.Background(), started.FlowID, "other-session"), openaioauth.ErrDeviceAuthOwnership)
	require.NoError(t, h.CancelOpenAIDeviceAuth(context.Background(), started.FlowID, "session-hash"))
	require.NoError(t, h.CancelOpenAIDeviceAuth(context.Background(), started.FlowID, "session-hash"))

	record, err := openaioauth.NewDeviceStore(h.Cache).Get(context.Background(), started.FlowID)
	require.NoError(t, err)
	assert.Equal(t, openaioauth.DeviceAuthStatusCancelled, record.Status)
	assert.Empty(t, record.DeviceAuthID)
	assert.Empty(t, record.UserCode)
}

func TestCancelOpenAIDeviceAuthDoesNotClaimInProgressExchange(t *testing.T) {
	server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{})
	h := newDeviceAuthHandlers(t, server)
	started, err := h.StartOpenAIDeviceAuth(context.Background(), "org-state", "session-hash")
	require.NoError(t, err)
	record, err := openaioauth.NewDeviceStore(h.Cache).Get(context.Background(), started.FlowID)
	require.NoError(t, err)
	record.Status = openaioauth.DeviceAuthStatusExchanging
	require.NoError(t, openaioauth.NewDeviceStore(h.Cache).Save(context.Background(), record))

	err = h.CancelOpenAIDeviceAuth(context.Background(), started.FlowID, "session-hash")

	require.ErrorIs(t, err, ErrOpenAIDeviceAuthExchangeInProgress)
	current, getErr := openaioauth.NewDeviceStore(h.Cache).Get(context.Background(), started.FlowID)
	require.NoError(t, getErr)
	assert.Equal(t, openaioauth.DeviceAuthStatusExchanging, current.Status)
}

func TestCancelOpenAIDeviceAuthReturnsNilWhenConcurrentExchangeAlreadyTerminal(t *testing.T) {
	server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{})
	h := newDeviceAuthHandlers(t, server)
	started, err := h.StartOpenAIDeviceAuth(context.Background(), "org-state", "session-hash")
	require.NoError(t, err)
	store := openaioauth.NewDeviceStore(h.Cache)
	record, err := store.Get(context.Background(), started.FlowID)
	require.NoError(t, err)

	locker, ok := h.Cache.(cache.LockCache)
	require.True(t, ok)
	require.NoError(t, err)
	locked, err := locker.AcquireLock(context.Background(), openaioauth.DeviceAuthLockKey(started.FlowID), "[REDACTED]", time.Minute)
	require.NoError(t, err)
	require.True(t, locked)
	defer func() {
		if releaseErr := locker.ReleaseLock(context.Background(), openaioauth.DeviceAuthLockKey(started.FlowID), "[REDACTED]"); releaseErr != nil {
			t.Errorf("release device auth lock: %v", releaseErr)
		}
	}()
	record.Status = openaioauth.DeviceAuthStatusSuccess
	record.CredentialID = "cred-id"
	clearDeviceAuthProviderState(&record)
	require.NoError(t, store.Save(context.Background(), record))

	require.NoError(t, h.CancelOpenAIDeviceAuth(context.Background(), started.FlowID, "session-hash"))
}

func TestPollOpenAIDeviceAuthConcurrentRequestsPollOnce(t *testing.T) {
	server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{pollSuccess: true, blockPoll: true})
	h := newDeviceAuthHandlers(t, server)
	started, err := h.StartOpenAIDeviceAuth(context.Background(), "org-state", "session-hash")
	require.NoError(t, err)
	makeDeviceFlowEligible(t, h.Cache, started.FlowID)

	results := make(chan OpenAIDeviceAuthStatus, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, pollErr := h.PollOpenAIDeviceAuth(context.Background(), started.FlowID, "session-hash")
			results <- result
			errs <- pollErr
		}()
	}
	<-server.pollStarted
	close(server.releasePoll)
	wg.Wait()
	close(results)
	close(errs)

	assert.Equal(t, 1, server.pollCalls())
	assert.Equal(t, 1, server.tokenCalls())
	for err := range errs {
		assert.NoError(t, err)
	}
	for result := range results {
		assert.Contains(t, []openaioauth.DeviceAuthStatus{openaioauth.DeviceAuthStatusPending, openaioauth.DeviceAuthStatusExchanging, openaioauth.DeviceAuthStatusSuccess}, result.Status)
	}
}

func TestPollOpenAIDeviceAuthIndependentCachesPollOnce(t *testing.T) {
	server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{pollSuccess: true, blockPoll: true})
	first := newDeviceAuthHandlers(t, server)
	second := newDeviceAuthHandlers(t, server)
	second.DB = first.DB

	redisServer := miniredis.RunT(t)
	newCache := func() cache.Cache {
		client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
		t.Cleanup(func() { _ = client.Close() })
		return cache.NewDualCache(cache.NewMemoryCache(), cache.NewRedisCache(client))
	}
	first.Cache = newCache()
	second.Cache = newCache()
	first.OpenAIOAuthHTTPClient = server.Client()
	second.OpenAIOAuthHTTPClient = server.Client()

	var createMu sync.Mutex
	createCalls := 0
	dbStore, ok := first.DB.(*mockStore)
	require.True(t, ok)
	dbStore.createCredentialFn = func(_ context.Context, arg db.CreateCredentialParams) (db.CredentialTable, error) {
		createMu.Lock()
		createCalls++
		createMu.Unlock()
		return db.CredentialTable{CredentialID: arg.CredentialID, CredentialType: arg.CredentialType, CredentialValue: arg.CredentialValue, CredentialInfo: arg.CredentialInfo, OrganizationID: arg.OrganizationID}, nil
	}

	started, err := first.StartOpenAIDeviceAuth(context.Background(), "org-state", "session-hash")
	require.NoError(t, err)
	makeDeviceFlowEligible(t, first.Cache, started.FlowID)

	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, handler := range []*Handlers{first, second} {
		wg.Add(1)
		go func(h *Handlers) {
			defer wg.Done()
			_, pollErr := h.PollOpenAIDeviceAuth(context.Background(), started.FlowID, "session-hash")
			errs <- pollErr
		}(handler)
	}
	<-server.pollStarted
	close(server.releasePoll)
	wg.Wait()
	close(errs)

	for pollErr := range errs {
		require.NoError(t, pollErr)
	}
	assert.Equal(t, 1, server.pollCalls())
	assert.Equal(t, 1, server.tokenCalls())
	assert.Equal(t, 1, createCalls)
	finalRecord, getErr := openaioauth.NewDeviceStore(second.Cache).Get(context.Background(), started.FlowID)
	require.NoError(t, getErr)
	assert.Equal(t, openaioauth.DeviceAuthStatusSuccess, finalRecord.Status)
	assert.NotEmpty(t, finalRecord.CredentialID)
}

// ExchangeCode has decoded a successful mock response before its deferred Close.
// Hold that return boundary, not CreateCredential: recovery must win before INSERT starts.
type durableFenceResponseBarrier struct {
	io.ReadCloser
	entered chan struct{}
	release chan struct{}
}

func (b *durableFenceResponseBarrier) Close() error {
	err := b.ReadCloser.Close()
	close(b.entered)
	<-b.release
	return err
}

func TestDurableFenceRecoveryBeforeInsertMustRejectOldWorker(t *testing.T) {
	ctx := context.Background()
	server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{pollSuccess: true})
	first := newDeviceAuthHandlers(t, server)
	second := newDeviceAuthHandlers(t, server)
	second.DB = first.DB
	redisServer := miniredis.RunT(t)
	newCache := func() cache.Cache {
		client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
		t.Cleanup(func() { _ = client.Close() })
		return cache.NewDualCache(cache.NewMemoryCache(), cache.NewRedisCache(client))
	}
	first.Cache, second.Cache = newCache(), newCache()

	var mu sync.Mutex
	var durable db.CredentialTable
	inserts := 0
	m, ok := first.DB.(*mockStore)
	require.True(t, ok)
	m.createCredentialFn = func(_ context.Context, arg db.CreateCredentialParams) (db.CredentialTable, error) {
		mu.Lock()
		defer mu.Unlock()
		inserts++
		durable = db.CredentialTable{CredentialID: arg.CredentialID, CredentialType: arg.CredentialType, OrganizationID: arg.OrganizationID}
		return durable, nil
	}
	m.getCredentialFn = func(_ context.Context, id string) (db.CredentialTable, error) {
		mu.Lock()
		defer mu.Unlock()
		if durable.CredentialID == "" || durable.CredentialID != id {
			return db.CredentialTable{}, pgx.ErrNoRows
		}
		return durable, nil
	}
	insertCount := func() int { mu.Lock(); defer mu.Unlock(); return inserts }

	started, err := first.StartOpenAIDeviceAuth(ctx, "org-fence", "session-fence")
	require.NoError(t, err)
	makeDeviceFlowEligible(t, first.Cache, started.FlowID)
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	transport := server.Client().Transport
	first.OpenAIOAuthHTTPClient = &http.Client{Transport: requestContextTransport(func(req *http.Request) (*http.Response, error) {
		resp, roundTripErr := transport.RoundTrip(req)
		if roundTripErr == nil && req.URL.Path == "/oauth/token" {
			resp.Body = &durableFenceResponseBarrier{ReadCloser: resp.Body, entered: entered, release: release}
		}
		return resp, roundTripErr
	})}
	done := make(chan error, 1)
	go func() {
		_, pollErr := first.PollOpenAIDeviceAuth(ctx, started.FlowID, "session-fence")
		done <- pollErr
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("worker did not reach successful exchange return barrier")
	}
	require.Zero(t, insertCount(), "INSERT must not have begun at the barrier")
	require.Equal(t, 1, server.pollCalls())
	require.Equal(t, 1, server.tokenCalls())
	require.True(t, redisServer.Exists(openaioauth.DeviceAuthLockKey(started.FlowID)))

	// Simulate only lease time, without sleeping or changing flow expiry/ownership.
	// miniredis advances Redis TTL; age UpdatedAt separately because Go's wall clock does not advance.
	redisServer.FastForward(deviceAuthExchangeLease + time.Second)
	require.False(t, redisServer.Exists(openaioauth.DeviceAuthLockKey(started.FlowID)))
	store := openaioauth.NewDeviceStore(second.Cache)
	record, err := store.Get(ctx, started.FlowID)
	require.NoError(t, err)
	require.Equal(t, openaioauth.DeviceAuthStatusExchanging, record.Status)
	require.True(t, time.Now().UTC().Before(record.ExpiresAt))
	record.UpdatedAt = time.Now().UTC().Add(-deviceAuthExchangeLease - time.Second)
	body, err := json.Marshal(record)
	require.NoError(t, err)
	require.NoError(t, second.Cache.Set(ctx, openaioauth.DeviceAuthCacheKey(record.FlowID), body, time.Until(record.ExpiresAt)+time.Minute))

	recovered, err := second.PollOpenAIDeviceAuth(ctx, started.FlowID, "session-fence")
	require.NoError(t, err)
	require.Equal(t, openaioauth.DeviceAuthStatusFailed, recovered.Status)
	require.Equal(t, "exchange_interrupted", recovered.ErrorCode)
	require.Zero(t, insertCount(), "recovery won before any durable insert")
	won, err := store.Get(ctx, started.FlowID)
	require.NoError(t, err)
	require.Equal(t, openaioauth.DeviceAuthStatusFailed, won.Status)

	unblock()
	select {
	case err = <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("old worker did not finish")
	}
	assert.Zero(t, insertCount(), "CODE-001: recovered attempt must not create a durable credential")
	afterWorker, err := store.Get(ctx, started.FlowID)
	require.NoError(t, err)
	assert.Equal(t, openaioauth.DeviceAuthStatusFailed, afterWorker.Status)
	final, err := second.PollOpenAIDeviceAuth(ctx, started.FlowID, "session-fence")
	require.NoError(t, err)
	assert.Equal(t, openaioauth.DeviceAuthStatusFailed, final.Status, "reconciliation must not legitimize a stale insert")
	finalRecord, err := store.Get(ctx, started.FlowID)
	require.NoError(t, err)
	assert.Equal(t, openaioauth.DeviceAuthStatusFailed, finalRecord.Status)
	assert.Equal(t, "exchange_interrupted", finalRecord.ErrorCode)
	assert.Equal(t, 1, server.pollCalls())
	assert.Equal(t, 1, server.tokenCalls())
	t.Logf("barrier=recovery_before_insert create_calls=%d poll_calls=%d exchange_calls=%d recovered=%s after_worker=%s final=%s", insertCount(), server.pollCalls(), server.tokenCalls(), recovered.Status, afterWorker.Status, finalRecord.Status)
}

func TestDeviceFenceExpiredExchangeCancelUsesArbitration(t *testing.T) {
	ctx := context.Background()
	server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{})
	h := newDeviceAuthHandlers(t, server)
	started, err := h.StartOpenAIDeviceAuth(ctx, "", "expired-cancel-session")
	require.NoError(t, err)
	record := expireDeviceFlow(t, h.Cache, started.FlowID)
	record.Status = openaioauth.DeviceAuthStatusExchanging
	record.Generation = "attempt:expired-cancel"
	body, err := json.Marshal(record)
	require.NoError(t, err)
	require.NoError(t, h.Cache.Set(ctx, openaioauth.DeviceAuthCacheKey(record.FlowID), body, time.Minute-time.Second))
	reads := 0
	m, ok := h.DB.(*mockStore)
	require.True(t, ok)
	m.getCredentialFn = func(context.Context, string) (db.CredentialTable, error) {
		reads++
		return db.CredentialTable{}, pgx.ErrNoRows
	}
	require.NoError(t, h.CancelOpenAIDeviceAuth(ctx, record.FlowID, record.SessionBinding))
	current, err := openaioauth.NewDeviceStore(h.Cache).Get(ctx, record.FlowID)
	require.ErrorIs(t, err, openaioauth.ErrDeviceAuthExpired)
	assert.Equal(t, openaioauth.DeviceAuthStatusExpired, current.Status)
	assert.True(t, current.DeviceAuthID == "" && current.UserCode == "" && current.TokenPollURL == "" && current.VerificationURI == "" && current.RedirectURI == "")
	assert.Equal(t, 1, reads)
	assert.Zero(t, len(m.deviceRows))
	assert.Zero(t, server.pollCalls())
	assert.Zero(t, server.tokenCalls())
}

func TestDeviceFenceCleanupDoesNotUseCancelledBeginContext(t *testing.T) {
	m := newMockStore()
	h := mockHandlers(m)
	cleanup := false
	m.beginTxFn = func(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
		tx, err := m.beginDeviceMockTx(ctx, opts)
		if err != nil {
			return nil, err
		}
		mockTx, ok := tx.(*deviceMockTx)
		require.True(t, ok)
		return &deviceCleanupTx{deviceMockTx: mockTx, check: func(ctx context.Context) {
			cleanup = true
			assert.NoError(t, ctx.Err())
			deadline, ok := ctx.Deadline()
			assert.True(t, ok)
			assert.LessOrEqual(t, time.Until(deadline), deviceAuthStateTimeout)
		}}, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := h.withDeviceAuthTx(ctx, "cleanup-fixture", func(context.Context, *db.Queries) error { cancel(); return context.Canceled })
	assert.ErrorIs(t, err, context.Canceled)
	assert.True(t, cleanup)
}

type deviceCleanupTx struct {
	*deviceMockTx
	check func(context.Context)
}

func (tx *deviceCleanupTx) Rollback(ctx context.Context) error {
	tx.check(ctx)
	return tx.deviceMockTx.Rollback(ctx)
}

func TestDeviceFenceDatabaseErrorsDoNotRevokeOrAdmit(t *testing.T) {
	for _, action := range []string{"recovery", "save", "cancel"} {
		t.Run(action, func(t *testing.T) {
			ctx := context.Background()
			server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{})
			h := newDeviceAuthHandlers(t, server)
			started, err := h.StartOpenAIDeviceAuth(ctx, "", "session-db-error")
			require.NoError(t, err)
			store := openaioauth.NewDeviceStore(h.Cache)
			record, err := store.Get(ctx, started.FlowID)
			require.NoError(t, err)
			if action != "cancel" {
				record.Status = openaioauth.DeviceAuthStatusExchanging
				record.Generation = "attempt:db-error"
			}
			if action == "recovery" {
				record.UpdatedAt = time.Now().UTC().Add(-2*time.Minute - time.Second)
			}
			body, err := json.Marshal(record)
			require.NoError(t, err)
			key := openaioauth.DeviceAuthCacheKey(record.FlowID)
			require.NoError(t, h.Cache.Set(ctx, key, body, time.Minute))
			m, ok := h.DB.(*mockStore)
			require.True(t, ok)
			m.getCredentialFn = func(context.Context, string) (db.CredentialTable, error) {
				return db.CredentialTable{}, context.DeadlineExceeded
			}
			switch action {
			case "recovery":
				_, err = h.PollOpenAIDeviceAuth(ctx, record.FlowID, record.SessionBinding)
			case "save":
				_, err = h.SaveOpenAISubscriptionCredentialForFlow(ctx, &record, "OpenAI Subscription", validOpenAITokenBundle(), OpenAISubscriptionCredentialInfo{})
			case "cancel":
				err = h.CancelOpenAIDeviceAuth(ctx, record.FlowID, record.SessionBinding)
			}
			assert.ErrorIs(t, err, context.DeadlineExceeded)
			after, readErr := h.Cache.Get(ctx, key)
			require.NoError(t, readErr)
			assert.True(t, string(body) == string(after), "query failure must not change cache state")
			assert.Zero(t, len(m.deviceRows))
			assert.Zero(t, server.pollCalls())
			assert.Zero(t, server.tokenCalls())
		})
	}
}

func TestDeviceFenceRecoveryAuditsOnlyWinningDecision(t *testing.T) {
	server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{})
	h := newDeviceAuthHandlers(t, server)
	started, err := h.StartOpenAIDeviceAuth(context.Background(), "", "session-recovery-audit")
	require.NoError(t, err)
	store := openaioauth.NewDeviceStore(h.Cache)
	record, err := store.Get(context.Background(), started.FlowID)
	require.NoError(t, err)
	record.Status = openaioauth.DeviceAuthStatusExchanging
	record.Generation = "attempt:audit-fixture"
	record.UpdatedAt = time.Now().UTC().Add(-2*time.Minute - time.Second)
	body, err := json.Marshal(record)
	require.NoError(t, err)
	require.NoError(t, h.Cache.Set(context.Background(), openaioauth.DeviceAuthCacheKey(record.FlowID), body, time.Minute))
	h.Config.GeneralSettings.StoreAuditLogs = true
	audits := 0
	m, ok := h.DB.(*mockStore)
	require.True(t, ok)
	m.insertAuditLogFn = func(_ context.Context, arg db.InsertAuditLogParams) (db.AuditLog, error) {
		audits++
		return db.AuditLog{}, nil
	}
	for range 2 {
		status, err := h.PollOpenAIDeviceAuth(context.Background(), record.FlowID, record.SessionBinding)
		require.NoError(t, err)
		assert.Equal(t, openaioauth.DeviceAuthStatusFailed, status.Status)
	}
	assert.Equal(t, 1, audits)
	assert.Zero(t, server.pollCalls())
	assert.Zero(t, server.tokenCalls())
}

func TestDeviceFenceLegacyFlowCannotAcquireNewAdmission(t *testing.T) {
	server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{pollSuccess: true})
	h := newDeviceAuthHandlers(t, server)
	started, err := h.StartOpenAIDeviceAuth(context.Background(), "", "legacy-session")
	require.NoError(t, err)
	store := openaioauth.NewDeviceStore(h.Cache)
	record, err := store.Get(context.Background(), started.FlowID)
	require.NoError(t, err)
	record.Generation = ""
	record.NextPollAt = time.Now().UTC().Add(-time.Second)
	require.NoError(t, store.Save(context.Background(), record))
	status, err := h.PollOpenAIDeviceAuth(context.Background(), record.FlowID, record.SessionBinding)
	require.NoError(t, err)
	assert.Equal(t, openaioauth.DeviceAuthStatusFailed, status.Status)
	m, ok := h.DB.(*mockStore)
	require.True(t, ok)
	assert.Zero(t, len(m.deviceRows))
	assert.Zero(t, server.pollCalls())
	assert.Zero(t, server.tokenCalls())
}

type deviceFenceCASCache struct {
	deviceTestSharedMemoryCache
	beforeCAS func(openaioauth.DeviceAuthRecord, openaioauth.DeviceAuthRecord)
}

func (c *deviceFenceCASCache) CompareAndSet(ctx context.Context, key string, expected, value []byte, ttl time.Duration) (bool, error) {
	var before, after openaioauth.DeviceAuthRecord
	if json.Unmarshal(expected, &before) != nil || json.Unmarshal(value, &after) != nil {
		return false, errors.New("invalid fixture record")
	}
	if c.beforeCAS != nil {
		c.beforeCAS(before, after)
	}
	return c.MemoryCache.CompareAndSet(ctx, key, expected, value, ttl)
}

func TestDeviceFenceConsumedAdmissionCannotBeReusedAfterDeletion(t *testing.T) {
	server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{pollSuccess: true})
	h := newDeviceAuthHandlers(t, server)
	started, err := h.StartOpenAIDeviceAuth(context.Background(), "", "session-consumed")
	require.NoError(t, err)
	makeDeviceFlowEligible(t, h.Cache, started.FlowID)
	// Leave the admitted state in cache, as when terminal publication is lost.
	sharedCache, ok := h.Cache.(deviceTestSharedMemoryCache)
	require.True(t, ok)
	h.Cache = &failingDeviceAuthCache{MemoryCache: sharedCache.MemoryCache, failTerminalSave: true}
	_, err = h.PollOpenAIDeviceAuth(context.Background(), started.FlowID, "session-consumed")
	require.Error(t, err)
	record, err := openaioauth.NewDeviceStore(h.Cache).Get(context.Background(), started.FlowID)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(record.Generation, "admitted:"))
	m, ok := h.DB.(*mockStore)
	require.True(t, ok)
	m.deviceTxMu.Lock()
	require.Len(t, m.deviceRows, 1)
	clear(m.deviceRows) // An operator's deletion must never reauthorize this login.
	m.deviceTxMu.Unlock()
	inserts := 0
	m.createCredentialFn = func(_ context.Context, arg db.CreateCredentialParams) (db.CredentialTable, error) {
		inserts++
		return db.CredentialTable{CredentialID: arg.CredentialID, CredentialType: arg.CredentialType}, nil
	}
	_, err = h.SaveOpenAISubscriptionCredentialForFlow(context.Background(), &record, "OpenAI Subscription", validOpenAITokenBundle(), OpenAISubscriptionCredentialInfo{})
	require.Error(t, err)
	assert.Zero(t, inserts)
	assert.Empty(t, m.deviceRows)
	assert.Equal(t, 1, server.pollCalls())
	assert.Equal(t, 1, server.tokenCalls())
}

func TestDeviceFenceCommitUncertaintyReconcilesWithoutReplay(t *testing.T) {
	for _, committed := range []bool{true, false} {
		t.Run(fmt.Sprint(committed), func(t *testing.T) {
			server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{pollSuccess: true})
			h := newDeviceAuthHandlers(t, server)
			started, err := h.StartOpenAIDeviceAuth(context.Background(), "", "session-commit")
			require.NoError(t, err)
			makeDeviceFlowEligible(t, h.Cache, started.FlowID)
			m, ok := h.DB.(*mockStore)
			require.True(t, ok)
			inserts := 0
			create := m.createCredentialFn
			m.createCredentialFn = func(ctx context.Context, arg db.CreateCredentialParams) (db.CredentialTable, error) {
				inserts++
				return create(ctx, arg)
			}
			m.beginTxFn = func(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
				tx, err := m.beginDeviceMockTx(ctx, opts)
				if err != nil {
					return nil, err
				}
				mockTx, txOK := tx.(*deviceMockTx)
				require.True(t, txOK)
				return &deviceCommitUncertainTx{deviceMockTx: mockTx, committed: committed}, nil
			}
			for range 2 {
				status, err := h.PollOpenAIDeviceAuth(context.Background(), started.FlowID, "session-commit")
				require.NoError(t, err)
				if committed {
					assert.Equal(t, openaioauth.DeviceAuthStatusSuccess, status.Status)
				} else {
					assert.Equal(t, openaioauth.DeviceAuthStatusFailed, status.Status)
				}
			}
			if committed {
				assert.Len(t, m.deviceRows, 1)
			} else {
				assert.Zero(t, len(m.deviceRows))
			}
			assert.Equal(t, 1, inserts)
			assert.Equal(t, 1, server.pollCalls())
			assert.Equal(t, 1, server.tokenCalls())
		})
	}
}

type deviceCommitUncertainTx struct {
	*deviceMockTx
	committed bool
}

func (tx *deviceCommitUncertainTx) Commit(ctx context.Context) error {
	if tx.pending != nil {
		tx.commitErr, tx.rollbackCommit = context.DeadlineExceeded, !tx.committed
	}
	return tx.deviceMockTx.Commit(ctx)
}

func TestDeviceFenceTransactionLossDuringAdmissionCannotReopen(t *testing.T) {
	server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{pollSuccess: true})
	h := newDeviceAuthHandlers(t, server)
	c := &deviceFenceCASCache{deviceTestSharedMemoryCache: deviceTestSharedMemoryCache{cache.NewMemoryCache()}}
	h.Cache = c
	started, err := h.StartOpenAIDeviceAuth(context.Background(), "", "session-lost-tx")
	require.NoError(t, err)
	makeDeviceFlowEligible(t, c, started.FlowID)
	m, ok := h.DB.(*mockStore)
	require.True(t, ok)
	var active atomic.Pointer[deviceMockTx]
	m.beginTxFn = func(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
		tx, beginErr := m.beginDeviceMockTx(ctx, opts)
		if beginErr == nil {
			mockTx, txOK := tx.(*deviceMockTx)
			require.True(t, txOK)
			active.Store(mockTx)
		}
		return tx, beginErr
	}
	inserts := 0
	m.createCredentialFn = func(_ context.Context, arg db.CreateCredentialParams) (db.CredentialTable, error) {
		inserts++
		return db.CredentialTable{CredentialID: arg.CredentialID, CredentialType: arg.CredentialType}, nil
	}
	var original openaioauth.DeviceAuthRecord
	c.beforeCAS = func(before, after openaioauth.DeviceAuthRecord) {
		if before.Generation != after.Generation && strings.HasPrefix(after.Generation, "admitted:") {
			original = before
			require.NoError(t, active.Load().Rollback(context.Background()))
		}
	}
	status, err := h.PollOpenAIDeviceAuth(context.Background(), started.FlowID, "session-lost-tx")
	require.NoError(t, err)
	assert.Equal(t, openaioauth.DeviceAuthStatusFailed, status.Status)
	require.NotEmpty(t, original.Generation)
	_, err = h.SaveOpenAISubscriptionCredentialForFlow(context.Background(), &original, "OpenAI Subscription", validOpenAITokenBundle(), OpenAISubscriptionCredentialInfo{})
	require.Error(t, err)
	assert.Zero(t, len(m.deviceRows))
	assert.Zero(t, inserts)
	assert.Equal(t, 1, server.pollCalls())
	assert.Equal(t, 1, server.tokenCalls())
}

func TestDeviceFenceDelayedRevokerAfterTransactionLossCannotRebind(t *testing.T) {
	ctx := context.Background()
	server := newDeviceAuthTestServer(t, deviceAuthTestServerOptions{pollSuccess: true})
	h := newDeviceAuthHandlers(t, server)
	c := &deviceFenceCASCache{deviceTestSharedMemoryCache: deviceTestSharedMemoryCache{cache.NewMemoryCache()}}
	h.Cache = c
	started, err := h.StartOpenAIDeviceAuth(ctx, "", "session-delayed-revoker")
	require.NoError(t, err)
	makeDeviceFlowEligible(t, c, started.FlowID)
	m, ok := h.DB.(*mockStore)
	require.True(t, ok)
	var active atomic.Pointer[deviceMockTx]
	m.beginTxFn = func(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
		tx, beginErr := m.beginDeviceMockTx(ctx, opts)
		if beginErr == nil {
			mockTx, txOK := tx.(*deviceMockTx)
			require.True(t, txOK)
			active.Store(mockTx)
		}
		return tx, beginErr
	}
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	base := server.Client().Transport
	h.OpenAIOAuthHTTPClient = &http.Client{Transport: requestContextTransport(func(req *http.Request) (*http.Response, error) {
		resp, roundTripErr := base.RoundTrip(req)
		if roundTripErr == nil && req.URL.Path == "/oauth/token" {
			resp.Body = &durableFenceResponseBarrier{resp.Body, entered, release}
		}
		return resp, roundTripErr
	})}
	done := make(chan error, 1)
	go func() {
		_, pollErr := h.PollOpenAIDeviceAuth(ctx, started.FlowID, "session-delayed-revoker")
		done <- pollErr
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("exchange barrier not reached")
	}
	store := openaioauth.NewDeviceStore(c)
	original, err := store.Get(ctx, started.FlowID)
	require.NoError(t, err)
	revokerEntered, revokerRelease := make(chan struct{}), make(chan struct{})
	var revokerOnce sync.Once
	releaseRevoker := func() { revokerOnce.Do(func() { close(revokerRelease) }) }
	defer releaseRevoker()
	c.beforeCAS = func(before, after openaioauth.DeviceAuthRecord) {
		if after.Status == openaioauth.DeviceAuthStatusFailed {
			require.NoError(t, active.Load().Rollback(ctx))
			close(revokerEntered)
			<-revokerRelease
		}
	}
	inserted, finishInsert := make(chan struct{}), make(chan struct{})
	var insertOnce sync.Once
	unblockInsert := func() { insertOnce.Do(func() { close(finishInsert) }) }
	defer unblockInsert()
	create := m.createCredentialFn
	m.createCredentialFn = func(ctx context.Context, arg db.CreateCredentialParams) (db.CredentialTable, error) {
		close(inserted)
		<-finishInsert
		return create(ctx, arg)
	}
	revoked := make(chan error, 1)
	go func() { copy := original; revoked <- h.failDeviceAuth(ctx, store, &copy, "save_failed") }()
	select {
	case <-revokerEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("revoker CAS barrier not reached")
	}
	unblock()
	select {
	case <-inserted:
	case <-time.After(5 * time.Second):
		t.Fatal("admitted INSERT barrier not reached")
	}
	admitted, err := store.Get(ctx, started.FlowID)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(admitted.Generation, "admitted:"))
	assert.Zero(t, len(m.deviceRows), "writer has not committed")
	releaseRevoker()
	revokerErr := <-revoked
	assert.ErrorIs(t, revokerErr, openaioauth.ErrDeviceAuthOwnership)
	stillAdmitted, err := store.Get(ctx, started.FlowID)
	require.NoError(t, err)
	assert.Equal(t, openaioauth.DeviceAuthStatusExchanging, stillAdmitted.Status)
	assert.Equal(t, admitted.Generation, stillAdmitted.Generation)
	unblockInsert()
	require.NoError(t, <-done)
	status, err := h.PollOpenAIDeviceAuth(ctx, started.FlowID, "session-delayed-revoker")
	require.NoError(t, err)
	assert.Equal(t, openaioauth.DeviceAuthStatusSuccess, status.Status)
	assert.Len(t, m.deviceRows, 1)
	assert.Equal(t, 1, server.pollCalls())
	assert.Equal(t, 1, server.tokenCalls())
}

type deviceTestSharedMemoryCache struct{ *cache.MemoryCache }

func (deviceTestSharedMemoryCache) SharedCoordinationAvailable() bool { return true }

type failingDeviceAuthCache struct {
	*cache.MemoryCache
	mu               sync.Mutex
	failTerminalSave bool
}

func (c *failingDeviceAuthCache) SharedCoordinationAvailable() bool { return true }

func (c *failingDeviceAuthCache) terminalSaveShouldFail(value []byte) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.failTerminalSave {
		return false
	}
	var record struct {
		Status openaioauth.DeviceAuthStatus `json:"status"`
	}
	if json.Unmarshal(value, &record) != nil || record.Status != openaioauth.DeviceAuthStatusSuccess {
		return false
	}
	c.failTerminalSave = false
	return true
}

func (c *failingDeviceAuthCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if c.terminalSaveShouldFail(value) {
		return errors.New("terminal state cache write failed")
	}
	return c.MemoryCache.Set(ctx, key, value, ttl)
}

func (c *failingDeviceAuthCache) CompareAndSet(ctx context.Context, key string, expected, value []byte, ttl time.Duration) (bool, error) {
	if c.terminalSaveShouldFail(value) {
		return false, errors.New("terminal state cache write failed")
	}
	return c.MemoryCache.CompareAndSet(ctx, key, expected, value, ttl)
}

func newDeviceAuthHandlers(t *testing.T, server *deviceAuthTestServer) *Handlers {
	t.Helper()
	m := newMockStore()
	m.createCredentialFn = func(_ context.Context, arg db.CreateCredentialParams) (db.CredentialTable, error) {
		return db.CredentialTable{CredentialID: arg.CredentialID, CredentialName: arg.CredentialName, CredentialType: arg.CredentialType, CredentialValue: arg.CredentialValue, CredentialInfo: arg.CredentialInfo, OrganizationID: arg.OrganizationID}, nil
	}
	h := mockHandlers(m)
	h.Cache = deviceTestSharedMemoryCache{MemoryCache: cache.NewMemoryCache()}
	h.Config.GeneralSettings.MasterKey = "test-master-key-32-bytes-long!!!"
	h.Config.GeneralSettings.OpenAIOAuth = config.OpenAIOAuthConfig{
		Enabled:   true,
		IssuerURL: server.URL(),
		TokenURL:  server.URL() + "/oauth/token",
		ClientID:  "client-test",
	}
	h.OpenAIOAuthHTTPClient = server.Client()
	return h
}

func makeDeviceFlowEligible(t *testing.T, c cache.Cache, flowID string) {
	t.Helper()
	store := openaioauth.NewDeviceStore(c)
	record, err := store.Get(context.Background(), flowID)
	require.NoError(t, err)
	record.NextPollAt = time.Now().UTC().Add(-time.Second)
	require.NoError(t, store.Save(context.Background(), record))
}

type deviceAuthTestServerOptions struct {
	pollSuccess   bool
	pollStatus    int
	pollBody      string
	blockPoll     bool
	omitAccountID bool
	mismatchPKCE  bool
}

type deviceAuthTestServer struct {
	t           *httptest.Server
	options     deviceAuthTestServerOptions
	mu          sync.Mutex
	pollCount   int
	tokenCount  int
	tokenForm   url.Values
	pollStarted chan struct{}
	releasePoll chan struct{}
}

type requestContextTransport func(*http.Request) (*http.Response, error)

func (f requestContextTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func newDeviceAuthTestServer(t *testing.T, options deviceAuthTestServerOptions) *deviceAuthTestServer {
	t.Helper()
	s := &deviceAuthTestServer{options: options, pollStarted: make(chan struct{}), releasePoll: make(chan struct{})}
	s.t = httptest.NewServer(http.HandlerFunc(s.handle))
	t.Cleanup(s.t.Close)
	return s
}

func (s *deviceAuthTestServer) URL() string          { return s.t.URL }
func (s *deviceAuthTestServer) Client() *http.Client { return s.t.Client() }
func (s *deviceAuthTestServer) pollCalls() int       { s.mu.Lock(); defer s.mu.Unlock(); return s.pollCount }
func (s *deviceAuthTestServer) tokenCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tokenCount
}

func (s *deviceAuthTestServer) tokenFormValues() url.Values {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tokenForm
}

func (s *deviceAuthTestServer) handle(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/accounts/deviceauth/usercode":
		writeDeviceJSON(w, http.StatusOK, map[string]any{"device_auth_id": "[REDACTED]", "user_code": "ABCD-EFGH", "interval": "5"})
	case "/api/accounts/deviceauth/token":
		s.mu.Lock()
		s.pollCount++
		pollNumber := s.pollCount
		s.mu.Unlock()
		if pollNumber == 1 {
			select {
			case <-s.pollStarted:
			default:
				close(s.pollStarted)
			}
		}
		if s.options.blockPoll {
			<-s.releasePoll
		}
		if s.options.pollSuccess {
			codeVerifier := "[REDACTED]"
			if s.options.mismatchPKCE {
				codeVerifier = "beta"
			}
			writeDeviceJSON(w, http.StatusOK, map[string]any{"authorization_code": "[REDACTED]", "code_challenge": openai.ChallengeFromVerifier(codeVerifier), "code_verifier": "[REDACTED]"})
			return
		}
		status := s.options.pollStatus
		if status == 0 {
			status = http.StatusForbidden
		}
		body := s.options.pollBody
		if body == "" {
			body = `{"error":"authorization_pending"}`
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	case "/oauth/token":
		body, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		s.tokenCount++
		s.tokenForm, _ = url.ParseQuery(string(body))
		s.mu.Unlock()
		accountID := "acct-123"
		if s.options.omitAccountID {
			accountID = ""
		}
		writeDeviceJSON(w, http.StatusOK, map[string]any{"access_token": "[REDACTED]", "refresh_token": "[REDACTED]", "expires_in": 3600, "scope": "openid profile email", "account_id": accountID, "email": "admin@example.com"})
	default:
		http.NotFound(w, r)
	}
}

func writeDeviceJSON(w http.ResponseWriter, status int, value map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
