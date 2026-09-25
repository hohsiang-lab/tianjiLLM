//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/praxisllmlab/tianjiLLM/internal/cache"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/openaioauth"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
	"github.com/praxisllmlab/tianjiLLM/internal/testutil/openaitest"
)

func TestOpenAIConnectFlow_AuthenticatedSuccessWithMockOAuth(t *testing.T) {
	f := setup(t)
	oauthServer := configureOpenAIOAuthMock(t, openaitest.OAuthServerOptions{TokenFixtures: []openaitest.TokenFixture{{
		Code: "fixture-authorization-code-success",
		Response: map[string]any{
			"access_token":  "[REDACTED]",
			"refresh_token": "[REDACTED]",
			"id_token":      "[REDACTED]",
			"expires_in":    3600,
			"scope":         "openid profile email",
			"account_id":    "[REDACTED]",
			"email":         "admin@example.com",
		},
	}}})
	f.NavigateToCredentials()
	page, err := f.Page.Context().ExpectPage(func() error {
		return f.Page.GetByRole("link", playwright.PageGetByRoleOptions{
			Name: "Browser callback fallback",
		}).Click()
	})
	require.NoError(t, err)
	require.NoError(t, page.WaitForURL(oauthServer.URL()+"/oauth/authorize**"))

	current := page.URL()
	authorizeURL, err := url.Parse(current)
	require.NoError(t, err)
	assert.Equal(t, oauthServer.AuthorizeURL(), authorizeURL.Scheme+"://"+authorizeURL.Host+authorizeURL.Path)
	assert.NotEmpty(t, authorizeURL.Query().Get("state"))
	assert.NotEmpty(t, authorizeURL.Query().Get("code_challenge"))
	assert.Equal(t, "S256", authorizeURL.Query().Get("code_challenge_method"))
	assert.Equal(t, "http://localhost:1455/auth/callback", authorizeURL.Query().Get("redirect_uri"))
	assert.Equal(t, "app_test", authorizeURL.Query().Get("client_id"))

	state := currentQueryValue(t, current, "state")
	require.NotEmpty(t, state)
	_, err = f.Page.Goto(testServer.URL + "/oauth/openai/callback?state=" + state + "&code=fixture-authorization-code-success")
	require.NoError(t, err)
	require.NoError(t, f.Page.WaitForLoadState())

	body := f.Text("body")
	assert.Contains(t, body, "OpenAI connected")
	assert.Contains(t, body, "OpenAI subscription credential connected successfully.")
	html, err := f.Page.Locator("body").InnerHTML()
	require.NoError(t, err)
	assertOpenAIConnectDOMHasNoSecrets(t, html)
	assert.NotContains(t, html, "fixture-authorization-code-success")

	require.NoError(t, f.Page.GetByRole("link", playwright.PageGetByRoleOptions{
		Name: "Back to Credentials",
	}).Click())
	require.NoError(t, f.Page.WaitForURL("**/ui/credentials"))

	body = f.Text("body")
	assert.Contains(t, body, "OpenAI Subscription")
	assert.Contains(t, body, "admin@example.com")
	assertGlobalOpenAISubscriptionCredential(t, "admin@example.com")

	html, err = f.Page.Locator("body").InnerHTML()
	require.NoError(t, err)
	assertOpenAIConnectDOMHasNoSecrets(t, html)
}

func TestOpenAIDeviceConnectFlow_AuthenticatedSuccessWithMockOAuth(t *testing.T) {
	f := setup(t)
	require.NoError(t, f.Page.Clock().Install())
	const (
		deviceAuthID      = "fixture-device-auth-id"
		userCode          = "fixture-user-code"
		authorizationCode = "fixture-authorization-code"
		codeVerifier      = "fixture-code-verifier"
	)
	codeChallenge := openai.ChallengeFromVerifier(codeVerifier)
	oauthServer := configureOpenAIDeviceOAuthMock(t, openaitest.DeviceAuthOptions{
		UserCodeResponse: map[string]any{
			"device_auth_id": deviceAuthID,
			"user_code":      userCode,
			"interval":       1,
		},
		PollResponses: []openaitest.DevicePollResponse{
			{StatusCode: http.StatusForbidden, Response: map[string]any{"error": "authorization_pending"}},
			{StatusCode: http.StatusOK, Response: map[string]any{
				"authorization_code": authorizationCode,
				"code_challenge":     codeChallenge,
				"code_verifier":      codeVerifier,
			}},
		},
	}, []openaitest.TokenFixture{{
		Code: authorizationCode,
		Response: map[string]any{
			"access_token":  "[REDACTED]",
			"refresh_token": "[REDACTED]",
			"id_token":      "[REDACTED]",
			"expires_in":    3600,
			"scope":         "openid profile email",
			"account_id":    "[REDACTED]",
			"email":         "admin@example.com",
		},
	}})

	f.NavigateToCredentials()
	statusRequests := observeDeviceStatusRequests(t, f)
	require.NoError(t, f.Page.GetByRole("button", playwright.PageGetByRoleOptions{
		Name: "Sign in with Device Code",
	}).Click())
	require.NoError(t, f.Page.GetByRole("button", playwright.PageGetByRoleOptions{
		Name: "Get device code",
	}).Click())
	f.WaitForTextIn("#openai-device-auth-dialog-content", "fixture-user-code")
	f.WaitForTextIn("#openai-device-auth-dialog-content", "OpenAI credential connected")
	terminalPollCount := countDevicePollRequests(oauthServer.Requests())
	time.Sleep(2200 * time.Millisecond)
	assert.Equal(t, terminalPollCount, countDevicePollRequests(oauthServer.Requests()))
	assertDevicePollingStopped(t, f, statusRequests)

	html, err := f.Page.Locator("#openai-device-auth-dialog-content").InnerHTML()
	require.NoError(t, err)
	assert.NotContains(t, html, deviceAuthID)
	assert.NotContains(t, html, "[REDACTED]")
	assert.NotContains(t, html, "[REDACTED]")
	assert.NotContains(t, html, "[REDACTED]")
	assertGlobalOpenAISubscriptionCredential(t, "admin@example.com")

	require.Eventually(t, func() bool {
		for _, request := range oauthServer.Requests() {
			if request.Path == "/oauth/token" && request.Form.Get("code") == authorizationCode {
				return request.Form.Get("code_verifier") == codeVerifier
			}
		}
		return false
	}, 5*time.Second, 100*time.Millisecond)

	var userCodeRequest, deviceTokenRequest, tokenRequest *openaitest.RecordedRequest
	for _, request := range oauthServer.Requests() {
		request := request
		switch request.Path {
		case "/api/accounts/deviceauth/usercode":
			userCodeRequest = &request
		case "/api/accounts/deviceauth/token":
			deviceTokenRequest = &request
		case "/oauth/token":
			tokenRequest = &request
		}
	}
	require.NotNil(t, userCodeRequest)
	require.NotNil(t, deviceTokenRequest)
	require.NotNil(t, tokenRequest)
	assert.Equal(t, http.MethodPost, userCodeRequest.Method)
	assert.Equal(t, http.MethodPost, deviceTokenRequest.Method)
	assert.Equal(t, http.MethodPost, tokenRequest.Method)
	assert.Contains(t, string(deviceTokenRequest.Body), "device_auth_id")
	assert.Contains(t, string(deviceTokenRequest.Body), "user_code")
	assert.Equal(t, authorizationCode, tokenRequest.Form.Get("code"))
}

