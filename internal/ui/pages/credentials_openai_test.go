package pages_test

import (
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/ui/pages"
	"github.com/stretchr/testify/assert"
)

func TestCredentialsPageHidesCredentialConnectFormsWhenUnavailable(t *testing.T) {
	html := renderToString(t, pages.CredentialsPage(pages.CredentialsPageData{
		ConnectDisabledReason: "Database is not configured",
	}))

	assert.NotContains(t, html, "openai-device-auth-dialog")
	assert.NotContains(t, html, "openai-callback-url-dialog")
	assert.Contains(t, html, "Database is not configured")
}

func TestCredentialsOpenAIConnectLaunch_OpensAuthorizeInNewTab(t *testing.T) {
	html := renderToString(t, pages.CredentialsPage(pages.CredentialsPageData{
		ConnectURL: "/ui/openai/connect?org_id=org_123",
	}))

	assert.Contains(t, html, `href="/ui/openai/connect?org_id=org_123"`)
	assert.Contains(t, html, `target="_blank"`)
	assert.Contains(t, html, `rel="noopener"`)
}

func TestCredentialsOpenAIPasteCallbackModal_RendersProtectedSubmitForm(t *testing.T) {
	html := renderToString(t, pages.CredentialsPage(pages.CredentialsPageData{
		ConnectURL: "/ui/openai/connect?org_id=org_123",
	}))

	assert.Contains(t, html, `role="dialog"`)
	assert.Contains(t, html, `aria-modal="true"`)
	assert.Contains(t, html, `aria-labelledby="openai-callback-url-dialog-title"`)
	assert.Contains(t, html, `aria-describedby="openai-callback-url-dialog-description"`)
	assert.Contains(t, html, "Paste callback URL")
	assert.Contains(t, html, `hx-post="/ui/openai/callback-url"`)
	assert.Contains(t, html, `name="callback_url"`)
	assert.Contains(t, html, `name="csrf_token"`)
	assert.NotContains(t, html, "code-secret")
	assert.NotContains(t, html, "state-secret")
}

func TestCredentialsPageSwitchesDisabledCredentialActionToEnable(t *testing.T) {
	html := renderToString(t, pages.CredentialsPage(pages.CredentialsPageData{
		Rows: []pages.CredentialRow{
			{
				ID:                    "cred-active",
				Name:                  "Active",
				CredentialStatus:      "Active",
				CredentialVariant:     "default",
				LifecycleAction:       "disable",
				LifecycleActionLabel:  "Disable",
				LifecycleActionPrompt: "Disable this OpenAI subscription credential?",
			},
			{
				ID:                   "cred-disabled",
				Name:                 "Disabled",
				CredentialStatus:     "Disabled",
				CredentialVariant:    "destructive",
				LifecycleAction:      "enable",
				LifecycleActionLabel: "Enable",
			},
		},
	}))

	assert.Contains(t, html, `hx-post="/ui/credentials/cred-active/disable"`)
	assert.Contains(t, html, `hx-post="/ui/credentials/cred-disabled/enable"`)
	assert.NotContains(t, html, `hx-post="/ui/credentials/cred-disabled/disable"`)
}

func TestCredentialDetailSwitchesDisabledCredentialActionToEnable(t *testing.T) {
	html := renderToString(t, pages.CredentialDetailContent(pages.CredentialDetailData{
		ID:                   "cred-disabled",
		Name:                 "Disabled",
		CredentialStatus:     "Disabled",
		CredentialVariant:    "destructive",
		LifecycleAction:      "enable",
		LifecycleActionLabel: "Enable",
	}))

	assert.Contains(t, html, `hx-post="/ui/credentials/cred-disabled/enable"`)
	assert.NotContains(t, html, `hx-post="/ui/credentials/cred-disabled/disable"`)
	assert.Contains(t, html, ">Enable<")
}
