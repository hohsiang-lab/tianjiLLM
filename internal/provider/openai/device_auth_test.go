package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestDeviceCode_UsesCodexJSONContract(t *testing.T) {
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/accounts/deviceauth/usercode", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&requestBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"device_auth_id":"[REDACTED]","user_code":"ABCD-EFGH","interval":"5"}`)
	}))
	defer server.Close()

	code, err := RequestDeviceCode(context.Background(), server.Client(), config.OpenAIOAuthConfig{
		IssuerURL: server.URL,
		ClientID:  "client-test",
	})

	require.NoError(t, err)
	assert.Equal(t, map[string]any{"client_id": "client-test"}, requestBody)
	assert.Equal(t, "[REDACTED]", code.DeviceAuthID)
	assert.Equal(t, "ABCD-EFGH", code.UserCode)
	assert.Equal(t, int64(5), int64(code.Interval.Seconds()))
	assert.Equal(t, server.URL+"/codex/device", code.VerificationURI)
	assert.Equal(t, server.URL+"/deviceauth/callback", code.RedirectURI)
	assert.Equal(t, server.URL+"/api/accounts/deviceauth/token", code.TokenPollURL)
}

func TestRequestDeviceCode_AcceptsUsercodeAliasAndNumericInterval(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"device_auth_id":"device-id","usercode":"CODE-1234","interval":7}`)
	}))
	defer server.Close()

	code, err := RequestDeviceCode(context.Background(), server.Client(), config.OpenAIOAuthConfig{IssuerURL: server.URL, ClientID: "client"})

	require.NoError(t, err)
	assert.Equal(t, "CODE-1234", code.UserCode)
	assert.Equal(t, int64(7), int64(code.Interval.Seconds()))
}

func TestResolveDeviceAuthEndpoints_RequiresTLSOutsideLoopback(t *testing.T) {
	_, err := ResolveDeviceAuthEndpoints(config.OpenAIOAuthConfig{IssuerURL: "http://auth.example.test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must use HTTPS")

	endpoints, err := ResolveDeviceAuthEndpoints(config.OpenAIOAuthConfig{IssuerURL: "http://127.0.0.1:8080"})
	require.NoError(t, err)
	assert.Equal(t, "http://127.0.0.1:8080/codex/device", endpoints.VerificationURL)
}

func TestRequestDeviceCode_ClassifiesTransportFailureAsTemporary(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("transport unavailable")
	})}
	_, err := RequestDeviceCode(context.Background(), client, config.OpenAIOAuthConfig{IssuerURL: "https://auth.example.test", ClientID: "client"})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDeviceAuthTemporary)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestPollDeviceCode_UsesJSONContractAndMapsCodexPending(t *testing.T) {
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/accounts/deviceauth/token", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&requestBody))
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	_, err := PollDeviceCode(context.Background(), server.Client(), config.OpenAIOAuthConfig{IssuerURL: server.URL}, DeviceCode{
		DeviceAuthID: "device-id",
		UserCode:     "ABCD-EFGH",
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDeviceAuthPending)
	assert.Equal(t, map[string]any{"device_auth_id": "device-id", "user_code": "ABCD-EFGH"}, requestBody)
}

func TestPollDeviceCode_Maps404ToPending(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	_, err := PollDeviceCode(context.Background(), server.Client(), config.OpenAIOAuthConfig{IssuerURL: server.URL}, DeviceCode{DeviceAuthID: "[REDACTED]", UserCode: "ABCD-EFGH"})

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDeviceAuthPending)
}

func TestPollDeviceCode_MapsRFC8628Errors(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want error
	}{
		{name: "pending", body: `{"error":"authorization_pending"}`, want: ErrDeviceAuthPending},
		{name: "slow_down", body: `{"error":"slow_down"}`, want: ErrDeviceAuthSlowDown},
		{name: "expired", body: `{"error":"expired_token"}`, want: ErrDeviceAuthExpired},
		{name: "denied", body: `{"error":"access_denied"}`, want: ErrDeviceAuthDenied},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()

			_, err := PollDeviceCode(context.Background(), server.Client(), config.OpenAIOAuthConfig{IssuerURL: server.URL}, DeviceCode{DeviceAuthID: "id", UserCode: "code"})

			require.Error(t, err)
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestPollDeviceCode_SuccessParsesAuthorizationCodeAndPKCE(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"authorization_code":"[REDACTED]","code_challenge":"challenge","code_verifier":"[REDACTED]"}`)
	}))
	defer server.Close()

	result, err := PollDeviceCode(context.Background(), server.Client(), config.OpenAIOAuthConfig{IssuerURL: server.URL}, DeviceCode{DeviceAuthID: "id", UserCode: "code"})

	require.NoError(t, err)
	assert.Equal(t, DeviceAuthPollResult{
		AuthorizationCode: "[REDACTED]",
		CodeChallenge:     "challenge",
		CodeVerifier:      "[REDACTED]",
	}, result)
}

