//go:build e2e

package e2e

import (
	"os"
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
)

// HO-72: Verify that an ErrorLog-only record (no corresponding SpendLogs row)
// appears in the Request Logs table with Status "Failed".

func TestLogsList_ErrorLogOnly(t *testing.T) {
	f := setup(t)

	// Seed ONLY an ErrorLog — no matching SpendLog row.
	// Use a short, recognizable prefix that fits within the UI's truncated display.
	reqID := "req-erronly-ho72"
	f.SeedErrorLog(reqID, "openai/gpt-4o", "AuthenticationError", 401)

	f.NavigateToLogs()
	time.Sleep(500 * time.Millisecond)

	// The ErrorLog-only row should appear in the table.
	rows := f.Count("#logs-table table tbody tr")
	assert.GreaterOrEqual(t, rows, 1, "Expected at least 1 row for ErrorLog-only record")

	body := f.Text("#logs-table")
	// The UI truncates long request IDs; assert on the visible prefix.
	assert.Contains(t, body, "req-erronly-", "ErrorLog-only request ID prefix should be visible in the table")
	assert.Contains(t, body, "Failed", "ErrorLog-only record should display Status = Failed")

	// Screenshot for CI artifact.
	os.MkdirAll("/tmp/screenshots", 0o755)
	f.Page.Screenshot(playwright.PageScreenshotOptions{
		Path:     playwright.String("/tmp/screenshots/ho72-error-log-only.png"),
		FullPage: playwright.Bool(true),
	})
	t.Log("Screenshot saved to /tmp/screenshots/ho72-error-log-only.png")
}
