package pages_test

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/praxisllmlab/tianjiLLM/internal/ui/components/dialog"
	"github.com/praxisllmlab/tianjiLLM/internal/ui/pages"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
)

func TestOpenAIDialogRawIDAttributes(t *testing.T) {
	for _, tc := range []struct {
		name, instance, id, titleID, descriptionID string
	}{
		{"default", "fixture-dialog", "", "fixture-dialog-title", "fixture-dialog-description"},
		{"explicit", "fixture-dialog", "explicit", "explicit-title", "explicit-description"},
		{"standalone", "", "", "", ""},
		{"standalone-explicit", "", "explicit", "explicit-title", "explicit-description"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			titleID, descriptionID := "", ""
			if tc.id != "" {
				titleID, descriptionID = tc.id+"-title", tc.id+"-description"
			}
			children := templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
				if err := dialog.Title(dialog.TitleProps{ID: titleID}).Render(ctx, w); err != nil {
					return err
				}
				return dialog.Description(dialog.DescriptionProps{ID: descriptionID}).Render(ctx, w)
			})
			var rendered strings.Builder
			if tc.instance == "" {
				require.NoError(t, children.Render(t.Context(), &rendered))
			} else {
				require.NoError(t, dialog.Dialog(dialog.Props{ID: tc.instance}).Render(templ.WithChildren(t.Context(), children), &rendered))
			}
			z := html.NewTokenizer(strings.NewReader(rendered.String()))
			seen := 0
			for z.Next() != html.ErrorToken {
				token := z.Token()
				if token.Type != html.StartTagToken || (token.Data != "h2" && token.Data != "p") {
					continue
				}
				seen++
				var ids []string
				// Inspect raw tokens: a DOM or attribute map can hide duplicate IDs.
				for _, attr := range token.Attr {
					assert.NotEqual(t, "else", attr.Key, "no stray control-flow attribute")
					if attr.Key == "id" {
						ids = append(ids, attr.Val)
					}
				}
				want := tc.titleID
				if token.Data == "p" {
					want = tc.descriptionID
				}
				if want == "" {
					assert.Empty(t, ids, "standalone without an explicit ID")
				} else {
					assert.Equal(t, []string{want}, ids, "exactly one explicit-or-instance ID")
				}
			}
			require.ErrorIs(t, z.Err(), io.EOF)
			assert.Equal(t, 2, seen, "both title and description were inspected")
		})
	}
}

func TestOpenAIDialogSiblingARIAReferences(t *testing.T) {
	for name, component := range map[string]templ.Component{
		"credentials": pages.CredentialsPage(pages.CredentialsPageData{ConnectURL: "/ui/openai/connect?scope=global"}),
		"users":       pages.UsersPage(pages.UsersPageData{Users: []pages.UserRow{{UserID: "fixture-user"}}}),
		"user_detail": pages.UserDetailPage(pages.UserDetailData{
			User:       pages.UserRow{UserID: "fixture-user", UserEmail: "fixture@example.test"},
			Identities: []pages.UserIdentityRow{{IdentityID: "fixture-identity", Provider: "fixture"}},
		}),
		"models":     pages.ModelsPage(pages.ModelsPageData{DBAvailable: true}),
		"edit_model": pages.EditModelForm(pages.ModelRow{}),
		"keys":       pages.KeysPage(pages.KeysPageData{}),
		"key_detail": pages.KeyDetailPage(pages.KeyDetailData{Token: "fixture-not-a-secret-token"}),
		"key_reveal": pages.KeyRevealDialog("fixture-not-a-key"),
	} {
		t.Run(name, func(t *testing.T) {
			z := html.NewTokenizer(strings.NewReader(renderToString(t, component)))
			ids := map[string]int{}
			var references []string
			for {
				kind := z.Next()
				if kind == html.ErrorToken {
					require.ErrorIs(t, z.Err(), io.EOF)
					break
				}
				if kind != html.StartTagToken && kind != html.SelfClosingTagToken {
					continue
				}
				token := z.Token()
				attrs := map[string]string{}
				for _, attr := range token.Attr {
					attrs[attr.Key] = attr.Val
				}
				if id := attrs["id"]; id != "" {
					ids[id]++
				}
				if attrs["role"] == "dialog" {
					for _, attr := range []string{"aria-labelledby", "aria-describedby"} {
						require.NotEmpty(t, attrs[attr])
						references = append(references, strings.Fields(attrs[attr])...)
					}
				}
			}
			require.NotEmpty(t, references, "consumer must render its dialogs")
			for _, ref := range references {
				assert.Equal(t, 1, ids[ref], "dialog reference %q must resolve exactly once", ref)
			}
		})
	}
}

