package pages_test

import (
	"os"
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

func TestMobileSidebarUsesCompiledSidebarBackground(t *testing.T) {
	html := renderToString(t, pages.DashboardPage(pages.DashboardStats{}))
	dialogContent := strings.Index(html, `data-tui-dialog-content`)
	if dialogContent < 0 {
		t.Fatal("dashboard is missing the mobile sidebar dialog content")
	}
	classStart := strings.LastIndex(html[:dialogContent], `class="`)
	if classStart < 0 {
		t.Fatal("mobile sidebar dialog is missing its class attribute")
	}
	classStart += len(`class="`)
	classEnd := strings.Index(html[classStart:dialogContent], `"`)
	if classEnd < 0 {
		t.Fatal("mobile sidebar dialog class attribute is not closed")
	}
	dialogClass := html[classStart : classStart+classEnd]
	if !strings.Contains(dialogClass, `md:hidden`) || !strings.Contains(dialogClass, `bg-sidebar-background`) {
		t.Fatal("mobile sidebar dialog is missing the themed background utility")
	}

	css, err := os.ReadFile("../assets/css/output.css")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(css), `.bg-sidebar-background{background-color:#fafafa}`) {
		t.Fatal("compiled UI CSS is missing the sidebar background utility")
	}
}