func TestOpenAIConnectFlow_LocalhostPasteSuccessWithMockOAuth(t *testing.T) {
	f := setup(t)
	oauthServer := configureOpenAIOAuthMock(t, openaitest.OAuthServerOptions{TokenFixtures: []openaitest.TokenFixture{{
		Code: "fixture-authorization-code-success",
		Response: map[string]any{
			"access_token":  "[REDACTED]",
			"refresh_token": "[REDACTED]",
			"id_token":      "[REDACTED]",
			"expires_in":    3600,
			"scope":         "openid profile email",
			"account_id":    "[REDACTED]",
			"email":         "admin@example.com",
		},
	}}})
	f.NavigateToCredentials()
	page, err := f.Page.Context().ExpectPage(func() error {
		return f.Page.GetByRole("link", playwright.PageGetByRoleOptions{
			Name: "Browser callback fallback",
		}).Click()
	})
	require.NoError(t, err)
	require.NoError(t, page.WaitForURL(oauthServer.URL()+"/oauth/authorize**"))

	authorizeURL, err := url.Parse(page.URL())
	require.NoError(t, err)
	state := authorizeURL.Query().Get("state")
	require.NotEmpty(t, state)
	assert.Equal(t, "http://localhost:1455/auth/callback", authorizeURL.Query().Get("redirect_uri"))
	require.Equal(t, testServer.URL+"/ui/credentials", f.Page.URL())

	callbackURL := "http://localhost:1455/auth/callback?state=" + url.QueryEscape(state) + "&code=fixture-authorization-code-success"
	require.NoError(t, f.Page.GetByRole("button", playwright.PageGetByRoleOptions{
		Name: "Paste callback URL",
	}).Click())
	require.NoError(t, f.Page.GetByRole("textbox", playwright.PageGetByRoleOptions{
		Name: "Callback URL",
	}).Fill(callbackURL))
	require.NoError(t, f.Page.GetByRole("button", playwright.PageGetByRoleOptions{
		Name: "Connect credential",
	}).Click())

	f.WaitForTextIn("body", "OpenAI connected")
	body := f.Text("body")
	assert.Contains(t, body, "OpenAI subscription credential connected successfully.")
	assert.NotContains(t, body, callbackURL)
	assert.NotContains(t, body, "fixture-authorization-code-success")

	require.NoError(t, f.Page.GetByRole("link", playwright.PageGetByRoleOptions{
		Name: "Back to Credentials",
	}).Click())
	require.NoError(t, f.Page.WaitForURL("**/ui/credentials"))
	body = f.Text("body")
	assert.Contains(t, body, "OpenAI Subscription")
	assert.Contains(t, body, "admin@example.com")
	assertGlobalOpenAISubscriptionCredential(t, "admin@example.com")
}

// Exercise the shared runtime on its real route-owned consumers, not copied HTML.
func TestOpenAIDialogKeyboard(t *testing.T) {
	f := setup(t)
	configureOpenAIOAuthMock(t, openaitest.OAuthServerOptions{})
	for _, tc := range []struct{ name, route, id, first string }{
		{"paste", "/ui/credentials", "openai-callback-url-dialog", "#callback_url"},
		{"user", "/ui/users", "create-user-dialog", "#user_email"},
		{"model", "/ui/models", "create-model-dialog", "#model_name"},
		{"key", "/ui/keys", "create-key-dialog", "#key_alias"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.Page.Goto(testServer.URL + tc.route)
			require.NoError(t, err)
			require.NoError(t, f.Page.WaitForLoadState())
			content := `[data-tui-dialog-content][data-dialog-instance="` + tc.id + `"]`
			opener := f.Page.Locator(`#` + tc.id + ` [data-tui-dialog-trigger] button`)
			_, err = f.Page.Evaluate(`selector => {
				window.dialogInitialFocus = new Promise(resolve => {
					document.querySelector(selector).addEventListener('focusin', () =>
						queueMicrotask(() => resolve(document.activeElement.id)), {once: true});
				});
			}`, content)
			require.NoError(t, err)
			require.NoError(t, opener.Click())
			initial, err := f.Page.Evaluate(`window.dialogInitialFocus`)
			require.NoError(t, err)
			assert.Equal(t, tc.first[1:], initial, "initial focus belongs to the first control")
			first := f.Page.Locator(tc.first)
			last := f.Page.Locator(content + ` button[aria-label="Close"]`)
			for _, boundary := range []struct {
				name, key   string
				start, want playwright.Locator
			}{
				{"forward wrap", "Tab", last, first},
				{"reverse wrap", "Shift+Tab", first, last},
				{"container forward", "Tab", f.Page.Locator(content), first},
				{"container reverse", "Shift+Tab", f.Page.Locator(content), last},
				{"outside forward", "Tab", opener, first},
				{"outside reverse", "Shift+Tab", opener, last},
			} {
				require.NoError(t, boundary.start.Focus())
				require.NoError(t, f.Page.Keyboard().Press(boundary.key))
				focused, focusErr := boundary.want.Evaluate(`el => el === document.activeElement`, nil)
				require.NoError(t, focusErr)
				assert.Equal(t, true, focused, boundary.name)
			}
			require.NoError(t, first.Fill("fixture"))
			require.NoError(t, f.Page.Keyboard().Press("Escape"))
			_, err = f.Page.WaitForFunction(`id => document.querySelector('[data-tui-dialog-content][data-dialog-instance="'+id+'"]').hasAttribute('data-tui-dialog-hidden') && document.activeElement.closest('[data-tui-dialog-trigger]')?.getAttribute('data-dialog-instance') === id`, tc.id)
			require.NoError(t, err)
			require.NoError(t, opener.Click())
			require.NoError(t, last.Click())
			_, err = f.Page.WaitForFunction(`id => document.querySelector('[data-tui-dialog-content][data-dialog-instance="'+id+'"]').hasAttribute('data-tui-dialog-hidden') && document.activeElement.closest('[data-tui-dialog-trigger]')?.getAttribute('data-dialog-instance') === id`, tc.id)
			require.NoError(t, err)
		})
	}
}

// Real centered callback dialog: user focus during the 50ms autofocus window must win.
func TestOpenAIDialogAutofocusPreservesUserFocus(t *testing.T) {
	f := setup(t)
	configureOpenAIOAuthMock(t, openaitest.OAuthServerOptions{})
	require.NoError(t, f.Page.SetViewportSize(1280, 900))
	f.NavigateToCredentials()
	require.NoError(t, f.Page.Clock().Install())

	opener := f.Page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Paste callback URL", Exact: playwright.Bool(true)})
	content := f.Page.Locator(`[data-tui-dialog-content][data-dialog-instance="openai-callback-url-dialog"]`)
	second := content.GetByRole("button", playwright.LocatorGetByRoleOptions{Name: "Cancel", Exact: playwright.Bool(true)})
	require.NoError(t, opener.Click())
	require.NoError(t, content.WaitFor())
	withinViewport, err := content.Evaluate(`el => { const r = el.getBoundingClientRect(); return r.x >= 0 && r.y >= 0 && r.right <= innerWidth && r.bottom <= innerHeight }`, nil)
	require.NoError(t, err)
	require.Equal(t, true, withinViewport, "callback dialog is centered within the viewport")
	require.NoError(t, f.Page.Clock().RunFor(20))
	require.NoError(t, second.Focus())
	focused, err := second.Evaluate(`el => el === document.activeElement`, nil)
	require.NoError(t, err)
	require.Equal(t, true, focused, "second dialog control receives user focus during autofocus window")
	require.NoError(t, f.Page.Clock().RunFor(100))
	focused, err = second.Evaluate(`el => el === document.activeElement`, nil)
	require.NoError(t, err)
	assert.Equal(t, true, focused, "50ms autofocus does not steal second dialog control focus")
	require.NoError(t, f.Page.Keyboard().Press("Escape"))
}