func TestOpenAIDeviceAuthWithoutCallbackHidesConnectControls(t *testing.T) {
	for _, status := range []string{"failed", "cancelled"} {
		html := renderToString(t, pages.OpenAIDeviceAuthStatus(pages.OpenAIDeviceAuthView{Status: status}))
		assert.False(t, strings.Contains(html, "/ui/openai/device/start"))
		assert.False(t, strings.Contains(html, "Use browser callback instead"))
	}
}

func TestOpenAIDeviceAuthEscapesDisplayValues(t *testing.T) {
	html := renderToString(t, pages.OpenAIDeviceAuthStatus(pages.OpenAIDeviceAuthView{
		Status:          "pending",
		VerificationURI: "javascript:alert(1)",
		UserCode:        "<script>alert(1)</script>",
		ErrorMessage:    "<script>status</script>",
		IntervalSeconds: 5,
		Polling:         true,
	}))

	assert.NotContains(t, html, "<script>alert(1)</script>")
	assert.NotContains(t, html, "<script>status</script>")
	assert.NotContains(t, html, "javascript:")
	assert.Contains(t, html, "&lt;script&gt;")
}

func TestOpenAIDeviceAuthSlowDownShowsRetryState(t *testing.T) {
	html := renderToString(t, pages.OpenAIDeviceAuthStatus(pages.OpenAIDeviceAuthView{
		Status:          "pending",
		ErrorMessage:    "OpenAI asked Tianji to slow down before the next check.",
		IntervalSeconds: 10,
		Polling:         true,
	}))

	assert.Contains(t, html, "OpenAI asked Tianji to slow down")
	assert.Contains(t, html, `hx-trigger="every 10s"`)
	assert.Contains(t, html, `aria-busy="true"`)
}

func TestOpenAIDeviceAuthDeniedStopsPollingAndOffersFallback(t *testing.T) {
	html := renderToString(t, pages.OpenAIDeviceAuthStatus(pages.OpenAIDeviceAuthView{
		Status:       "denied",
		ErrorMessage: "OpenAI declined this device authorization.",
		CallbackURL:  "/ui/openai/connect?scope=global",
		Polling:      false,
	}))

	assert.Contains(t, html, "OpenAI declined this device authorization")
	assert.Contains(t, html, `role="alert"`)
	assert.Contains(t, html, `aria-live="assertive"`)
	assert.Contains(t, html, `href="/ui/openai/connect?scope=global"`)
	assert.Contains(t, html, `target="_blank"`)
	assert.NotContains(t, html, `hx-post="/ui/openai/device/status`)
	assert.NotContains(t, html, "hx-trigger=")
}

func TestCredentialsOpenAIDeviceLoginIsPrimaryAndKeepsCallbackFallback(t *testing.T) {
	html := renderToString(t, pages.CredentialsPage(pages.CredentialsPageData{
		ConnectURL: "/ui/openai/connect?scope=global",
	}))

	assert.Contains(t, html, "Sign in with Device Code")
	assert.Contains(t, html, `hx-post="/ui/openai/device/start"`)
	assert.Contains(t, html, "Paste callback URL")
	assert.Contains(t, html, "Browser callback fallback")
	assert.Contains(t, html, `hx-post="/ui/openai/callback-url"`)
}