func TestDeviceAuthErrors_DoNotIncludeProviderBody(t *testing.T) {
	const secretBody = `{"error":"invalid_request","access_token":"[REDACTED]","refresh_token":"[REDACTED]"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, secretBody)
	}))
	defer server.Close()

	_, err := PollDeviceCode(context.Background(), server.Client(), config.OpenAIOAuthConfig{IssuerURL: server.URL}, DeviceCode{DeviceAuthID: "id", UserCode: "code"})

	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrDeviceAuthPending))
	assert.NotContains(t, err.Error(), "[REDACTED]")
}

func TestRequestDeviceCode_MapsServerFailureWithoutBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"error":"unknown_provider_error","access_token":"[REDACTED]"}`)
	}))
	defer server.Close()

	_, err := RequestDeviceCode(context.Background(), server.Client(), config.OpenAIOAuthConfig{IssuerURL: server.URL})

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDeviceAuthTemporary)
	assert.NotContains(t, err.Error(), "unknown_provider_error")
	assert.NotContains(t, err.Error(), "[REDACTED]")
}

func TestResolveDeviceAuthEndpoints_RejectsUnsupportedScheme(t *testing.T) {
	_, err := ResolveDeviceAuthEndpoints(config.OpenAIOAuthConfig{IssuerURL: "ftp://provider.example"})
	require.Error(t, err)
}

func TestRequestDeviceCode_RejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, strings.Repeat("x", oauthMaxBodyBytes+1))
	}))
	defer server.Close()

	_, err := RequestDeviceCode(context.Background(), server.Client(), config.OpenAIOAuthConfig{IssuerURL: server.URL, ClientID: "client-test"})

	require.ErrorIs(t, err, ErrDeviceAuthProvider)
	assert.NotContains(t, err.Error(), "response too large")
}

