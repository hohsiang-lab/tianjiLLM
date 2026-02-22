//go:build e2e

package e2e

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/require"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

// ---------------------------------------------------------------------------
// Fixture — test context for each Test* function.
//
// Each test calls setup(t) once → gets an isolated, logged-in page.
// t.Cleanup handles page close + DB wipe automatically.
// ---------------------------------------------------------------------------

type Fixture struct {
	T    *testing.T
	Page playwright.Page
}

func setup(t *testing.T) *Fixture {
	t.Helper()

	// Clean slate
	cleanDB(t)
	t.Cleanup(func() { cleanDB(t) })

	// Isolated browser context → no cross-test cookie bleed
	ctx, err := testBrowser.NewContext()
	require.NoError(t, err)
	t.Cleanup(func() { ctx.Close() })

	page, err := ctx.NewPage()
	require.NoError(t, err)

	// Auto-accept browser confirm/alert dialogs (hx-confirm)
	page.OnDialog(func(dialog playwright.Dialog) {
		dialog.Accept()
	})

	// Login
	_, err = page.Goto(testServer.URL + "/ui/login")
	require.NoError(t, err)
	require.NoError(t, page.Locator("#api_key").Fill(masterKey))
	require.NoError(t, page.Locator("button[type=submit]").Click())
	require.NoError(t, page.WaitForURL("**/ui/**"))

	return &Fixture{T: t, Page: page}
}

// ---------------------------------------------------------------------------
// Navigation
// ---------------------------------------------------------------------------

func (f *Fixture) NavigateToKeys() {
	f.T.Helper()
	_, err := f.Page.Goto(testServer.URL + "/ui/keys")
	require.NoError(f.T, err)
	require.NoError(f.T, f.Page.WaitForLoadState())
}

func (f *Fixture) NavigateToKeyDetail(token string) {
	f.T.Helper()
	_, err := f.Page.Goto(testServer.URL + "/ui/keys/" + token)
	require.NoError(f.T, err)
	require.NoError(f.T, f.Page.WaitForLoadState())
}

// ---------------------------------------------------------------------------
// Data seeding
// ---------------------------------------------------------------------------

func (f *Fixture) SeedKeys(n int) []string {
	f.T.Helper()
	ctx := context.Background()
	hashes := make([]string, n)

	for i := range n {
		raw := generateTestKey()
		hash := hashTestKey(raw)
		alias := fmt.Sprintf("test-key-%d", i+1)

		_, err := testDB.CreateVerificationToken(ctx, db.CreateVerificationTokenParams{
			Token:       hash,
			KeyName:     &alias,
			KeyAlias:    &alias,
			Spend:       0,
			Models:      []string{},
			Permissions: []byte("{}"),
			Metadata:    []byte(`{"generated_by":"e2e"}`),
		})
		require.NoError(f.T, err)
		hashes[i] = hash
	}
	return hashes
}

type SeedOpts struct {
	Alias          string
	Spend          float64
	MaxBudget      *float64
	Models         []string
	Blocked        bool
	TeamID         string
	UserID         string
	TPMLimit       *int64
	RPMLimit       *int64
	BudgetDuration string
	Expires        *time.Time
}

func (f *Fixture) SeedKey(opts SeedOpts) string {
	f.T.Helper()
	ctx := context.Background()
	raw := generateTestKey()
	hash := hashTestKey(raw)

	if opts.Models == nil {
		opts.Models = []string{}
	}

	params := db.CreateVerificationTokenParams{
		Token:       hash,
		KeyName:     &opts.Alias,
		KeyAlias:    &opts.Alias,
		Spend:       opts.Spend,
		MaxBudget:   opts.MaxBudget,
		Models:      opts.Models,
		Permissions: []byte("{}"),
		Metadata:    []byte(`{"generated_by":"e2e"}`),
		TpmLimit:    opts.TPMLimit,
		RpmLimit:    opts.RPMLimit,
	}
	if opts.TeamID != "" {
		params.TeamID = &opts.TeamID
	}
	if opts.UserID != "" {
		params.UserID = &opts.UserID
	}
	if opts.BudgetDuration != "" {
		params.BudgetDuration = &opts.BudgetDuration
	}
	if opts.Expires != nil {
		params.Expires = pgtype.Timestamptz{Time: *opts.Expires, Valid: true}
	}

	_, err := testDB.CreateVerificationToken(ctx, params)
	require.NoError(f.T, err)

	if opts.Blocked {
		require.NoError(f.T, testDB.BlockVerificationToken(ctx, hash))
	}

	return hash
}

// ---------------------------------------------------------------------------
// DOM helpers — Playwright Locator API
// ---------------------------------------------------------------------------

// WaitStable waits for the network to be idle after HTMX swaps.
func (f *Fixture) WaitStable() {
	f.T.Helper()
	f.Page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	})
}