func TestOpenAIDeviceAuthPendingRendersSafePollingView(t *testing.T) {
	html := renderToString(t, pages.OpenAIDeviceAuthStarted(pages.OpenAIDeviceAuthView{
		FlowID:          "flow_opaque",
		VerificationURI: "https://auth.example.test/codex/device?x=1&y=2",
		UserCode:        "ABCD-EFGH",
		StatusURL:       "/ui/openai/device/status?flow_id=flow_opaque",
		CancelURL:       "/ui/openai/device/cancel",
		StatusValues:    `{"csrf_token":"csrf-value"}`,
		ExpiresAt:       "2026-09-08T00:15:00Z",
		IntervalSeconds: 5,
		Status:          "pending",
		Polling:         true,
	}))

	assert.Contains(t, html, `href="https://auth.example.test/codex/device?x=1&amp;y=2"`)
	assert.Contains(t, html, "ABCD-EFGH")
	assert.Contains(t, html, `hx-post="/ui/openai/device/status?flow_id=flow_opaque"`)
	assert.Contains(t, html, `hx-vals="{&#34;csrf_token&#34;:&#34;csrf-value&#34;}"`)
	assert.Contains(t, html, `hx-trigger="every 5s"`)
	assert.Contains(t, html, `aria-busy="true"`)
	assert.Contains(t, html, `role="status"`)
	assert.Contains(t, html, "Only enter this code on the official OpenAI verification page.")
	assert.Contains(t, html, "border-yellow-500/50 bg-yellow-500/10")
	assert.Contains(t, html, "text-yellow-700")
	assert.NotContains(t, html, "dark:text-yellow-400")
	assert.Contains(t, html, `class="block select-all`)
	assert.Contains(t, html, `datetime="2026-09-08T00:15:00Z"`)
	assert.NotContains(t, html, "device_auth_id")
}

func TestOpenAIDeviceAuthSuccessStopsPollingAndEscapesCredentialID(t *testing.T) {
	html := renderToString(t, pages.OpenAIDeviceAuthStatus(pages.OpenAIDeviceAuthView{
		Status:       "success",
		CredentialID: `<script>alert("x")</script>`,
		Polling:      false,
	}))

	assert.Contains(t, html, "OpenAI credential connected")
	assert.Contains(t, html, `role="status"`)
	assert.Contains(t, html, `aria-live="polite"`)
	assert.NotContains(t, html, "hx-get=")
	assert.NotContains(t, html, `hx-post="/ui/openai/device/status`)
	assert.NotContains(t, html, `<script>alert`)
}

func TestOpenAIDeviceAuthErrorOffersScopedCallbackFallback(t *testing.T) {
	html := renderToString(t, pages.OpenAIDeviceAuthError(pages.OpenAIDeviceAuthView{
		ErrorMessage: "Device-code login is not enabled for this OpenAI account or workspace.",
		CallbackURL:  "/ui/openai/connect?org_id=org%2F123",
	}))

	assert.Contains(t, html, "Device-code login is not enabled")
	assert.Contains(t, html, `href="/ui/openai/connect?org_id=org%2F123"`)
	assert.Contains(t, html, `target="_blank"`)
}

func TestOpenAIDeviceAuthCancelledOffersRetry(t *testing.T) {
	html := renderToString(t, pages.OpenAIDeviceAuthStatus(pages.OpenAIDeviceAuthView{
		Status:         "cancelled",
		CSRFToken:      "csrf-value",
		OrganizationID: "org-123",
		CallbackURL:    "/ui/openai/connect?org_id=org-123",
		Polling:        false,
	}))

	assert.Contains(t, html, "Device login cancelled")
	assert.Contains(t, html, "Start a new device login")
	assert.Contains(t, html, `name="org_id" value="org-123"`)
	assert.Contains(t, html, `name="csrf_token" value="csrf-value"`)
}
