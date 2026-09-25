package pages_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/a-h/templ"
)

// renderToString renders a templ component to a plain HTML string for inspection.
func renderToString(t *testing.T, c templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	if err := c.Render(context.Background(), &buf); err != nil {
		t.Fatalf("failed to render component: %v", err)
	}
	return buf.String()
}
