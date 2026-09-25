package handler

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
)

type openAISubscriptionRequestBuilder func(openAISubscriptionProviderAttempt) (*http.Request, error)

func (h *Handlers) doOpenAISubscriptionProviderRequest(
	ctx context.Context,
	apiKey string,
	candidates []resolvedOpenAISubscriptionCredential,
	build openAISubscriptionRequestBuilder,
) (*http.Response, time.Duration, *callback.OpenAISubscriptionAttribution, error) {
	attempts := openAISubscriptionProviderAttempts(apiKey, candidates)
	var lastErr error
	var totalLatency time.Duration

	for i, attempt := range attempts {
		req, err := build(attempt)
		if err != nil {
			return nil, totalLatency, nil, err
		}

		start := time.Now()
		resp, err := http.DefaultClient.Do(req)
		totalLatency += time.Since(start)
		if err != nil {
			lastErr = err
			if i+1 < len(attempts) {
				continue
			}
			return nil, totalLatency, attempt.attribution(ctx, "failure", "request_failed"), err
		}

		if attempt.credentialID != "" && h != nil {
			if state, ok := parseOpenAIRateLimitHeaders(resp.Header, attempt.credentialID, h.openAISubscriptionNowUTC()); ok {
				h.recordOpenAISubscriptionRateLimit(attempt.credentialID, state)
			}
		}
		if resp.StatusCode == http.StatusUnauthorized && attempt.credentialID != "" && h != nil {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if isNonRefreshableOpenAISubscription401(body) {
				resp.Body = io.NopCloser(strings.NewReader(string(body)))
				return resp, totalLatency, attempt.attribution(ctx, "failure", openAISubscriptionHTTPReasonCode(resp.StatusCode)), nil
			}

			refreshed, refreshErr := h.forceRefreshOpenAISubscriptionCredential(ctx, attempt.credentialID)
			if refreshErr != nil {
				lastErr = refreshErr
				if i+1 < len(attempts) {
					continue
				}
				return openAISubscriptionReauthorizationRequiredResponse(req), totalLatency, attempt.attribution(ctx, "failure", string(openAISubscriptionErrorCode(refreshErr))), nil
			}

			retryReq, err := build(openAISubscriptionProviderAttempt{
				credentialID: refreshed.CredentialID,
				apiKey:       refreshed.BearerToken,
				accountID:    refreshed.AccountID,
			})
			if err != nil {
				return nil, totalLatency, nil, err
			}
			start = time.Now()
			retryResp, retryErr := http.DefaultClient.Do(retryReq)
			totalLatency += time.Since(start)
			if retryErr != nil {
				lastErr = retryErr
				if i+1 < len(attempts) {
					continue
				}
				return nil, totalLatency, attempt.attribution(ctx, "failure", "retry_failed"), retryErr
			}
			if state, ok := parseOpenAIRateLimitHeaders(retryResp.Header, attempt.credentialID, h.openAISubscriptionNowUTC()); ok {
				h.recordOpenAISubscriptionRateLimit(attempt.credentialID, state)
			}
			if retryResp.StatusCode == http.StatusUnauthorized && i+1 < len(attempts) {
				lastErr = h.recordOpenAISubscriptionAuthFailureAfterRefresh(ctx, attempt.credentialID, "upstream returned 401 after forced refresh")
				_, _ = io.Copy(io.Discard, retryResp.Body)
				_ = retryResp.Body.Close()
				continue
			}
			if retryResp.StatusCode == http.StatusUnauthorized {
				_ = h.recordOpenAISubscriptionAuthFailureAfterRefresh(ctx, attempt.credentialID, "upstream returned 401 after forced refresh")
				_, _ = io.Copy(io.Discard, retryResp.Body)
				_ = retryResp.Body.Close()
				return openAISubscriptionReauthorizationRequiredResponse(req), totalLatency, attempt.attribution(ctx, "failure", string(OpenAISubscriptionCredentialAuthErr)), nil
			}
			if openAISubscriptionRetryableStatus(retryResp.StatusCode) && i+1 < len(attempts) {
				_, _ = io.Copy(io.Discard, retryResp.Body)
				_ = retryResp.Body.Close()
				continue
			}
			status := "success"
			reasonCode := ""
			if retryResp.StatusCode >= http.StatusBadRequest {
				status = "failure"
				reasonCode = openAISubscriptionHTTPReasonCode(retryResp.StatusCode)
			}
			return retryResp, totalLatency, attempt.attribution(ctx, status, reasonCode), nil
		}
		if openAISubscriptionRetryableStatus(resp.StatusCode) && i+1 < len(attempts) {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			continue
		}
		status := "success"
		reasonCode := ""
		if resp.StatusCode >= http.StatusBadRequest {
			status = "failure"
			reasonCode = openAISubscriptionHTTPReasonCode(resp.StatusCode)
		}
		return resp, totalLatency, attempt.attribution(ctx, status, reasonCode), nil
	}

	return nil, totalLatency, nil, lastErr
}

func isNonRefreshableOpenAISubscription401(body []byte) bool {
	lower := strings.ToLower(string(body))
	return strings.Contains(lower, "missing") && strings.Contains(lower, "scope")
}

type openAISubscriptionProviderAttempt struct {
	credentialID string
	apiKey       string
	accountID    string
}

func openAISubscriptionProviderAttempts(apiKey string, candidates []resolvedOpenAISubscriptionCredential) []openAISubscriptionProviderAttempt {
	if len(candidates) == 0 {
		return []openAISubscriptionProviderAttempt{{apiKey: apiKey}}
	}
	attempts := make([]openAISubscriptionProviderAttempt, 0, len(candidates))
	for _, candidate := range candidates {
		attempts = append(attempts, openAISubscriptionProviderAttempt{
			credentialID: candidate.CredentialID,
			apiKey:       candidate.BearerToken,
			accountID:    candidate.AccountID,
		})
	}
	return attempts
}

func (a openAISubscriptionProviderAttempt) attribution(ctx context.Context, status, reasonCode string) *callback.OpenAISubscriptionAttribution {
	if a.credentialID == "" {
		return nil
	}
	attr := &callback.OpenAISubscriptionAttribution{
		CredentialID: a.credentialID,
		Provider:     "openai",
		Action:       "completion",
		Status:       status,
		ReasonCode:   reasonCode,
	}
	if ctx != nil {
		if orgID, ok := ctx.Value(middleware.ContextKeyOrgID).(string); ok {
			attr.OrganizationID = orgID
		}
	}
	return attr
}

func openAISubscriptionReauthorizationRequiredResponse(req *http.Request) *http.Response {
	body := `{"error":{"message":"OpenAI subscription reauthorization required: all configured credentials failed authentication or refresh","type":"authentication_error","code":"openai_subscription_reauthorization_required"}}`
	return &http.Response{
		StatusCode: http.StatusUnauthorized,
		Status:     "401 Unauthorized",
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}