// Real mobile sidebar: user Tab during the opening transition must win over deferred autofocus.
func TestOpenAIDialogSidebarKeyboardDuringOpening(t *testing.T) {
	f := setup(t)
	configureOpenAIOAuthMock(t, openaitest.OAuthServerOptions{})
	require.NoError(t, f.Page.SetViewportSize(390, 844))
	f.NavigateToCredentials()
	opener := f.Page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Toggle Sidebar", Exact: playwright.Bool(true)})
	require.NoError(t, opener.Click())
	dialog := f.Page.GetByRole("dialog", playwright.PageGetByRoleOptions{Name: "Navigation", Exact: playwright.Bool(true)})
	require.NoError(t, dialog.WaitFor())
	_, err := f.Page.WaitForFunction(`() => {
		const dialog = document.querySelector('[data-tui-dialog-content][data-dialog-instance="app-sidebar-mobile"]');
		const animation = dialog?.getAnimations().find(animation => animation.effect?.getComputedTiming()?.endTime !== Infinity && animation.playState === 'running');
		return dialog?.getAttribute('data-tui-dialog-open') === 'true' && animation && animation.currentTime >= 100 && animation.currentTime < 450;
	}`, nil)
	require.NoError(t, err, "sidebar CSS opening transition must be active before keyboard input")
	links := dialog.Locator("a[href]:visible")
	count, err := links.Count()
	require.NoError(t, err)
	require.GreaterOrEqual(t, count, 2, "sidebar must expose a second visible navigation control")
	second := links.Nth(1)
	require.NoError(t, f.Page.Keyboard().Press("Tab"))
	require.NoError(t, f.Page.Keyboard().Press("Tab"))
	focused, err := second.Evaluate(`el => el === document.activeElement`, nil)
	require.NoError(t, err)
	require.Equal(t, true, focused, "second visible navigation control receives the second Tab")
	_, err = f.Page.Evaluate(`async () => {
		const dialog = document.querySelector('[data-tui-dialog-content][data-dialog-instance="app-sidebar-mobile"]');
		await Promise.allSettled(dialog.getAnimations().map(animation => animation.finished));
		await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)));
	}`, nil)
	require.NoError(t, err)
	focused, err = second.Evaluate(`el => el === document.activeElement`, nil)
	require.NoError(t, err)
	assert.Equal(t, true, focused, "deferred autofocus does not steal second-control focus after opening settles")
	require.NoError(t, f.Page.Keyboard().Press("Escape"))
}

// Real mobile sidebar: hidden duplicate links must not capture autofocus or Tab.
func TestOpenAIDialogSidebarKeyboard(t *testing.T) {
	f := setup(t)
	configureOpenAIOAuthMock(t, openaitest.OAuthServerOptions{})
	require.NoError(t, f.Page.SetViewportSize(390, 844))
	f.NavigateToCredentials()
	require.NoError(t, f.Page.Clock().Install())
	opener := f.Page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Toggle Sidebar", Exact: playwright.Bool(true)})
	require.NoError(t, opener.Click())
	require.NoError(t, f.Page.Clock().RunFor(500))
	dialog := f.Page.GetByRole("dialog", playwright.PageGetByRoleOptions{Name: "Navigation", Exact: playwright.Bool(true)})
	require.NoError(t, dialog.WaitFor())
	first := dialog.Locator("a[href]:visible").First()
	last := dialog.Locator("button:visible").Last()
	require.Eventually(t, func() bool {
		focused, focusErr := first.Evaluate(`el => el === document.activeElement`, nil)
		return focusErr == nil && focused == true
	}, 5*time.Second, 10*time.Millisecond, "initial sidebar link focus must settle before geometry assertions")
	focused, err := first.Evaluate(`el => el === document.activeElement`, nil)
	require.NoError(t, err)
	assert.Equal(t, true, focused, "initial focus selects the first visible sidebar link")
	open, err := dialog.GetAttribute("data-tui-dialog-open")
	require.NoError(t, err)
	assert.Equal(t, "true", open, "sidebar dialog is committed open before focus assertions")
	metrics, err := first.Evaluate(`el => {
		const dialog = el.closest('[role="dialog"]');
		const rect = el.getBoundingClientRect();
		const dialogRect = dialog.getBoundingClientRect();
		const within = (outer, inner) => outer.x >= inner.x && outer.y >= inner.y && outer.right <= inner.right && outer.bottom <= inner.bottom;
		return {
			withinDialog: within(rect, dialogRect),
			withinViewport: within(rect, {x: 0, y: 0, right: innerWidth, bottom: innerHeight}),
			viewportWidth: innerWidth,
			viewportHeight: innerHeight,
			focusVisible: el.matches(':focus-visible'),
		};
	}`, nil)
	require.NoError(t, err)
	data, ok := metrics.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, data["withinDialog"], "initial active sidebar link is fully within the dialog")
	assert.Equal(t, true, data["withinViewport"], "initial active sidebar link is fully within the viewport")
	assert.EqualValues(t, 390, data["viewportWidth"])
	assert.EqualValues(t, 844, data["viewportHeight"])
	assert.Equal(t, true, data["focusVisible"], "initial active sidebar link matches :focus-visible")
	for _, boundary := range []struct {
		name, key  string
		from, want playwright.Locator
	}{
		{"forward", "Tab", last, first},
		{"reverse", "Shift+Tab", first, last},
		{"container-forward", "Tab", dialog, first},
		{"container-reverse", "Shift+Tab", dialog, last},
	} {
		require.NoError(t, boundary.from.Focus())
		require.NoError(t, f.Page.Keyboard().Press(boundary.key))
		focused, err = boundary.want.Evaluate(`el => el === document.activeElement`, nil)
		require.NoError(t, err)
		assert.Equal(t, true, focused, "sidebar visible focus boundary: %s", boundary.name)
		inside, insideErr := dialog.Evaluate(`el => el.contains(document.activeElement)`, nil)
		require.NoError(t, insideErr)
		assert.Equal(t, true, inside, "sidebar containment: %s", boundary.name)
	}
	require.NoError(t, f.Page.Keyboard().Press("Escape"))
	require.NoError(t, f.Page.Clock().RunFor(500))
	focused, err = opener.Evaluate(`el => el === document.activeElement`, nil)
	require.NoError(t, err)
	assert.Equal(t, true, focused, "Escape restores the real opener")
}

// Supplemental eligibility states on a real dialog; the sidebar regression above
// needs no DOM changes. Reopening also exercises the empty-container fallback.
func TestOpenAIDialogFocusableEligibility(t *testing.T) {
	f := setup(t)
	configureOpenAIOAuthMock(t, openaitest.OAuthServerOptions{})
	for _, tc := range []struct{ name, selector, attribute, value string }{
		{"hidden-ancestor", "form", "hidden", ""},
		{"display-none", "form", "style", "display:none"},
		{"visibility-hidden", "form", "style", "visibility:hidden"},
		{"visibility-collapse", "form", "style", "visibility:collapse"},
		{"inert-ancestor", "form", "inert", ""},
		{"disabled", "form :is(button, input, textarea, select)", "disabled", ""},
		{"negative-tabindex", "form :is(button, input, textarea, select)", "tabindex", "-2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f.NavigateToCredentials()
			require.NoError(t, f.Page.Clock().Install())
			dialog := f.Page.Locator(`#openai-callback-url-dialog [role="dialog"]`)
			_, err := dialog.Locator(tc.selector).EvaluateAll(`(els, attr) => els.forEach(el => el.setAttribute(attr[0], attr[1]))`, []string{tc.attribute, tc.value})
			require.NoError(t, err)
			opener := f.Page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Paste callback URL", Exact: playwright.Bool(true)})
			require.NoError(t, opener.Click())
			require.NoError(t, f.Page.Clock().RunFor(500))
			closeButton := dialog.Locator(`button[aria-label="Close"]`)
			focused, err := closeButton.Evaluate(`el => el === document.activeElement`, nil)
			require.NoError(t, err)
			assert.Equal(t, true, focused, "autofocus skips ineligible controls")
			for _, from := range []playwright.Locator{closeButton, dialog} {
				for _, key := range []string{"Tab", "Shift+Tab"} {
					require.NoError(t, from.Focus())
					require.NoError(t, f.Page.Keyboard().Press(key))
					focused, err = closeButton.Evaluate(`el => el === document.activeElement`, nil)
					require.NoError(t, err)
					assert.Equal(t, true, focused, "%s stays on the sole eligible control", key)
				}
			}
			_, err = closeButton.Evaluate(`el => el.disabled = true`, nil)
			require.NoError(t, err)
			for _, key := range []string{"Tab", "Shift+Tab"} {
				require.NoError(t, dialog.Focus())
				require.NoError(t, f.Page.Keyboard().Press(key))
				focused, err = dialog.Evaluate(`el => el === document.activeElement`, nil)
				require.NoError(t, err)
				assert.Equal(t, true, focused, "dynamic empty dialog keeps %s focus", key)
			}
			require.NoError(t, f.Page.Keyboard().Press("Escape"))
			require.NoError(t, f.Page.Clock().RunFor(500))
			require.NoError(t, opener.Click())
			require.NoError(t, f.Page.Clock().RunFor(500))
			focused, err = dialog.Evaluate(`el => el === document.activeElement`, nil)
			require.NoError(t, err)
			assert.Equal(t, true, focused, "empty dialog autofocus falls back to its container")
		})
	}
}

