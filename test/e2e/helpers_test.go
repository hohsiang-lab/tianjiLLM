//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/require"

	"github.com/praxisllmlab/tianjiLLM/internal/auth"
	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

// defaultWaitTimeout is the upper bound Playwright waits for a locator/state
// to resolve. Raised from 5s → 15s because self-hosted CI runners occasionally
// take longer than 5s to land an HTMX swap under load (see flaky runs on main
// and PRs #122-#124). The value is a ceiling, not a target — passing tests
// still return as soon as the condition is met.
const defaultWaitTimeout = 15000.0

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
	uiHandler.RateLimitStore = callback.NewInMemoryRateLimitStore()
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

func (f *Fixture) NavigateToCredentials() {
	f.T.Helper()
	_, err := f.Page.Goto(testServer.URL + "/ui/credentials")
	require.NoError(f.T, err)
	require.NoError(f.T, f.Page.WaitForLoadState())
}

func (f *Fixture) NavigateToCredentialDetail(credentialID string) {
	f.T.Helper()
	_, err := f.Page.Goto(testServer.URL + "/ui/credentials/" + credentialID)
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

type SeedCredentialOpts struct {
	ID             string
	Name           string
	Type           string
	Value          string
	Info           map[string]any
	OrganizationID string
}

func (f *Fixture) SeedCredential(opts SeedCredentialOpts) string {
	f.T.Helper()
	ctx := context.Background()

	if opts.ID == "" {
		opts.ID = "cred-" + generateTestKey()[20:32]
	}
	if opts.Name == "" {
		opts.Name = opts.ID
	}
	if opts.Type == "" {
		opts.Type = "openai_subscription"
	}
	if opts.Value == "" {
		opts.Value = "encrypted-test-value-" + opts.ID
	}
	if opts.Info == nil {
		opts.Info = map[string]any{}
	}
	info, err := json.Marshal(opts.Info)
	require.NoError(f.T, err)

	params := db.CreateCredentialParams{
		CredentialID:    opts.ID,
		CredentialName:  opts.Name,
		CredentialType:  opts.Type,
		CredentialValue: opts.Value,
		CredentialInfo:  info,
		CreatedBy:       "e2e",
	}
	if opts.OrganizationID != "" {
		params.OrganizationID = &opts.OrganizationID
	}
	_, err = testDB.CreateCredential(ctx, params)
	require.NoError(f.T, err)
	return opts.ID
}

func (f *Fixture) SeedOpenAIQuotaState(credentialID string, state callback.OpenAIQuotaState) {
	f.T.Helper()
	require.NotNil(f.T, uiHandler.RateLimitStore)
	uiHandler.RateLimitStore.SetOpenAIQuotaState(credentialID, state)
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
	f.waitDialog(dialogID, playwright.WaitForSelectorStateVisible)
	// dialog.min.js defers autofocus by 50ms after marking the dialog open.
	// Wait it out so a pending focus shift cannot interrupt a later Fill call.
	f.Page.WaitForTimeout(75)
}

// WaitDialogClose waits until the templUI dialog content is no longer open.
func (f *Fixture) WaitDialogClose(dialogID string) {
	f.T.Helper()
	f.waitDialog(dialogID, playwright.WaitForSelectorStateHidden)
}

func (f *Fixture) waitDialog(dialogID string, state *playwright.WaitForSelectorState) {
	sel := fmt.Sprintf(`[data-dialog-instance="%s"][data-tui-dialog-content][data-tui-dialog-open="true"]`, dialogID)
	require.NoError(f.T, f.Page.Locator(sel).WaitFor(playwright.LocatorWaitForOptions{
		State:   state,
		Timeout: playwright.Float(defaultWaitTimeout),
	}))
}

// WaitToast waits for a toast notification to appear and returns its text.
func (f *Fixture) WaitToast() string {
	f.T.Helper()
	loc := f.Page.Locator("[data-tui-toast]")
	require.NoError(f.T, loc.WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(defaultWaitTimeout),
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

// ClickButtonIn finds a <button> by visible text within a container and clicks it.
func (f *Fixture) ClickButtonIn(container, text string) {
	f.T.Helper()
	require.NoError(f.T, f.Page.Locator(container+" button").Filter(playwright.LocatorFilterOptions{
		HasText: text,
	}).Click())
}

// ClickTab clicks a templUI tab trigger by visible text.
func (f *Fixture) ClickTab(text string) {
	f.T.Helper()
	require.NoError(f.T, f.Page.Locator("[data-tui-tabs-trigger]").Filter(playwright.LocatorFilterOptions{
		HasText: text,
	}).Click())
	f.WaitStable()
}

// SubmitDialog clicks the submit button inside a dialog by visible text.
func (f *Fixture) SubmitDialog(dialogID, text string) {
	f.T.Helper()
	sel := fmt.Sprintf("#%s button[type=submit]", dialogID)
	require.NoError(f.T, f.Page.Locator(sel).Filter(playwright.LocatorFilterOptions{
		HasText: text,
	}).Click())
	f.WaitStable()
}

// ConfirmDelete types the alias into the delete confirmation input,
// enables the button (workaround for templ ComponentScript), and clicks it.
func (f *Fixture) ConfirmDelete(alias string) {
	f.T.Helper()
	f.InputByID("confirm_alias", alias)
	f.EnableByValue("confirm_alias", alias, "delete-confirm-btn")
	f.WaitStable()
	require.NoError(f.T, f.Page.Locator("#delete-confirm-btn").Click())
}

// WaitKeyReveal waits for the key reveal dialog to appear after creation.
func (f *Fixture) WaitKeyReveal() {
	f.T.Helper()
	require.NoError(f.T, f.Page.Locator("text=Save your Key").WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(defaultWaitTimeout),
	}))
	f.WaitStable()
}

// CloseKeyReveal closes the key reveal dialog by clicking "Done".
func (f *Fixture) CloseKeyReveal() {
	f.T.Helper()
	f.ClickButtonIn("#key-reveal-dialog", "Done")
	f.WaitStable()
}

// NavigateToSettings navigates to key detail and clicks the Settings tab.
func (f *Fixture) NavigateToSettings(token string) {
	f.T.Helper()
	f.NavigateToKeyDetail(token)
	f.ClickTab("Settings")
}

// NavigateToSettingsEdit navigates to key detail → Settings → Edit Settings.
func (f *Fixture) NavigateToSettingsEdit(token string) {
	f.T.Helper()
	f.NavigateToSettings(token)
	// Wait for the settings tab content to become visible after client-side tab switch.
	require.NoError(f.T, f.Page.Locator("#settings-content").WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(defaultWaitTimeout),
	}))
	f.ClickButton("Edit Settings")
	f.WaitStable()
}

