package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
)

const deviceAuthDefaultInterval = 5 * time.Second

var (
	ErrDeviceAuthPending     = errors.New("device authorization pending")
	ErrDeviceAuthSlowDown    = errors.New("device authorization polling slowed down")
	ErrDeviceAuthExpired     = errors.New("device authorization expired")
	ErrDeviceAuthDenied      = errors.New("device authorization denied")
	ErrDeviceAuthUnavailable = errors.New("device authorization unavailable")
	ErrDeviceAuthTemporary   = errors.New("device authorization temporarily unavailable")
	ErrDeviceAuthProvider    = errors.New("device authorization provider error")
)

// DeviceAuthError preserves only a safe category and HTTP status from a provider response.
type DeviceAuthError struct {
	Kind       error
	StatusCode int
}

func (e *DeviceAuthError) Error() string {
	if e == nil || e.Kind == nil {
		return "openai device authorization failed"
	}
	if e.StatusCode == 0 {
		return e.Kind.Error()
	}
	return fmt.Sprintf("%s (%d)", e.Kind, e.StatusCode)
}

func (e *DeviceAuthError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Kind
}

type DeviceAuthEndpoints struct {
	UserCodeURL      string
	TokenPollURL     string
	VerificationURL  string
	TokenRedirectURI string
}

// DeviceCode is the server-side data returned by OpenAI's Codex device flow.
// DeviceAuthID must never be sent to the browser.
type DeviceCode struct {
	DeviceAuthID    string
	UserCode        string
	Interval        time.Duration
	VerificationURI string
	RedirectURI     string
	TokenPollURL    string
}

type DeviceAuthPollResult struct {
	AuthorizationCode string `json:"authorization_code"`
	CodeChallenge     string `json:"code_challenge"`
	CodeVerifier      string `json:"code_verifier"`
}

func isLoopbackDeviceIssuer(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func ResolveDeviceAuthEndpoints(input config.OpenAIOAuthConfig) (DeviceAuthEndpoints, error) {
	cfg := config.ResolveOpenAIOAuthConfig(input)
	issuer := strings.TrimRight(strings.TrimSpace(cfg.IssuerURL), "/")
	if err := validateDeviceAuthEndpointURL(issuer); err != nil {
		return DeviceAuthEndpoints{}, err
	}
	return DeviceAuthEndpoints{
		UserCodeURL:      issuer + "/api/accounts/deviceauth/usercode",
		TokenPollURL:     issuer + "/api/accounts/deviceauth/token",
		VerificationURL:  issuer + "/codex/device",
		TokenRedirectURI: issuer + "/deviceauth/callback",
	}, nil
}

func validateDeviceAuthEndpointURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return errors.New("openai device auth endpoint URL must be an absolute http(s) URL")
	}
	if parsed.Scheme == "http" && !isLoopbackDeviceIssuer(parsed.Hostname()) {
		return errors.New("openai device auth endpoint URL must use HTTPS")
	}
	return nil
}

