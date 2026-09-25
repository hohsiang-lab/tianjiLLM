//go:build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogoutButtonEndsBrowserSession(t *testing.T) {
	f := setup(t)

	logoutButton := f.Page.Locator(`form[action="/ui/logout"] button[type="submit"]`)
	count, err := logoutButton.Count()
	require.NoError(t, err)
	require.Equal(t, 1, count, "the visible logout control must submit its form")

	require.NoError(t, logoutButton.Click())
	require.NoError(t, f.Page.WaitForURL("**/ui/login"))

	cookies, err := f.Page.Context().Cookies(testServer.URL)
	require.NoError(t, err)
	for _, cookie := range cookies {
		assert.NotEqual(t, "tianji_session", cookie.Name, "the browser session cookie must be removed")
	}

	_, err = f.Page.Goto(testServer.URL + "/ui/")
	require.NoError(t, err)
	require.NoError(t, f.Page.WaitForURL("**/ui/login"))
	assert.Contains(t, f.Page.URL(), "/ui/login")
}