const openAIDeviceTextContrast = `target => {
	const style = getComputedStyle(target);
	// Composite actual ancestor backgrounds and text in the browser's sRGB canvas.
	const canvas = document.createElement('canvas');
	canvas.width = canvas.height = 1;
	const ctx = canvas.getContext('2d', {colorSpace:'srgb'});
	const paint = color => { ctx.fillStyle = color; ctx.fillRect(0, 0, 1, 1); };
	const rgb = () => [...ctx.getImageData(0, 0, 1, 1).data].slice(0, 3);
	const ancestors = [];
	for (let el = target; el; el = el.parentElement) ancestors.unshift(el);
	paint('white');
	ancestors.forEach(el => paint(getComputedStyle(el).backgroundColor));
	const backgroundRGB = rgb();
	paint(style.color);
	const foregroundRGB = rgb();
	const luminance = channels => channels.map(c => c / 255).map(c => c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4).reduce((sum, c, i) => sum + c * [0.2126, 0.7152, 0.0722][i], 0);
	const foregroundLuminance = luminance(foregroundRGB), backgroundLuminance = luminance(backgroundRGB);
	const contrastRatio = (Math.max(foregroundLuminance, backgroundLuminance) + 0.05) / (Math.min(foregroundLuminance, backgroundLuminance) + 0.05);
	return {foregroundRGB, backgroundRGB, contrastRatio, color:style.color, background:style.backgroundColor, fontSize:style.fontSize, fontWeight:style.fontWeight};
}`