// WaitRegenerateResult waits for the regenerated key to appear in #regenerate-result.
// HTMX swaps the result asynchronously; this ensures the DOM is updated before assertions.
func (f *Fixture) WaitRegenerateResult() {
	f.T.Helper()
	require.NoError(f.T, f.Page.Locator("#regenerate-result button").WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(defaultWaitTimeout),
	}))
}

// WaitForTextIn waits until the text content of the given selector contains substr.
// Useful after HTMX swaps where WaitStable (networkidle) may resolve before DOM update.
func (f *Fixture) WaitForTextIn(selector, substr string) {
	f.T.Helper()
	loc := f.Page.Locator(selector)
	require.NoError(f.T, loc.Filter(playwright.LocatorFilterOptions{
		HasText: substr,
	}).WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(defaultWaitTimeout),
	}))
}

// FilterByName fills a filter input inside #key-filters by name attribute.
// Uses the #key-filters scope to avoid strict mode violations when the same
// name exists in dialogs (e.g. create-key-dialog also has name="key_alias").
func (f *Fixture) FilterByName(name, value string) {
	f.T.Helper()
	sel := fmt.Sprintf(`#key-filters input[name="%s"]`, name)
	require.NoError(f.T, f.Page.Locator(sel).First().Fill(value))
}

// InputByID fills an element by #id. Playwright Fill triggers real input events,
// so HTMX "input changed" triggers fire correctly.
func (f *Fixture) InputByID(id, value string) {
	f.T.Helper()
	require.NoError(f.T, f.Page.Locator(visibleIDSelector(id)).Fill(value))
}

