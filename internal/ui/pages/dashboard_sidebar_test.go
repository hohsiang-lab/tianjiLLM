package pages_test

import (
	"strings"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/ui/pages"
)

func TestDashboardIncludesDialogRuntimeForMobileSidebar(t *testing.T) {
	html := renderToString(t, pages.DashboardPage(pages.DashboardStats{}))

	if !strings.Contains(html, `data-tui-dialog-trigger="app-sidebar-mobile"`) {
		t.Fatal("dashboard is missing the mobile sidebar dialog trigger")
	}
	if count := strings.Count(html, `src="/ui/static/js/dialog.min.js`); count != 1 {
		t.Fatalf("dashboard must load dialog runtime exactly once, got %d", count)
	}
}