func TestOpenAIDeviceErrorTextContrast(t *testing.T) {
	for _, state := range []string{"failed", "unavailable"} {
		t.Run(state, func(t *testing.T) {
			f := setup(t)
			message := "Device login is unavailable."
			if state == "failed" {
				configureOpenAIDeviceOAuthMock(t, openaitest.DeviceAuthOptions{
					UserCodeResponse: map[string]any{"device_auth_id": "fixture-device", "user_code": "FIXT-URE1", "interval": 1},
					PollResponses:    []openaitest.DevicePollResponse{{StatusCode: http.StatusBadRequest, Response: map[string]any{"error": "invalid_request"}}},
				}, nil)
				message = "Device login failed."
			} else {
				configureOpenAIOAuthMock(t, openaitest.OAuthServerOptions{})
			}
			var consoleErrors, faviconErrors atomic.Int64
			f.Page.OnPageError(func(error) { consoleErrors.Add(1) })
			f.Page.OnConsole(func(message playwright.ConsoleMessage) {
				if message.Type() != "error" {
					return
				}
				if loc := message.Location(); loc != nil && loc.URL == testServer.URL+"/favicon.ico" && strings.Contains(message.Text(), "Failed to load resource: the server responded with a status of 401") {
					faviconErrors.Add(1)
					return
				}
				consoleErrors.Add(1)
			})
			require.NoError(t, f.Page.SetViewportSize(1280, 900))
			f.NavigateToCredentials()
			require.NoError(t, f.Page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Sign in with Device Code"}).Click())
			require.NoError(t, f.Page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Get device code"}).Click())
			f.WaitForTextIn("#openai-device-auth-dialog-content", message)
			heading := f.Page.Locator("#openai-device-auth-dialog-content p").First()
			text, err := heading.TextContent()
			require.NoError(t, err)
			require.Equal(t, message, text)
			dir := t.TempDir()
			if base := os.Getenv("E2E_OPENAI_UI_ARTIFACT_DIR"); base != "" {
				require.True(t, filepath.IsAbs(base))
				dir, err = os.MkdirTemp(base, "openai-error-"+state+"-")
				require.NoError(t, err)
			}
			for _, theme := range []string{"light", "dark"} {
				scheme := playwright.ColorSchemeLight
				if theme == "dark" {
					scheme = playwright.ColorSchemeDark
				}
				require.NoError(t, f.Page.EmulateMedia(playwright.PageEmulateMediaOptions{ColorScheme: scheme}))
				evidence, err := heading.Evaluate(`el => ({...(`+openAIDeviceTextContrast+`)(el), prefersDark:matchMedia('(prefers-color-scheme: dark)').matches})`, nil)
				require.NoError(t, err)
				data, ok := evidence.(map[string]any)
				require.True(t, ok)
				data["state"], data["route"] = state, "/ui/credentials"
				data["consoleErrors"], data["expectedFaviconErrors"] = consoleErrors.Load(), faviconErrors.Load()
				payload, err := json.MarshalIndent(data, "", "  ")
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(filepath.Join(dir, theme+".json"), payload, 0600))
				_, err = f.Page.Screenshot(playwright.PageScreenshotOptions{Path: playwright.String(filepath.Join(dir, theme+".png")), Animations: playwright.ScreenshotAnimationsDisabled})
				require.NoError(t, err)
				assert.Equal(t, theme == "dark", data["prefersDark"])
				contrast, ok := data["contrastRatio"].(float64)
				require.True(t, ok)
				assert.GreaterOrEqual(t, contrast, 4.5, "error normal text must remain readable: %s/%s", state, theme)
				assert.Zero(t, consoleErrors.Load())
				t.Logf("error_text state=%s theme=%s contrast=%f console_errors=%d expected_favicon_errors=%d", state, theme, contrast, consoleErrors.Load(), faviconErrors.Load())
			}
		})
	}
}

func TestOpenAIDeviceVisual(t *testing.T) {
	for _, viewport := range []struct {
		name          string
		width, height int
	}{
		{"desktop", 1280, 900}, {"mobile", 390, 844},
	} {
		t.Run(viewport.name, func(t *testing.T) {
			f := setup(t)
			configureOpenAIDeviceOAuthMock(t, openaitest.DeviceAuthOptions{
				UserCodeResponse: map[string]any{"device_auth_id": "fixture-device", "user_code": "FIXT-URE1", "interval": 30},
				PollResponses:    []openaitest.DevicePollResponse{{StatusCode: http.StatusForbidden, Response: map[string]any{"error": "authorization_pending"}}},
			}, nil)
			var pageErrors atomic.Int64
			f.Page.OnPageError(func(error) { pageErrors.Add(1) })
			require.NoError(t, f.Page.SetViewportSize(viewport.width, viewport.height))
			f.NavigateToCredentials()
			dir := t.TempDir()
			if base := os.Getenv("E2E_OPENAI_UI_ARTIFACT_DIR"); base != "" {
				require.True(t, filepath.IsAbs(base), "artifact directory must be absolute")
				var err error
				dir, err = os.MkdirTemp(base, "openai-"+viewport.name+"-")
				require.NoError(t, err)
			}
			snapshot := func(name string) {
				_, err := f.Page.Screenshot(playwright.PageScreenshotOptions{Path: playwright.String(filepath.Join(dir, name+".png")), Animations: playwright.ScreenshotAnimationsDisabled})
				require.NoError(t, err)
			}
			snapshot("credentials")
			require.NoError(t, f.Page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Sign in with Device Code"}).Click())
			dialog := f.Page.GetByRole("dialog", playwright.PageGetByRoleOptions{Name: "Connect OpenAI with a device code", Exact: playwright.Bool(true)})
			require.NoError(t, dialog.WaitFor())
			require.NoError(t, f.Page.Locator(`#openai-device-auth-dialog [role="dialog"][data-tui-dialog-open="true"]`).WaitFor())
			for _, text := range []string{
				"Connect OpenAI with a device code",
				"Open the verification page on another device, then enter the code shown below.",
				"Only start this login from your own Tianji session. Never share the code with anyone.",
			} {
				require.NoError(t, dialog.GetByText(text, playwright.LocatorGetByTextOptions{Exact: playwright.Bool(true)}).WaitFor())
			}
			for _, action := range []string{"Cancel", "Get device code"} {
				require.NoError(t, dialog.GetByRole("button", playwright.LocatorGetByRoleOptions{Name: action, Exact: playwright.Bool(true)}).WaitFor())
			}
			snapshot("device-initial")
			require.NoError(t, f.Page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Get device code"}).Click())
			f.WaitForTextIn("#openai-device-auth-status", "FIXT-URE1")
			for _, theme := range []string{"light", "dark"} {
				colorScheme := playwright.ColorSchemeLight
				wantColor := "oklch(0.554 0.135 66.442)"
				if theme == "dark" {
					colorScheme = playwright.ColorSchemeDark
				}
				require.NoError(t, f.Page.EmulateMedia(playwright.PageEmulateMediaOptions{ColorScheme: colorScheme}))
				snapshot("device-pending-" + theme)
				evidence, err := f.Page.Evaluate(`() => {
					const dialog = document.querySelector('[data-dialog-instance="openai-device-auth-dialog"][role="dialog"]');
					const status = document.getElementById('openai-device-auth-status');
					const warning = [...status.querySelectorAll('p')].find(el => el.textContent === 'Only enter this code on the official OpenAI verification page.');
					const style = getComputedStyle(warning);
					const {foregroundRGB, backgroundRGB, contrastRatio} = (` + openAIDeviceTextContrast + `)(warning);
					const bounds = el => { const r = el.getBoundingClientRect(); return {x:r.x,y:r.y,width:r.width,height:r.height,right:r.right,bottom:r.bottom}; };
					const refs = [...document.querySelectorAll('[role="dialog"]')].flatMap(el => ['aria-labelledby','aria-describedby'].flatMap(attr => (el.getAttribute(attr)||'').split(/\s+/).filter(Boolean)));
					return {viewport:{width:innerWidth,height:innerHeight}, dialog:bounds(dialog), status:bounds(status), warning:bounds(warning),
						foregroundRGB, backgroundRGB, contrastRatio, prefersDark:matchMedia('(prefers-color-scheme: dark)').matches,
						dialogWithinViewport:dialog.getBoundingClientRect().x >= 0 && dialog.getBoundingClientRect().y >= 0 && dialog.getBoundingClientRect().right <= innerWidth && dialog.getBoundingClientRect().bottom <= innerHeight,
						background:style.backgroundColor, border:style.borderTopColor, color:style.color, warningText:warning.textContent,
						warningClasses:warning.className, documentWidth:document.documentElement.scrollWidth,
						statusOverflow:status.scrollWidth > status.clientWidth, dialogOverflow:dialog.scrollWidth > dialog.clientWidth,
						ariaReferencesResolve:refs.length>0 && refs.every(id => document.querySelectorAll('[id="'+id+'"]').length===1),
						pollTrigger:status.getAttribute('hx-trigger'), verificationLink:status.querySelector('a').getAttribute('rel')};
				}`)
				require.NoError(t, err)
				data, ok := evidence.(map[string]any)
				require.True(t, ok)
				data["pageErrors"] = pageErrors.Load()
				data["route"] = "/ui/credentials"
				payload, err := json.MarshalIndent(data, "", "  ")
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(filepath.Join(dir, "device-pending-"+theme+".json"), payload, 0600))
				assert.NotEqual(t, "rgba(0, 0, 0, 0)", data["background"], "warning background must be compiled")
				assert.Contains(t, data["background"], " / 0.1)")
				assert.Contains(t, data["border"], " / 0.5)")
				assert.Equal(t, wantColor, data["color"])
				assert.Equal(t, theme == "dark", data["prefersDark"])
				contrastRatio, ok := data["contrastRatio"].(float64)
				require.True(t, ok)
				assert.GreaterOrEqual(t, contrastRatio, 4.5, "warning normal text must remain readable under %s media preference", theme)
				assert.Contains(t, data["warningClasses"], "border-yellow-500/50")
				assert.Contains(t, data["warningClasses"], "bg-yellow-500/10")
				assert.Contains(t, data["warningClasses"], "text-yellow-700")
				assert.NotContains(t, data["warningClasses"], "dark:text-yellow-400")
				assert.Equal(t, true, data["ariaReferencesResolve"])
				assert.Equal(t, false, data["statusOverflow"])
				assert.Equal(t, false, data["dialogOverflow"])
				assert.EqualValues(t, viewport.width, data["documentWidth"])
				assert.Equal(t, true, data["dialogWithinViewport"])
				assert.Zero(t, pageErrors.Load())
			}
		})
	}
}

func TestOpenAIDialogDisableAutoFocus(t *testing.T) {
	f := setup(t)
	configureOpenAIOAuthMock(t, openaitest.OAuthServerOptions{})
	f.NavigateToCredentials()
	require.NoError(t, f.Page.Clock().Install())
	_, err := f.Page.Evaluate(`document.querySelector('#openai-callback-url-dialog [role="dialog"]').setAttribute('data-tui-dialog-disable-autofocus', 'true')`)
	require.NoError(t, err)
	opener := f.Page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Paste callback URL", Exact: playwright.Bool(true)})
	require.NoError(t, opener.Click())
	require.NoError(t, f.Page.Clock().RunFor(500))
	focused, err := opener.Evaluate(`el => el === document.activeElement && window.tui.dialog.isOpen('openai-callback-url-dialog')`, nil)
	require.NoError(t, err)
	assert.Equal(t, true, focused, "disabled autofocus leaves opener focus unchanged")
}

// Pause native frames/timers: Escape must also dismiss the visible opening phase.
func TestOpenAIDialogOpeningEscape(t *testing.T) {
	for _, tc := range []struct {
		name, button, id string
		advance          int
	}{
		{"device_opening", "Sign in with Device Code", "openai-device-auth-dialog", 0},
		{"sidebar_opening", "Toggle Sidebar", "app-sidebar-mobile", 0},
		{"device_before_autofocus", "Sign in with Device Code", "openai-device-auth-dialog", 20},
		{"sidebar_before_autofocus", "Toggle Sidebar", "app-sidebar-mobile", 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setup(t)
			configureOpenAIOAuthMock(t, openaitest.OAuthServerOptions{})
			require.NoError(t, f.Page.SetViewportSize(390, 844))
			f.NavigateToCredentials()
			require.NoError(t, f.Page.Clock().Install(playwright.ClockInstallOptions{Time: 0}))
			require.NoError(t, f.Page.Clock().PauseAt(1000))
			opener := f.Page.GetByRole("button", playwright.PageGetByRoleOptions{Name: tc.button, Exact: playwright.Bool(true)})
			content := f.Page.Locator(`[data-tui-dialog-content][data-dialog-instance="` + tc.id + `"]`)
			require.NoError(t, opener.Click())
			opening, err := content.Evaluate(`el => !el.hasAttribute('data-tui-dialog-hidden') && el.getAttribute('data-tui-dialog-open') === 'false'`, nil)
			require.NoError(t, err)
			require.Equal(t, true, opening, "actual opener exposes the pre-frame phase")
			require.NoError(t, f.Page.Clock().RunFor(tc.advance))
			require.NoError(t, f.Page.Keyboard().Press("Escape"))
			// Advance beyond autofocus, then beyond the existing close animation.
			require.NoError(t, f.Page.Clock().RunFor(100))
			focused, err := opener.Evaluate(`el => el === document.activeElement`, nil)
			require.NoError(t, err)
			assert.Equal(t, true, focused, "closing must cancel deferred autofocus")
			require.NoError(t, f.Page.Clock().RunFor(400))
			closed, err := content.Evaluate(`el => el.hasAttribute('data-tui-dialog-hidden') && el.getAttribute('data-tui-dialog-open') === 'false' && document.body.style.overflow === '' && !window.tui.dialog.isOpen(el.getAttribute('data-dialog-instance'))`, nil)
			require.NoError(t, err)
			require.Equal(t, true, closed, "Escape must remain closed after queued opening work")
			focused, err = opener.Evaluate(`el => el === document.activeElement`, nil)
			require.NoError(t, err)
			assert.Equal(t, true, focused, "closed dialog restores the actual opener")
		})
	}
}

func TestOpenAIDialogRenderedARIA(t *testing.T) {
	f := setup(t)
	configureOpenAIOAuthMock(t, openaitest.OAuthServerOptions{})
	f.SeedUser("fixture-aria-user")
	for _, tc := range []struct {
		name, route, button, title string
		width, height              int
	}{
		{"edit_user", "/ui/users/fixture-aria-user", "Edit", "Edit User", 1280, 900},
		{"mobile_sidebar", "/ui/credentials", "Toggle Sidebar", "Navigation", 390, 844},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, f.Page.SetViewportSize(tc.width, tc.height))
			_, err := f.Page.Goto(testServer.URL + tc.route)
			require.NoError(t, err)
			require.NoError(t, f.Page.WaitForLoadState())
			opener := f.Page.GetByRole("button", playwright.PageGetByRoleOptions{Name: tc.button, Exact: playwright.Bool(true)})
			require.NoError(t, opener.Click())
			dialog := f.Page.GetByRole("dialog", playwright.PageGetByRoleOptions{Name: tc.title, Exact: playwright.Bool(true)})
			require.NoError(t, dialog.WaitFor(), "real %s click must open %s dialog", tc.button, tc.title)
			resolved, err := f.Page.Evaluate(`() => [...document.querySelectorAll('[role="dialog"]')].every(dialog =>
				['aria-labelledby','aria-describedby'].every(attr => (dialog.getAttribute(attr)||'').split(/\s+/).every(id => {
					const nodes = document.querySelectorAll('[id="'+id+'"]');
					return id && nodes.length === 1 && nodes[0].textContent.trim().length > 0;
				})))`)
			require.NoError(t, err)
			assert.Equal(t, true, resolved, "all rendered sibling dialog names/descriptions resolve")
			hiddenDescription, err := dialog.Evaluate(`el => { const description = document.getElementById(el.getAttribute('aria-describedby')); const style = getComputedStyle(description); return style.position === 'absolute' && style.width === '1px' && style.height === '1px'; }`, nil)
			require.NoError(t, err)
			assert.Equal(t, true, hiddenDescription, "screen-reader description preserves visible layout")
			require.NoError(t, f.Page.Keyboard().Press("Escape"))
			require.NoError(t, dialog.WaitFor(playwright.LocatorWaitForOptions{State: playwright.WaitForSelectorStateHidden}))
			require.Eventually(t, func() bool {
				focused, focusErr := opener.Evaluate(`el => el === document.activeElement`, nil)
				require.NoError(t, focusErr)
				return focused == true
			}, 5*time.Second, 10*time.Millisecond, "Escape restores the actual opener")
		})
	}
}

