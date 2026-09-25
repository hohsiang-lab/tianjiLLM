package codexapp

const (
	EnvCodexAPIKey  = "CODEX_API_KEY"
	EnvOpenAIAPIKey = "OPENAI_API_KEY"
)

type ChatGPTAuthTokens struct {
	AccessToken      string  `json:"accessToken"`
	ChatGPTAccountID string  `json:"chatgptAccountId"`
	ChatGPTPlanType  *string `json:"chatgptPlanType"`
}

type LoginStartParams struct {
	Type             string  `json:"type"`
	AccessToken      string  `json:"accessToken"`
	ChatGPTAccountID string  `json:"chatgptAccountId"`
	ChatGPTPlanType  *string `json:"chatgptPlanType"`
}

func NewChatGPTAuthTokensLogin(tokens ChatGPTAuthTokens) LoginStartParams {
	return LoginStartParams{
		Type:             "chatgptAuthTokens",
		AccessToken:      tokens.AccessToken,
		ChatGPTAccountID: tokens.ChatGPTAccountID,
		ChatGPTPlanType:  tokens.ChatGPTPlanType,
	}
}

func StdioEnvForProfile(env map[string]string, subscriptionProfile bool) map[string]string {
	cleaned := make(map[string]string, len(env))
	for key, value := range env {
		if subscriptionProfile && (key == EnvCodexAPIKey || key == EnvOpenAIAPIKey) {
			continue
		}
		cleaned[key] = value
	}
	return cleaned
}

func WebSocketConnectionHeaders(headers map[string]string, appServerAuthToken string) map[string]string {
	out := make(map[string]string, len(headers)+1)
	for key, value := range headers {
		if key == "Authorization" || key == EnvOpenAIAPIKey || key == EnvCodexAPIKey {
			continue
		}
		out[key] = value
	}
	if appServerAuthToken != "" {
		out["Authorization"] = "Bearer " + appServerAuthToken
	}
	return out
}
