package handler

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"
)

var retryableStatusCodes = map[int]bool{
	http.StatusTooManyRequests:     true, // 429
	http.StatusInternalServerError: true, // 500
	http.StatusBadGateway:         true, // 502
	http.StatusServiceUnavailable: true, // 503
	http.StatusGatewayTimeout:     true, // 504
}

const retryBaseDelay = time.Second

// doUpstreamWithRetry executes the request built by buildReq, retrying on
// retryable status codes up to maxRetries times with exponential backoff.
// buildReq is called on every attempt so that the request body can be re-read.
// On exhausting retries the last *http.Response is returned (caller must close body).
func doUpstreamWithRetry(ctx context.Context, client *http.Client, buildReq func() (*http.Request, error), maxRetries int) (*http.Response, error) {
	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := buildReq()
		if err != nil {
			return nil, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}

		if !retryableStatusCodes[resp.StatusCode] || attempt == maxRetries {
			return resp, nil
		}

		// Close body before retry to avoid leaking connections.
		resp.Body.Close()

		// Determine wait duration.
		waitSecs := 1 << attempt // 1, 2, 4 ...
		if resp.StatusCode == http.StatusTooManyRequests {
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if secs, parseErr := strconv.Atoi(ra); parseErr == nil {
					waitSecs = secs
				}
			}
		}

		log.Printf("[retry] attempt %d/%d status=%d waiting=%ds", attempt+1, maxRetries, resp.StatusCode, waitSecs)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(waitSecs) * time.Second):
		}
	}

	// Unreachable, but satisfies compiler.
	return nil, nil
}