// Browser request counts and synchronous HTMX dispatch counts are separate from
// provider request counts: the server can short-circuit a terminal status POST.
func observeDeviceStatusRequests(t *testing.T, f *Fixture) *atomic.Int64 {
	t.Helper()
	count := &atomic.Int64{}
	f.Page.OnRequest(func(request playwright.Request) {
		if strings.Contains(request.URL(), "/ui/openai/device/status?") {
			count.Add(1)
		}
	})
	_, err := f.Page.Evaluate(`() => {
		window.deviceStatusDispatches = 0;
		document.addEventListener('htmx:beforeRequest', event => {
			if (event.detail.requestConfig.path.startsWith('/ui/openai/device/status?')) window.deviceStatusDispatches++;
		});
	}`)
	require.NoError(t, err)
	return count
}

func assertDevicePollingStopped(t *testing.T, f *Fixture, requests *atomic.Int64) {
	t.Helper()
	trigger, err := f.Page.Locator("#openai-device-auth-status").GetAttribute("hx-trigger")
	require.NoError(t, err)
	require.Empty(t, trigger, "terminal DOM has no polling trigger")
	before := requests.Load()
	dispatches, err := f.Page.Evaluate(`window.deviceStatusDispatches`)
	require.NoError(t, err)
	require.NoError(t, f.Page.Clock().FastForward(10000))
	after, err := f.Page.Evaluate(`window.deviceStatusDispatches`)
	require.NoError(t, err)
	assert.Equal(t, dispatches, after, "no HTMX status dispatch across ten poll intervals")
	assert.Equal(t, before, requests.Load(), "no browser status HTTP request after terminal")
}

// Fail exactly one shared read; the real handler must return 503 without
// replacing the still-valid DOM/flow. No production cache or route is changed.
type deviceUIReadFault struct {
	e2eSharedMemoryCache
	failNext atomic.Bool
}

func (c *deviceUIReadFault) GetShared(ctx context.Context, key string) ([]byte, error) {
	if c.failNext.Swap(false) {
		return nil, errors.New("fixture shared read unavailable")
	}
	return c.MemoryCache.GetShared(ctx, key)
}