// InputByIDIn fills an element by #id within a container. Use this for dialog
// forms where hidden templates or swapped fragments may duplicate field IDs.
func (f *Fixture) InputByIDIn(container, id, value string) {
	f.T.Helper()
	require.NoError(f.T, f.Page.Locator(container).Locator(visibleIDSelector(id)).Fill(value))
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

// InputValue returns the current value of an input element by #id.
func (f *Fixture) InputValue(id string) string {
	f.T.Helper()
	val, err := f.Page.Locator(visibleIDSelector(id)).InputValue()
	require.NoError(f.T, err)
	return val
}

// InputValueIn returns the current value of an input by #id within a container.
func (f *Fixture) InputValueIn(container, id string) string {
	f.T.Helper()
	val, err := f.Page.Locator(container).Locator(visibleIDSelector(id)).InputValue()
	require.NoError(f.T, err)
	return val
}

func visibleIDSelector(id string) string {
	return fmt.Sprintf(`[id="%s"]:visible`, id)
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

// WaitTextContains waits until a selector's text includes the expected value.
func (f *Fixture) WaitTextContains(selector, expected string) {
	f.T.Helper()
	require.Eventually(f.T, func() bool {
		return strings.Contains(f.Text(selector), expected)
	}, time.Duration(defaultWaitTimeout)*time.Millisecond, 100*time.Millisecond)
}

// WaitTextNotContains waits until a selector's text no longer includes the value.
func (f *Fixture) WaitTextNotContains(selector, unexpected string) {
	f.T.Helper()
	require.Eventually(f.T, func() bool {
		return !strings.Contains(f.Text(selector), unexpected)
	}, time.Duration(defaultWaitTimeout)*time.Millisecond, 100*time.Millisecond)
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

// URL returns the current page URL.
func (f *Fixture) URL() string {
	return f.Page.URL()
}

// ---------------------------------------------------------------------------
// Model navigation & seeding
// ---------------------------------------------------------------------------

func (f *Fixture) NavigateToModels() {
	f.T.Helper()
	_, err := f.Page.Goto(testServer.URL + "/ui/models")
	require.NoError(f.T, err)
	require.NoError(f.T, f.Page.WaitForLoadState())
}

type SeedModelOpts struct {
	ModelName string
	Model     string // provider/id format
	APIKey    string
	APIBase   string
	TPM       int64
	RPM       int64
	Extra     map[string]any // extra tianji_params fields (e.g. timeout, region)
}

func (f *Fixture) SeedModel(opts SeedModelOpts) string {
	f.T.Helper()
	ctx := context.Background()

	if opts.Model == "" {
		opts.Model = "openai/gpt-4o"
	}

	tp := map[string]any{"model": opts.Model}
	if opts.APIKey != "" {
		tp["api_key"] = opts.APIKey
	}
	if opts.APIBase != "" {
		tp["api_base"] = opts.APIBase
	}
	if opts.TPM > 0 {
		tp["tpm"] = opts.TPM
	}
	if opts.RPM > 0 {
		tp["rpm"] = opts.RPM
	}
	for k, v := range opts.Extra {
		tp[k] = v
	}

	tpJSON, err := json.Marshal(tp)
	require.NoError(f.T, err)

	modelID := fmt.Sprintf("e2e-%s", generateTestKey()[20:32])
	_, err = testDB.CreateProxyModel(ctx, db.CreateProxyModelParams{
		ModelID:      modelID,
		ModelName:    opts.ModelName,
		TianjiParams: tpJSON,
		ModelInfo:    []byte("{}"),
		CreatedBy:    "e2e",
	})
	require.NoError(f.T, err)
	return modelID
}

func (f *Fixture) SeedModels(n int) []string {
	f.T.Helper()
	ids := make([]string, n)
	for i := range n {
		ids[i] = f.SeedModel(SeedModelOpts{
			ModelName: fmt.Sprintf("e2e-model-%d", i+1),
			Model:     "openai/gpt-4o",
		})
	}
	return ids
}

func (f *Fixture) FilterModels(search string) {
	f.T.Helper()
	sel := `#model-filters input[name="search"]`
	require.NoError(f.T, f.Page.Locator(sel).Fill(search))
	time.Sleep(500 * time.Millisecond)
	f.WaitStable()
}

// ---------------------------------------------------------------------------
// Logs navigation & seeding
// ---------------------------------------------------------------------------

func (f *Fixture) NavigateToLogs() {
	f.T.Helper()
	_, err := f.Page.Goto(testServer.URL + "/ui/logs")
	require.NoError(f.T, err)
	require.NoError(f.T, f.Page.WaitForLoadState())
	// Wait for #logs-table to be present in the DOM
	require.NoError(f.T, f.Page.Locator("#logs-table").WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(defaultWaitTimeout),
	}))
}

type SeedSpendLogOpts struct {
	RequestID       string
	Model           string
	ApiKey          string
	Spend           float64
	Tokens          int32 // total
	Prompt          int32
	Completion      int32
	TeamID          string
	ReasoningEffort string
	StartAge        time.Duration // how long ago (default: 1 minute)
	DurationMs      int           // request duration in ms (default: 500)
}

func (f *Fixture) SeedSpendLog(opts SeedSpendLogOpts) string {
	f.T.Helper()
	ctx := context.Background()

	if opts.RequestID == "" {
		opts.RequestID = "req-" + generateTestKey()[20:32]
	}
	if opts.Model == "" {
		opts.Model = "openai/gpt-4o"
	}
	if opts.ApiKey == "" {
		opts.ApiKey = hashTestKey("sk-test-key")
	}
	if opts.StartAge == 0 {
		opts.StartAge = 1 * time.Minute
	}
	if opts.DurationMs == 0 {
		opts.DurationMs = 500
	}

	start := time.Now().Add(-opts.StartAge)
	end := start.Add(time.Duration(opts.DurationMs) * time.Millisecond)

	params := db.CreateSpendLogParams{
		RequestID:        opts.RequestID,
		CallType:         "completion",
		ApiKey:           opts.ApiKey,
		Spend:            opts.Spend,
		TotalTokens:      opts.Tokens,
		PromptTokens:     opts.Prompt,
		CompletionTokens: opts.Completion,
		Starttime:        pgtype.Timestamptz{Time: start, Valid: true},
		Endtime:          pgtype.Timestamptz{Time: end, Valid: true},
		Model:            opts.Model,
		ReasoningEffort:  opts.ReasoningEffort,
		Metadata:         []byte("{}"),
		RequestTags:      []string{},
	}
	if opts.TeamID != "" {
		params.TeamID = &opts.TeamID
	}

	err := testDB.CreateSpendLog(ctx, params)
	require.NoError(f.T, err)
	return opts.RequestID
}

func (f *Fixture) SeedErrorLog(requestID, model, errorType string, statusCode int32) {
	f.T.Helper()
	ctx := context.Background()

	err := testDB.InsertErrorLog(ctx, db.InsertErrorLogParams{
		RequestID:    requestID,
		Model:        model,
		Provider:     "openai",
		StatusCode:   statusCode,
		ErrorType:    errorType,
		ErrorMessage: "test error: " + errorType,
	})
	require.NoError(f.T, err)

	_, err = testPool.Exec(ctx,
		`UPDATE "ErrorLogs" SET created_at = $1 WHERE request_id = $2`,
		pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Minute), Valid: true},
		requestID,
	)
	require.NoError(f.T, err)
}