// WaitDialogOpen waits until the templUI dialog content is open.
func (f *Fixture) WaitDialogOpen(dialogID string) {
	f.T.Helper()
	sel := fmt.Sprintf(`[data-dialog-instance="%s"][data-tui-dialog-content][data-tui-dialog-open="true"]`, dialogID)
	require.NoError(f.T, f.Page.Locator(sel).WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(5000),
	}))
}

// WaitDialogClose waits until the templUI dialog content is no longer open.
func (f *Fixture) WaitDialogClose(dialogID string) {
	f.T.Helper()
	sel := fmt.Sprintf(`[data-dialog-instance="%s"][data-tui-dialog-content][data-tui-dialog-open="true"]`, dialogID)
	require.NoError(f.T, f.Page.Locator(sel).WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateHidden,
		Timeout: playwright.Float(5000),
	}))
}

// WaitToast waits for a toast notification to appear and returns its text.
func (f *Fixture) WaitToast() string {
	f.T.Helper()
	loc := f.Page.Locator("[data-tui-toast]")
	require.NoError(f.T, loc.WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(5000),
	}))
	text, err := loc.TextContent()
	require.NoError(f.T, err)
	return text
}

// ClickButton finds a <button> by visible text and clicks it.
func (f *Fixture) ClickButton(text string) {
	f.T.Helper()
	require.NoError(f.T, f.Page.GetByRole("button", playwright.PageGetByRoleOptions{
		Name: text,
	}).Click())
}

// ClickLink finds an <a> by visible text and clicks it.
func (f *Fixture) ClickLink(text string) {
	f.T.Helper()
	require.NoError(f.T, f.Page.GetByRole("link", playwright.PageGetByRoleOptions{
		Name: text,
	}).Click())
}

// FilterByName fills a filter input inside #key-filters by name attribute.
// Uses the #key-filters scope to avoid strict mode violations when the same
// name exists in dialogs (e.g. create-key-dialog also has name="key_alias").
func (f *Fixture) FilterByName(name, value string) {
	f.T.Helper()
	sel := fmt.Sprintf(`#key-filters input[name="%s"]`, name)
	require.NoError(f.T, f.Page.Locator(sel).Fill(value))
}

// InputByID fills an element by #id. Playwright Fill triggers real input events,
// so HTMX "input changed" triggers fire correctly.
func (f *Fixture) InputByID(id, value string) {
	f.T.Helper()
	require.NoError(f.T, f.Page.Locator("#"+id).Fill(value))
}

// EnableByValue enables a button when an input's value matches the expected string.
// Workaround for templ ComponentScript oninput bindings that don't render in the DOM
// (templ sets oninput as ComponentScript type in Attributes spread, but the script
// function never gets injected — both el.oninput and the global function are null).
func (f *Fixture) EnableByValue(inputID, expected, btnID string) {
	f.T.Helper()
	_, err := f.Page.Evaluate(`([inputID, expected, btnID]) => {
		const el = document.getElementById(inputID);
		const btn = document.getElementById(btnID);
		if (el.value === expected) {
			btn.removeAttribute("disabled");
		} else {
			btn.setAttribute("disabled", "true");
		}
	}`, []string{inputID, expected, btnID})
	require.NoError(f.T, err)
}

// SelectByName sets a <select name="..."> to a value.
func (f *Fixture) SelectByName(name, value string) {
	f.T.Helper()
	sel := fmt.Sprintf(`select[name="%s"]`, name)
	_, err := f.Page.Locator(sel).SelectOption(playwright.SelectOptionValues{
		Values: &[]string{value},
	})
	require.NoError(f.T, err)
}

// Text returns the text content of a CSS selector, or "" if not found.
func (f *Fixture) Text(selector string) string {
	f.T.Helper()
	loc := f.Page.Locator(selector)
	count, err := loc.Count()
	if err != nil || count == 0 {
		return ""
	}
	text, err := loc.TextContent()
	if err != nil {
		return ""
	}
	return text
}

// Has returns true if the selector matches at least one visible element.
func (f *Fixture) Has(selector string) bool {
	f.T.Helper()
	count, err := f.Page.Locator(selector).Count()
	return err == nil && count > 0
}

// Count returns the number of elements matching a CSS selector.
func (f *Fixture) Count(selector string) int {
	f.T.Helper()
	count, err := f.Page.Locator(selector).Count()
	if err != nil {
		return 0
	}
	return count
}

// Attribute returns the attribute value of the first element matching selector.
func (f *Fixture) Attribute(selector, attr string) *string {
	f.T.Helper()
	val, err := f.Page.Locator(selector).GetAttribute(attr)
	if err != nil {
		return nil
	}
	if val == "" {
		return nil
	}
	return &val
}

// URL returns the current page URL.
func (f *Fixture) URL() string {
	return f.Page.URL()
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func cleanDB(t *testing.T) {
	t.Helper()
	_, err := testPool.Exec(context.Background(), `DELETE FROM "VerificationToken"`)
	require.NoError(t, err)
}

func generateTestKey() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return "sk-" + hex.EncodeToString(b)
}

func hashTestKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func ptr[T any](v T) *T { return &v }