func TestOpenAIDeviceLifecycle_CancelRetryAndRecover(t *testing.T) {
	f := setup(t)
	require.NoError(t, f.Page.Clock().Install())
	c := &deviceUIReadFault{e2eSharedMemoryCache: e2eSharedMemoryCache{cache.NewMemoryCache()}}
	oldCache := uiHandler.Cache
	uiHandler.Cache = c
	t.Cleanup(func() { uiHandler.Cache = oldCache })
	oauth := configureOpenAIDeviceOAuthMock(t, openaitest.DeviceAuthOptions{
		UserCodeResponse: map[string]any{"device_auth_id": "fixture-device", "user_code": "FIXT-URE1", "interval": 1},
	}, nil)
	f.NavigateToCredentials()
	requests := observeDeviceStatusRequests(t, f)
	require.NoError(t, f.Page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Sign in with Device Code"}).Click())
	require.NoError(t, f.Page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Get device code"}).Click())
	f.WaitForTextIn("#openai-device-auth-status", "FIXT-URE1")
	_, err := f.Page.Evaluate(`() => {
		window.originalDeviceStatus = document.getElementById('openai-device-auth-status');
		window.originalDeviceFlow = originalDeviceStatus.getAttribute('hx-post');
		window.originalDeviceCSRF = originalDeviceStatus.querySelector('[name="csrf_token"]').value;
	}`)
	require.NoError(t, err)
	response, err := f.Page.ExpectResponse("**/ui/openai/device/status?**", func() error { c.failNext.Store(true); return nil })
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, response.Status())
	preserved, err := f.Page.Evaluate(`document.getElementById('openai-device-auth-status') === originalDeviceStatus && originalDeviceStatus.getAttribute('hx-post') === originalDeviceFlow`)
	require.NoError(t, err)
	assert.Equal(t, true, preserved, "503 retains the original polling element and flow")
	response, err = f.Page.ExpectResponse("**/ui/openai/device/status?**", func() error { return nil })
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, response.Status())
	_, err = f.Page.WaitForFunction(`() => {
		const status = document.getElementById('openai-device-auth-status');
		return status !== originalDeviceStatus && status.getAttribute('hx-post') === originalDeviceFlow;
	}`, nil)
	require.NoError(t, err)
	require.NoError(t, f.Page.Keyboard().Press("Tab"))
	contained, err := f.Page.Evaluate(`document.querySelector('#openai-device-auth-dialog [role="dialog"]').contains(document.activeElement)`)
	require.NoError(t, err)
	assert.Equal(t, true, contained, "Tab after HTMX replacement stays in the dialog")
	assert.Equal(t, 1, countDeviceStartRequests(oauth.Requests()), "recovery must not create another flow")
	assert.GreaterOrEqual(t, countDevicePollRequests(oauth.Requests()), 1)
	require.NoError(t, f.Page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Cancel device login"}).Click())
	f.WaitForTextIn("#openai-device-auth-status", "Device login cancelled.")
	assertDevicePollingStopped(t, f, requests)
	retryScope, err := f.Page.Locator(`#openai-device-auth-status input[name="scope"]`).InputValue()
	require.NoError(t, err)
	assert.Equal(t, "global", retryScope)
	require.NoError(t, f.Page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Start a new device login"}).Click())
	f.WaitForTextIn("#openai-device-auth-status", "FIXT-URE1")
	distinct, err := f.Page.Evaluate(`() => {
		const status = document.getElementById('openai-device-auth-status');
		return status.getAttribute('hx-post') !== originalDeviceFlow && status.querySelector('[name="csrf_token"]').value === originalDeviceCSRF;
	}`)
	require.NoError(t, err)
	assert.Equal(t, true, distinct, "explicit retry creates a new flow with the same CSRF")
	assert.Equal(t, 2, countDeviceStartRequests(oauth.Requests()))
	// Closing and reopening is not an implicit retry.
	require.NoError(t, f.Page.Keyboard().Press("Escape"))
	_, err = f.Page.WaitForFunction(`document.querySelector('[data-dialog-instance="openai-device-auth-dialog"][role="dialog"]').hasAttribute('data-tui-dialog-hidden')`, nil)
	require.NoError(t, err)
	require.NoError(t, f.Page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Sign in with Device Code"}).Click())
	f.WaitForTextIn("#openai-device-auth-status", "FIXT-URE1")
	assert.Equal(t, 2, countDeviceStartRequests(oauth.Requests()))
	require.NoError(t, f.Page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Cancel device login"}).Click())
	f.WaitForTextIn("#openai-device-auth-status", "Device login cancelled.")
	assertDevicePollingStopped(t, f, requests)
}

func TestOpenAIDeviceLifecycle_LongIntervalExpiry(t *testing.T) {
	f := setup(t)
	require.NoError(t, f.Page.Clock().Install())
	oauth := configureOpenAIDeviceOAuthMock(t, openaitest.DeviceAuthOptions{
		UserCodeResponse: map[string]any{"device_auth_id": "fixture-device", "user_code": "FIXT-URE1", "interval": 90},
	}, nil)
	var consoleErrors, faviconErrors atomic.Int64
	f.Page.OnPageError(func(error) { consoleErrors.Add(1) })
	f.Page.OnConsole(func(message playwright.ConsoleMessage) {
		if message.Type() != "error" {
			return
		}
		// The existing harness protects /favicon.ico with API auth (401).
		// Ignore only that known resource error, never a device/HTMX error.
		if loc := message.Location(); loc != nil && loc.URL == testServer.URL+"/favicon.ico" && strings.Contains(message.Text(), "Failed to load resource: the server responded with a status of 401") {
			faviconErrors.Add(1)
			return
		}
		consoleErrors.Add(1)
	})
	f.NavigateToCredentials()
	requests := observeDeviceStatusRequests(t, f)
	require.NoError(t, f.Page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Sign in with Device Code"}).Click())
	require.NoError(t, f.Page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Get device code"}).Click())
	f.WaitForTextIn("#openai-device-auth-status", "FIXT-URE1")
	status := f.Page.Locator("#openai-device-auth-status")
	statusURL, err := status.GetAttribute("hx-post")
	require.NoError(t, err)
	flowID := currentQueryValue(t, statusURL, "flow_id")
	ctx := context.Background()
	store := openaioauth.NewDeviceStore(uiHandler.Cache)
	record, err := store.Get(ctx, flowID)
	require.NoError(t, err)
	require.EqualValues(t, 90, record.IntervalSeconds)
	require.Empty(t, record.OrgID)
	// Controlled server aging: browser time alone does not advance Go time.
	// Keep provider NextPollAt untouched; only the local observation is due.
	record.ExpiresAt = time.Now().Add(3 * time.Second)
	body, err := json.Marshal(record)
	require.NoError(t, err)
	require.NoError(t, uiHandler.Cache.Set(ctx, openaioauth.DeviceAuthCacheKey(flowID), body, time.Until(record.ExpiresAt)+time.Minute))
	response, err := f.Page.ExpectResponse("**/ui/openai/device/status?**", func() error { return f.Page.Clock().FastForward(90000) })
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.Status())
	_, err = f.Page.WaitForFunction(`expiry => document.querySelector('#openai-device-auth-status time').dateTime === expiry`, record.ExpiresAt.UTC().Format(time.RFC3339))
	require.NoError(t, err)
	trigger, err := status.GetAttribute("hx-trigger")
	require.NoError(t, err)
	require.Contains(t, []string{"every 1s", "every 2s", "every 3s"}, trigger, "interval_runtime_observation_bound")
	after, err := store.Get(ctx, flowID)
	require.NoError(t, err)
	assert.True(t, after.NextPollAt.Equal(record.NextPollAt), "UI observation does not shorten provider floor")
	// Let the real Go expiry pass, then the rendered HTMX timer swaps naturally.
	f.WaitForTextIn("#openai-device-auth-status", "The device code expired. Start a new device login.")
	expired, err := store.Get(ctx, flowID)
	require.ErrorIs(t, err, openaioauth.ErrDeviceAuthExpired)
	assert.Equal(t, openaioauth.DeviceAuthStatusExpired, expired.Status)
	assert.True(t, expired.ExpiresAt.Equal(record.ExpiresAt))
	assertDevicePollingStopped(t, f, requests)
	scope, err := status.Locator(`input[name="scope"]`).InputValue()
	require.NoError(t, err)
	assert.Equal(t, "global", scope)
	_, err = f.Page.Evaluate(`window.expiryRetryCSRF = document.querySelector('#openai-device-auth-status [name="csrf_token"]').value`)
	require.NoError(t, err)
	require.NoError(t, f.Page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Start a new device login"}).Click())
	f.WaitForTextIn("#openai-device-auth-status", "FIXT-URE1")
	retryURL, err := status.GetAttribute("hx-post")
	require.NoError(t, err)
	assert.NotEqual(t, statusURL, retryURL)
	retry, err := store.Get(ctx, currentQueryValue(t, retryURL, "flow_id"))
	require.NoError(t, err)
	assert.Equal(t, record.OrgID, retry.OrgID)
	sameCSRF, err := f.Page.Evaluate(`document.querySelector('#openai-device-auth-status [name="csrf_token"]').value === expiryRetryCSRF`)
	require.NoError(t, err)
	assert.Equal(t, true, sameCSRF)
	assert.Equal(t, 2, countDeviceStartRequests(oauth.Requests()))
	assert.Zero(t, countDevicePollRequests(oauth.Requests()))
	exchanges := 0
	for _, request := range oauth.Requests() {
		if request.Path == "/oauth/token" {
			exchanges++
		}
	}
	assert.Zero(t, exchanges)
	credentials, err := testDB.ListCredentials(ctx)
	require.NoError(t, err)
	assert.Zero(t, len(credentials))
	assert.Zero(t, consoleErrors.Load())
	t.Logf("interval_runtime controlled_server_age=true natural_expiry_swap=true poll_calls=%d exchange_calls=%d credential_rows=%d console_errors=%d expected_favicon_errors=%d", countDevicePollRequests(oauth.Requests()), exchanges, len(credentials), consoleErrors.Load(), faviconErrors.Load())
}

func TestOpenAIDeviceLifecycle_UnavailableRetry(t *testing.T) {
	f := setup(t)
	unavailable := configureOpenAIOAuthMock(t, openaitest.OAuthServerOptions{}) // existing mock returns 404 for device auth
	f.NavigateToCredentials()
	require.NoError(t, f.Page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Sign in with Device Code"}).Click())
	require.NoError(t, f.Page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Get device code"}).Click())
	f.WaitForTextIn("#openai-device-auth-dialog-content", "Device login is unavailable.")
	assert.Equal(t, 1, countDeviceStartRequests(unavailable.Requests()))
	fallback := f.Page.GetByRole("link", playwright.PageGetByRoleOptions{Name: "Use browser callback instead"})
	href, err := fallback.GetAttribute("href")
	require.NoError(t, err)
	assert.Equal(t, "/ui/openai/connect?scope=global", href)
	_, err = f.Page.Evaluate(`window.retryCSRF = document.querySelector('#openai-device-auth-dialog-content [name="csrf_token"]').value`)
	require.NoError(t, err)
	oauth := configureOpenAIDeviceOAuthMock(t, openaitest.DeviceAuthOptions{
		UserCodeResponse: map[string]any{"device_auth_id": "fixture-retry", "user_code": "FIXT-URE2", "interval": 30},
	}, nil)
	require.NoError(t, f.Page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Try device login again"}).Click())
	f.WaitForTextIn("#openai-device-auth-status", "FIXT-URE2")
	sameCSRF, err := f.Page.Evaluate(`document.querySelector('#openai-device-auth-status [name="csrf_token"]').value === retryCSRF`)
	require.NoError(t, err)
	assert.Equal(t, true, sameCSRF)
	assert.Equal(t, 1, countDeviceStartRequests(oauth.Requests()))
}

func countDeviceStartRequests(requests []openaitest.RecordedRequest) int {
	count := 0
	for _, request := range requests {
		if request.Path == "/api/accounts/deviceauth/usercode" {
			count++
		}
	}
	return count
}

func countDevicePollRequests(requests []openaitest.RecordedRequest) int {
	count := 0
	for _, request := range requests {
		if request.Path == "/api/accounts/deviceauth/token" {
			count++
		}
	}
	return count
}

func assertGlobalOpenAISubscriptionCredential(t *testing.T, email string) {
	t.Helper()
	credentials, err := testDB.ListCredentials(context.Background())
	require.NoError(t, err)
	for _, credential := range credentials {
		if credential.CredentialType != "openai_subscription" {
			continue
		}
		var info map[string]any
		require.NoError(t, json.Unmarshal(credential.CredentialInfo, &info))
		if info["email"] == email {
			require.Nil(t, credential.OrganizationID)
			return
		}
	}
	require.Fail(t, "global OpenAI subscription credential not found")
}

func TestOpenAICallback_FailurePagesAreReadableAndSafe(t *testing.T) {
	f := setup(t)
	configureOpenAIOAuthMock(t, openaitest.OAuthServerOptions{TokenFixtures: []openaitest.TokenFixture{{
		Code:       "fixture-authorization-code-failure",
		StatusCode: http.StatusBadRequest,
		Response: map[string]any{
			"error":             "invalid_grant",
			"error_description": "access_token=[REDACTED] refresh_token=[REDACTED]",
		},
	}}})

	for _, tc := range []struct {
		name        string
		targetURL   func(state string) string
		wantStatus  int
		wantMessage string
	}{
		{
			name: "provider_denial",
			targetURL: func(state string) string {
				return testServer.URL + "/oauth/openai/callback?state=" + state + "&error=access_denied&error_description=%3Cscript%3Edenied%3C%2Fscript%3E"
			},
			wantStatus:  http.StatusBadRequest,
			wantMessage: "access_denied",
		},
		{
			name: "invalid_state",
			targetURL: func(string) string {
				return testServer.URL + "/oauth/openai/callback?state=missing-state&code=fixture-authorization-code-success"
			},
			wantStatus:  http.StatusBadRequest,
			wantMessage: "invalid",
		},
		{
			name: "token_exchange_failure",
			targetURL: func(state string) string {
				return testServer.URL + "/oauth/openai/callback?state=" + state + "&code=fixture-authorization-code-failure"
			},
			wantStatus:  http.StatusBadGateway,
			wantMessage: "token exchange failed",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := startOpenAIConnectAndReturnState(t, f)

			resp, err := f.Page.Goto(tc.targetURL(state))
			require.NoError(t, err)
			require.Equal(t, tc.wantStatus, resp.Status())
			require.NoError(t, f.Page.WaitForLoadState())

			body := f.Text("body")
			assert.Contains(t, body, "OpenAI connection failed")
			assert.Contains(t, body, tc.wantMessage)
			assert.Contains(t, body, "Back to Credentials")

			html, err := f.Page.Locator("body").InnerHTML()
			require.NoError(t, err)
			assertOpenAIConnectDOMHasNoSecrets(t, html)
			assert.NotContains(t, html, "<script>denied</script>")
			assert.NotContains(t, html, "fixture-authorization-code-failure")
		})
	}
}

func TestOpenAIConnect_UnauthenticatedRedirectsToLogin(t *testing.T) {
	configureOpenAIOAuthMock(t, openaitest.OAuthServerOptions{})
	ctx, err := testBrowser.NewContext()
	require.NoError(t, err)
	t.Cleanup(func() { _ = ctx.Close() })
	page, err := ctx.NewPage()
	require.NoError(t, err)

	resp, err := page.Goto(testServer.URL + "/ui/openai/connect?org_id=org_unauthenticated")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Status())
	require.NoError(t, page.WaitForURL("**/ui/login"))
	assert.Contains(t, page.URL(), "/ui/login")
}

func configureOpenAIOAuthMock(t *testing.T, options openaitest.OAuthServerOptions) *openaitest.OAuthServer {
	t.Helper()
	oauthServer := openaitest.NewOAuthServer(t, options)
	oldOAuth := cfg.GeneralSettings.OpenAIOAuth
	oldProxyClient := proxyHandler.OpenAIOAuthHTTPClient
	oldUIClient := uiHandler.OpenAIOAuthHTTPClient
	cfg.GeneralSettings.OpenAIOAuth = config.OpenAIOAuthConfig{
		Enabled:      true,
		IssuerURL:    oauthServer.URL(),
		AuthorizeURL: oauthServer.AuthorizeURL(),
		TokenURL:     oauthServer.TokenURL(),
		ClientID:     "app_test",
	}
	guardedClient := openaitest.NewGuardedClient(oauthServer.Host())
	proxyHandler.OpenAIOAuthHTTPClient = guardedClient
	uiHandler.OpenAIOAuthHTTPClient = guardedClient
	t.Cleanup(func() {
		cfg.GeneralSettings.OpenAIOAuth = oldOAuth
		proxyHandler.OpenAIOAuthHTTPClient = oldProxyClient
		uiHandler.OpenAIOAuthHTTPClient = oldUIClient
	})
	return oauthServer
}

func configureOpenAIDeviceOAuthMock(t *testing.T, deviceAuth openaitest.DeviceAuthOptions, fixtures []openaitest.TokenFixture) *openaitest.OAuthServer {
	t.Helper()
	return configureOpenAIOAuthMock(t, openaitest.OAuthServerOptions{
		TokenFixtures: fixtures,
		DeviceAuth:    &deviceAuth,
	})
}

func startOpenAIConnectAndReturnState(t *testing.T, f *Fixture) string {
	t.Helper()
	f.SeedOrg(SeedOrgOpts{Alias: "openai-callback"})
	f.NavigateToCredentials()
	page, err := f.Page.Context().ExpectPage(func() error {
		return f.Page.GetByRole("link", playwright.PageGetByRoleOptions{
			Name: "Browser callback fallback",
		}).Click()
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = page.Close() })
	require.NoError(t, page.WaitForURL("**/oauth/authorize**"))
	return currentQueryValue(t, page.URL(), "state")
}

func currentQueryValue(t *testing.T, rawURL, key string) string {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	require.NoError(t, err)
	return parsed.Query().Get(key)
}

func assertOpenAIConnectDOMHasNoSecrets(t *testing.T, html string) {
	t.Helper()
	for _, forbidden := range []string{
		"[REDACTED]",
		"[REDACTED]",
		"[REDACTED]",
		"access_token",
		"refresh_token",
		"id_token",
		"code_verifier",
		"Bearer ",
		"eyJ",
		"credential_value",
		"encrypted",
		"raw upstream",
	} {
		assert.NotContains(t, html, forbidden)
	}
}