// FilterLogs fills a filter input inside #log-filters by name attribute.
func (f *Fixture) FilterLogs(name, value string) {
	f.T.Helper()
	sel := fmt.Sprintf(`#log-filters input[name="%s"]`, name)
	require.NoError(f.T, f.Page.Locator(sel).Fill(value))
}

// SelectLogFilter selects a value from a <select> inside #log-filters.
func (f *Fixture) SelectLogFilter(name, value string) {
	f.T.Helper()
	sel := fmt.Sprintf(`#log-filters select[name="%s"]`, name)
	_, err := f.Page.Locator(sel).SelectOption(playwright.SelectOptionValues{
		Values: &[]string{value},
	})
	require.NoError(f.T, err)
}

// ---------------------------------------------------------------------------
// Organizations seeding for retained credential/OpenAI E2E flows
// ---------------------------------------------------------------------------

type SeedOrgOpts struct {
	Alias     string
	MaxBudget *float64
	Models    []string
	Spend     float64
}

func (f *Fixture) SeedOrg(opts SeedOrgOpts) string {
	f.T.Helper()
	ctx := context.Background()

	orgID := "org-" + generateTestKey()[20:32]
	if opts.Models == nil {
		opts.Models = []string{}
	}

	params := db.CreateOrganizationParams{
		OrganizationID:    orgID,
		OrganizationAlias: &opts.Alias,
		MaxBudget:         opts.MaxBudget,
		Models:            opts.Models,
		CreatedBy:         "e2e",
	}

	_, err := testDB.CreateOrganization(ctx, params)
	require.NoError(f.T, err)

	if opts.Spend > 0 {
		_, err := testPool.Exec(ctx, `UPDATE "OrganizationTable" SET spend = $1 WHERE organization_id = $2`, opts.Spend, orgID)
		require.NoError(f.T, err)
	}

	return orgID
}

func (f *Fixture) SeedUser(userID string) {
	f.T.Helper()
	ctx := context.Background()
	_, err := testDB.CreateUser(ctx, db.CreateUserParams{
		UserID:   userID,
		UserRole: "internal_user",
		Teams:    []string{},
		Models:   []string{},
	})
	require.NoError(f.T, err)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func cleanDB(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	// Use a dedicated connection (not pool) so SET is session-scoped.
	conn, err := testPool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()

	// Disable FK trigger checks so deletion order doesn't matter.
	_, err = conn.Exec(ctx, `SET session_replication_role = 'replica'`)
	require.NoError(t, err)

	for _, tbl := range []string{
		"RequestPayloads", "ErrorLogs", "SpendLogs", "VerificationToken",
		"ProxyModelTable", "ModelAccessGroup", "CredentialTable",
		"OrganizationMembership", "ModelPricing",
		"TeamTable", "OrganizationTable", "UserTable",
		"OAuthTokenRateLimitState",
	} {
		_, err := conn.Exec(ctx, fmt.Sprintf(`DELETE FROM "%s"`, tbl))
		require.NoError(t, err)
	}

	_, err = conn.Exec(ctx, `SET session_replication_role = 'origin'`)
	require.NoError(t, err)
}

func generateTestKey() string {
	return auth.GenerateVirtualKey()
}

func hashTestKey(key string) string {
	return auth.HashKey(key)
}
