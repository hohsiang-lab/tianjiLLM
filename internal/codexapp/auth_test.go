package codexapp

import (
	"encoding/json"
	"testing"
)

func TestCodexAppServerResolution_UsesChatGPTAuthTokensLogin(t *testing.T) {
	plan := "plus"
	params := NewChatGPTAuthTokensLogin(ChatGPTAuthTokens{
		AccessToken:      "access-secret",
		ChatGPTAccountID: "acct_123",
		ChatGPTPlanType:  &plan,
	})

	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}

	if got := string(raw); got != `{"type":"chatgptAuthTokens","accessToken":"access-secret","chatgptAccountId":"acct_123","chatgptPlanType":"plus"}` {
		t.Fatalf("login payload = %s", got)
	}
}

func TestCodexAppServerEnv_StripsOpenAIKeysForSubscriptionProfile(t *testing.T) {
	env := map[string]string{
		"PATH":           "/usr/bin",
		"CODEX_API_KEY":  "codex-key",
		"OPENAI_API_KEY": "openai-key",
	}

	cleaned := StdioEnvForProfile(env, true)

	if _, ok := cleaned["CODEX_API_KEY"]; ok {
		t.Fatal("CODEX_API_KEY should be stripped for subscription profiles")
	}
	if _, ok := cleaned["OPENAI_API_KEY"]; ok {
		t.Fatal("OPENAI_API_KEY should be stripped for subscription profiles")
	}
	if cleaned["PATH"] != "/usr/bin" {
		t.Fatalf("PATH should be preserved, got %q", cleaned["PATH"])
	}
}

func TestCodexAppServerWebSocket_ConnectionAuthSeparatedFromOpenAIAuth(t *testing.T) {
	headers := WebSocketConnectionHeaders(map[string]string{
		"X-Client":       "tianji",
		"Authorization":  "Bearer openai-account-token",
		"OPENAI_API_KEY": "openai-key",
	}, "app-server-token")

	if headers["Authorization"] != "Bearer app-server-token" {
		t.Fatalf("Authorization should carry app-server auth only, got %q", headers["Authorization"])
	}
	if _, ok := headers["OPENAI_API_KEY"]; ok {
		t.Fatal("OPENAI_API_KEY should not be sent as WebSocket account auth")
	}
	if headers["X-Client"] != "tianji" {
		t.Fatalf("X-Client should be preserved, got %q", headers["X-Client"])
	}
}
