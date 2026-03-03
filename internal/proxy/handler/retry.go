package handler

import (
	"context"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"
)

var retryableStatusCodes = map[int]bool{
	429: true,
	500: true,
	502: true,
	503: true,
	504: true,
}

// doUpstreamWithRetry executes an HTTP request with exponential backoff retry.
// buildReq must build a fresh request each time (Body can only be read once).
func doUpstreamWithRetry(ctx context.Context, client *http.Client, buildReq func() (*http.Request, error), maxRetries int) (*http.Response, error) {
	baseDelay := time.Second
	var lastResp *http.Response
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := buildReq()
		if err != nil {
			return nil, err
		}
		req = req.WithContext(ctx)

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxRetries {
				wait := baseDelay * (1 << attempt)
				log.Printf("[retry] attempt %d/%d error=%v waiting=%v", attempt+1, maxRetries, err, wait)
				time.Sleep(wait)
				continue
			}
			return nil, lastErr
		}

		if !retryableStatusCodes[resp.StatusCode] || attempt == maxRetries {
			return resp, nil
		}

		// Drain and close body before retry
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		lastResp = resp

		wait := baseDelay * (1 << attempt)
		// Respect Retry-After header on 429
		if resp.StatusCode == 429 {
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if secs, err := strconv.Atoi(ra); err == nil {
					wait = time.Duration(secs) * time.Second
				}
			}
		}
		log.Printf("[retry] attempt %d/%d status=%d waiting=%v", attempt+1, maxRetries, resp.StatusCode, wait)
		time.Sleep(wait)
	}
	return lastResp, nil
}