func RequestDeviceCode(ctx context.Context, client *http.Client, input config.OpenAIOAuthConfig) (DeviceCode, error) {
	endpoints, err := ResolveDeviceAuthEndpoints(input)
	if err != nil {
		return DeviceCode{}, err
	}
	if client == nil {
		client = http.DefaultClient
	}
	body, err := json.Marshal(struct {
		ClientID string `json:"client_id"`
	}{ClientID: config.ResolveOpenAIOAuthConfig(input).ClientID})
	if err != nil {
		return DeviceCode{}, fmt.Errorf("marshal openai device auth request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoints.UserCodeURL, strings.NewReader(string(body)))
	if err != nil {
		return DeviceCode{}, fmt.Errorf("create openai device auth request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := client.Do(request)
	if err != nil {
		return DeviceCode{}, errors.Join(ErrDeviceAuthTemporary, err)
	}
	defer response.Body.Close()
	responseBody, err := readOAuthResponseBody(response.Body)
	if err != nil {
		return DeviceCode{}, deviceAuthResponseError(response.StatusCode, false, err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return DeviceCode{}, requestDeviceAuthError(response.StatusCode, responseBody)
	}

	var payload struct {
		DeviceAuthID string          `json:"device_auth_id"`
		UserCode     string          `json:"user_code"`
		Usercode     string          `json:"usercode"`
		Interval     json.RawMessage `json:"interval"`
	}
	if unmarshalErr := json.Unmarshal(responseBody, &payload); unmarshalErr != nil {
		return DeviceCode{}, &DeviceAuthError{Kind: ErrDeviceAuthProvider, StatusCode: response.StatusCode}
	}
	if payload.UserCode == "" {
		payload.UserCode = payload.Usercode
	}
	if strings.TrimSpace(payload.DeviceAuthID) == "" || strings.TrimSpace(payload.UserCode) == "" {
		return DeviceCode{}, &DeviceAuthError{Kind: ErrDeviceAuthProvider, StatusCode: response.StatusCode}
	}
	interval, err := parseDeviceAuthInterval(payload.Interval)
	if err != nil {
		return DeviceCode{}, &DeviceAuthError{Kind: ErrDeviceAuthProvider, StatusCode: response.StatusCode}
	}
	return DeviceCode{
		DeviceAuthID:    payload.DeviceAuthID,
		UserCode:        payload.UserCode,
		Interval:        interval,
		VerificationURI: endpoints.VerificationURL,
		RedirectURI:     endpoints.TokenRedirectURI,
		TokenPollURL:    endpoints.TokenPollURL,
	}, nil
}

func PollDeviceCode(ctx context.Context, client *http.Client, input config.OpenAIOAuthConfig, device DeviceCode) (DeviceAuthPollResult, error) {
	endpoints, err := ResolveDeviceAuthEndpoints(input)
	if err != nil {
		return DeviceAuthPollResult{}, err
	}
	return PollDeviceCodeAt(ctx, client, endpoints.TokenPollURL, device)
}

func PollDeviceCodeAt(ctx context.Context, client *http.Client, tokenPollURL string, device DeviceCode) (DeviceAuthPollResult, error) {
	if err := validateDeviceAuthEndpointURL(tokenPollURL); err != nil {
		return DeviceAuthPollResult{}, &DeviceAuthError{Kind: ErrDeviceAuthProvider}
	}
	if strings.TrimSpace(device.DeviceAuthID) == "" || strings.TrimSpace(device.UserCode) == "" {
		return DeviceAuthPollResult{}, &DeviceAuthError{Kind: ErrDeviceAuthProvider}
	}
	if client == nil {
		client = http.DefaultClient
	}
	body, err := json.Marshal(struct {
		DeviceAuthID string `json:"device_auth_id"`
		UserCode     string `json:"user_code"`
	}{DeviceAuthID: device.DeviceAuthID, UserCode: device.UserCode})
	if err != nil {
		return DeviceAuthPollResult{}, fmt.Errorf("marshal openai device auth poll: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenPollURL, strings.NewReader(string(body)))
	if err != nil {
		return DeviceAuthPollResult{}, fmt.Errorf("create openai device auth poll: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := client.Do(request)
	if err != nil {
		return DeviceAuthPollResult{}, deviceAuthResponseError(0, true, err)
	}
	defer response.Body.Close()
	responseBody, err := readOAuthResponseBody(response.Body)
	if err != nil {
		return DeviceAuthPollResult{}, deviceAuthResponseError(response.StatusCode, true, err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return DeviceAuthPollResult{}, pollDeviceAuthError(response.StatusCode, responseBody)
	}

	var payload DeviceAuthPollResult
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return DeviceAuthPollResult{}, &DeviceAuthError{Kind: ErrDeviceAuthProvider, StatusCode: response.StatusCode}
	}
	if strings.TrimSpace(payload.AuthorizationCode) == "" || strings.TrimSpace(payload.CodeChallenge) == "" || strings.TrimSpace(payload.CodeVerifier) == "" {
		return DeviceAuthPollResult{}, &DeviceAuthError{Kind: ErrDeviceAuthProvider, StatusCode: response.StatusCode}
	}
	return payload, nil
}

func parseDeviceAuthInterval(raw json.RawMessage) (time.Duration, error) {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return deviceAuthDefaultInterval, nil
	}
	var text string
	if raw[0] == '"' {
		if err := json.Unmarshal(raw, &text); err != nil {
			return 0, err
		}
	} else {
		text = strings.TrimSpace(string(raw))
	}
	seconds, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
	if err != nil || seconds < 0 || seconds > int64((time.Duration(1<<63-1)/time.Second)) {
		return 0, errors.New("invalid device auth interval")
	}
	if seconds == 0 {
		return deviceAuthDefaultInterval, nil
	}
	return time.Duration(seconds) * time.Second, nil
}

func deviceAuthResponseError(status int, poll bool, err error) error {
	if status == 0 || (status >= http.StatusOK && status < http.StatusMultipleChoices) {
		kind := ErrDeviceAuthTemporary
		// A poll may have consumed the one-time code even without a response.
		if poll || errors.Is(err, errOAuthBodyTooLarge) {
			kind = ErrDeviceAuthProvider
		}
		bodyErr := &DeviceAuthError{Kind: kind, StatusCode: status}
		if poll && errors.Is(err, context.Canceled) {
			return errors.Join(bodyErr, context.Canceled)
		}
		if poll && errors.Is(err, context.DeadlineExceeded) {
			return errors.Join(bodyErr, context.DeadlineExceeded)
		}
		return bodyErr
	}
	if poll {
		return pollDeviceAuthError(status, nil)
	}
	return requestDeviceAuthError(status, nil)
}

func requestDeviceAuthError(status int, body []byte) error {
	if code := deviceAuthErrorCode(body); code != "" {
		if mapped := mappedDeviceAuthError(code); mapped != nil {
			return &DeviceAuthError{Kind: mapped, StatusCode: status}
		}
	}
	kind := ErrDeviceAuthProvider
	switch {
	case status == http.StatusNotFound || status == http.StatusForbidden:
		kind = ErrDeviceAuthUnavailable
	case status == http.StatusTooManyRequests || status >= http.StatusInternalServerError:
		kind = ErrDeviceAuthTemporary
	}
	return &DeviceAuthError{Kind: kind, StatusCode: status}
}

func pollDeviceAuthError(status int, body []byte) error {
	if code := deviceAuthErrorCode(body); code != "" {
		if mapped := mappedDeviceAuthError(code); mapped != nil {
			return &DeviceAuthError{Kind: mapped, StatusCode: status}
		}
	}
	kind := ErrDeviceAuthProvider
	switch {
	case status == http.StatusForbidden || status == http.StatusNotFound:
		// This is the behavior used by the current Codex CLI implementation.
		kind = ErrDeviceAuthPending
	case status == http.StatusTooManyRequests:
		kind = ErrDeviceAuthSlowDown
	case status >= http.StatusInternalServerError:
		kind = ErrDeviceAuthTemporary
	}
	return &DeviceAuthError{Kind: kind, StatusCode: status}
}

func deviceAuthErrorCode(body []byte) string {
	var payload struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &payload) != nil {
		return ""
	}
	return strings.TrimSpace(payload.Error)
}

func mappedDeviceAuthError(code string) error {
	switch code {
	case "authorization_pending":
		return ErrDeviceAuthPending
	case "slow_down":
		return ErrDeviceAuthSlowDown
	case "expired_token":
		return ErrDeviceAuthExpired
	case "access_denied":
		return ErrDeviceAuthDenied
	default:
		return nil
	}
}
