package callback

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/praxisllmlab/tianjiLLM/internal/security/redact"
)

const DiscordUsageReportContentLimit = 2000

type DiscordUsageReportDeliveryOutcome struct {
	AttemptedParts int
	DeliveredParts int
	Skipped        bool
	FailureReason  string
}

type DiscordUsageReportSender struct {
	webhookURL string
	client     *http.Client
}

func NewDiscordUsageReportSender(webhookURL string) *DiscordUsageReportSender {
	return &DiscordUsageReportSender{webhookURL: strings.TrimSpace(webhookURL)}
}

func (s *DiscordUsageReportSender) Send(ctx context.Context, parts []string) DiscordUsageReportDeliveryOutcome {
	var outcome DiscordUsageReportDeliveryOutcome
	if s == nil || s.webhookURL == "" {
		outcome.Skipped = true
		return outcome
	}
	endpoint, err := url.Parse(s.webhookURL)
	if err != nil || endpoint.Scheme == "" || endpoint.Host == "" {
		outcome.FailureReason = "webhook_invalid"
		return outcome
	}
	query := endpoint.Query()
	query.Set("wait", "true")
	endpoint.RawQuery = query.Encode()

	for _, part := range parts {
		content := redact.String(part)
		if utf8.RuneCountInString(content) > DiscordUsageReportContentLimit {
			outcome.FailureReason = "content_too_long"
			return outcome
		}
		outcome.AttemptedParts++
		if reason := s.sendPart(ctx, endpoint.String(), content); reason != "" {
			outcome.FailureReason = reason
			return outcome
		}
		outcome.DeliveredParts++
	}
	return outcome
}

func (s *DiscordUsageReportSender) sendPart(ctx context.Context, endpoint, content string) string {
	status, retryAfter, reason := s.postPart(ctx, endpoint, content)
	if reason != "" {
		return reason
	}
	if status >= http.StatusOK && status < http.StatusMultipleChoices {
		return ""
	}
	if status != http.StatusTooManyRequests || !discordUsageReportRetryAllowed(ctx, retryAfter) {
		return discordUsageReportStatusReason(status)
	}

	timer := time.NewTimer(time.Duration(retryAfter * float64(time.Second)))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return "context_cancelled"
	case <-timer.C:
	}

	status, _, reason = s.postPart(ctx, endpoint, content)
	if reason != "" {
		return reason
	}
	if status >= http.StatusOK && status < http.StatusMultipleChoices {
		return ""
	}
	return discordUsageReportStatusReason(status)
}

func (s *DiscordUsageReportSender) postPart(ctx context.Context, endpoint, content string) (int, float64, string) {
	payload, err := json.Marshal(struct {
		Content         string `json:"content"`
		AllowedMentions struct {
			Parse []string `json:"parse"`
		} `json:"allowed_mentions"`
	}{
		Content: content,
		AllowedMentions: struct {
			Parse []string `json:"parse"`
		}{Parse: []string{}},
	})
	if err != nil {
		return 0, 0, "payload_encode_failed"
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return 0, 0, "request_create_failed"
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := s.httpClient().Do(request)
	if err != nil {
		return 0, 0, "transport_failed"
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusTooManyRequests {
		return response.StatusCode, 0, ""
	}

	var body struct {
		RetryAfter float64 `json:"retry_after"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&body); err != nil {
		return response.StatusCode, 0, ""
	}
	return response.StatusCode, body.RetryAfter, ""
}

func (s *DiscordUsageReportSender) httpClient() *http.Client {
	if s != nil && s.client != nil {
		return s.client
	}
	return http.DefaultClient
}

func discordUsageReportRetryAllowed(ctx context.Context, retryAfter float64) bool {
	if retryAfter < 0 {
		return false
	}
	deadline, ok := ctx.Deadline()
	return ok && retryAfter < time.Until(deadline).Seconds()
}

func discordUsageReportStatusReason(status int) string {
	if status == http.StatusTooManyRequests {
		return "rate_limited"
	}
	if status >= 300 && status <= 599 {
		return "http_status_" + strconv.Itoa(status)
	}
	return "http_status_unknown"
}