func TestPollDeviceCode_BodyReadFailurePreservesStatusSemantics(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		want    error
		readErr error
		cause   error
	}{
		{name: "forbidden_pending", status: http.StatusForbidden, want: ErrDeviceAuthPending},
		{name: "not_found_pending", status: http.StatusNotFound, want: ErrDeviceAuthPending},
		{name: "too_many_requests_slow_down", status: http.StatusTooManyRequests, want: ErrDeviceAuthSlowDown},
		{name: "server_error_temporary", status: http.StatusBadGateway, want: ErrDeviceAuthTemporary},
		{name: "unreadable_success_terminal", status: http.StatusOK, want: ErrDeviceAuthProvider},
		{name: "cancelled_success_terminal", status: http.StatusOK, want: ErrDeviceAuthProvider, readErr: context.Canceled, cause: context.Canceled},
		{name: "deadline_success_terminal", status: http.StatusOK, want: ErrDeviceAuthProvider, readErr: context.DeadlineExceeded, cause: context.DeadlineExceeded},
		{name: "wrapped_cancelled_success_terminal", status: http.StatusOK, want: ErrDeviceAuthProvider, readErr: fmt.Errorf("private reader diagnostic: %w", context.Canceled), cause: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			readErr := tc.readErr
			if readErr == nil {
				readErr = io.ErrUnexpectedEOF
			}
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: tc.status,
					Body:       io.NopCloser(iotest.ErrReader(readErr)),
				}, nil
			})}

			_, err := PollDeviceCodeAt(context.Background(), client, "https://auth.example.test/api/accounts/deviceauth/token", DeviceCode{
				DeviceAuthID: "device-id",
				UserCode:     "ABCD-EFGH",
			})

			require.ErrorIs(t, err, tc.want)
			if tc.cause != nil {
				assert.ErrorIs(t, err, tc.cause)
			} else {
				assert.NotErrorIs(t, err, context.Canceled)
				assert.NotErrorIs(t, err, context.DeadlineExceeded)
			}
			var responseErr *DeviceAuthError
			require.ErrorAs(t, err, &responseErr)
			assert.Equal(t, tc.want, responseErr.Kind)
			assert.Equal(t, tc.status, responseErr.StatusCode)
			if tc.status == http.StatusOK {
				assert.NotErrorIs(t, err, ErrDeviceAuthTemporary)
			}
			assert.NotContains(t, err.Error(), "private reader diagnostic")
		})
	}
}

func TestRequestDeviceCode_NotFoundIsUnavailable(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	_, err := RequestDeviceCode(context.Background(), server.Client(), config.OpenAIOAuthConfig{IssuerURL: server.URL})

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDeviceAuthUnavailable)
}

func TestPollDeviceCode_TransportFailureIsTerminalAndSafe(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cause error
	}{
		{"unexpected_eof", io.ErrUnexpectedEOF},
		{"unknown", errors.New("network unavailable")},
		{"cancelled", context.Canceled},
		{"deadline", context.DeadlineExceeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			transportErr := fmt.Errorf("private transport diagnostic: %w", tc.cause)
			handled := 0
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				handled++
				return nil, transportErr
			})}

			_, err := PollDeviceCode(context.Background(), client, config.OpenAIOAuthConfig{IssuerURL: "https://auth.example.test"}, DeviceCode{DeviceAuthID: "device-id", UserCode: "ABCD-EFGH"})

			require.Error(t, err)
			assert.Equal(t, 1, handled)
			assert.True(t, errors.Is(err, ErrDeviceAuthProvider), "ambiguous polls must be terminal provider errors")
			assert.False(t, errors.Is(err, ErrDeviceAuthTemporary), "ambiguous polls must not be retryable")
			assert.False(t, errors.Is(err, transportErr), "raw transport error chain must be dropped")
			wantMessage := ErrDeviceAuthProvider.Error()
			if tc.cause == context.Canceled || tc.cause == context.DeadlineExceeded {
				assert.ErrorIs(t, err, tc.cause)
				wantMessage += "\n" + tc.cause.Error()
			} else {
				assert.False(t, errors.Is(err, tc.cause), "arbitrary transport cause must be dropped")
				assert.NotErrorIs(t, err, context.Canceled)
				assert.NotErrorIs(t, err, context.DeadlineExceeded)
			}
			assert.True(t, err.Error() == wantMessage, "error text must contain only safe categories")
			var providerErr *DeviceAuthError
			require.True(t, errors.As(err, &providerErr), "poll failure must retain the provider error type")
			assert.Equal(t, ErrDeviceAuthProvider, providerErr.Kind)
			assert.Zero(t, providerErr.StatusCode)
		})
	}
}

func TestPollDeviceCodeAtRejectsUnsafeEndpointBeforeRequest(t *testing.T) {
	called := false
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		called = true
		return nil, errors.New("request must not be sent")
	})}

	_, err := PollDeviceCodeAt(context.Background(), client, "http://auth.example.test/api/accounts/deviceauth/token", DeviceCode{
		DeviceAuthID: "device-id",
		UserCode:     "user-code",
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDeviceAuthProvider)
	assert.False(t, called)
}
