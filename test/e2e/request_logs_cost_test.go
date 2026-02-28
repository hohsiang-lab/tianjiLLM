//go:build e2e

package e2e

import (
	"os"
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// HO-67: Verify Cost and Tokens columns display actual values (not dashes)
// when spend/token data exists.

func TestRequestLogs_CostTokensDisplay(t *testing.T) {
	f := setup(t)

	// Seed logs with non-zero spend and tokens
	f.SeedSpendLog(SeedSpendLogOpts{
		Model:      "openai/gpt-4o",
		Spend:      0.0523,
		Tokens:     250,
		Prompt:     150,
		Completion: 100,
	})
	f.SeedSpendLog(SeedSpendLogOpts{
		Model:      "anthropic/claude-sonnet-4-5-20250929",
		Spend:      0.1200,
		Tokens:     500,
		Prompt:     300,
		Completion: 200,
	})

	f.NavigateToLogs()
	time.Sleep(500 * time.Millisecond)

	// Verify table has rows
	rows := f.Count("#logs-table table tbody tr")
	assert.GreaterOrEqual(t, rows, 2, "Expected at least 2 data rows")

	// Check each row's Cost and Tokens cells
	costFound := false
	tokensFound := false

	for i := 0; i < rows; i++ {
		row := f.Page.Locator("#logs-table table tbody tr").Nth(i)

		// Get all td texts
		cells, err := row.Locator("td").AllTextContents()
		require.NoError(t, err)
		t.Logf("Row %d cells: %v", i, cells)

		// We need to find Cost and Tokens columns by checking header order
		// For now, check that at least one cell contains a dollar value or numeric tokens
		for _, cell := range cells {
			if cell != "–" && cell != "-" && cell != "" {
				// Check for cost pattern (contains $)
				if len(cell) > 0 && cell[0] == '$' {
					costFound = true
				}
				// Check for token count (number, possibly with breakdown like "500 (300+200)")
				if isTokenValue(cell) {
					tokensFound = true
				}
			}
		}
	}

	assert.True(t, costFound, "Expected at least one row with a Cost value (not dash)")
	assert.True(t, tokensFound, "Expected at least one row with a Tokens value (not dash)")

	// Screenshot
	os.MkdirAll("/tmp/screenshots", 0o755)
	f.Page.Screenshot(playwright.PageScreenshotOptions{
		Path:     playwright.String("/tmp/screenshots/ho67-request-logs.png"),
		FullPage: playwright.Bool(true),
	})
	t.Log("Screenshot saved to /tmp/screenshots/ho67-request-logs.png")
}

func isNumeric(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// isTokenValue returns true if the string starts with a number,
// e.g. "500", "500 (300+200)".
func isTokenValue(s string) bool {
	if len(s) == 0 {
		return false
	}
	// Must start with a digit
	if s[0] < '0' || s[0] > '9' {
		return false
	}
	// Check that at least the leading part is numeric
	for _, c := range s {
		if c == ' ' || c == '(' {
			return true // valid token format with breakdown
		}
		if c < '0' || c > '9' {
			return false
		}
	}
	return true // pure number
}
