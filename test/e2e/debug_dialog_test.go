//go:build e2e

package e2e

import (
	"testing"
	"github.com/playwright-community/playwright-go"
)

func TestDebugDialog(t *testing.T) {
	f := setup(t)
	f.NavigateToModels()
	f.ClickButton("Add Model")
	f.WaitDialogOpen("create-model-dialog")

	// Debug: check if button exists
	count, err := f.Page.Locator("#create-model-dialog button[type=submit]").Count()
	t.Logf("submit button count: %d, err: %v", count, err)

	// Check all buttons in dialog
	allBtns, _ := f.Page.Locator("#create-model-dialog button").Count()
	t.Logf("total buttons in #create-model-dialog: %d", allBtns)

	// Try innerHTML
	html, err := f.Page.Locator("#create-model-dialog").InnerHTML()
	if err != nil {
		t.Logf("innerHTML err: %v", err)
	} else if len(html) > 2000 {
		t.Logf("innerHTML (first 2000): %s", html[:2000])
	} else {
		t.Logf("innerHTML: %s", html)
	}

	// Try force click
	err = f.Page.Locator("#create-model-dialog button[type=submit]").Filter(playwright.LocatorFilterOptions{
		HasText: "Create",
	}).Click(playwright.LocatorClickOptions{Force: playwright.Bool(true)})
	t.Logf("force click result: %v", err)
}
