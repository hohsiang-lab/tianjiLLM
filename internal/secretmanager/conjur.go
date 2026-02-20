package secretmanager

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ConjurSecretManager retrieves secrets from CyberArk Conjur.
type ConjurSecretManager struct {
	account string
	baseURL string
	apiKey  string
	login   string
	client  *http.Client
}

func init() {
	Register("cyberark_conjur", func(cfg map[string]any) (SecretManager, error) {
		account, _ := cfg["conjur_account"].(string)
		url, _ := cfg["conjur_url"].(string)
		login, _ := cfg["conjur_login"].(string)
		apiKey, _ := cfg["conjur_api_key"].(string)

		if account == "" || url == "" {
			return nil, fmt.Errorf("conjur requires conjur_account and conjur_url")
		}

		return &ConjurSecretManager{
			account: account,
			baseURL: strings.TrimRight(url, "/"),
			apiKey:  apiKey,
			login:   login,
			client:  &http.Client{Timeout: 10 * time.Second},
		}, nil
	})
}

func (c *ConjurSecretManager) Name() string { return "cyberark_conjur" }

func (c *ConjurSecretManager) Get(ctx context.Context, path string) (string, error) {
	url := fmt.Sprintf("%s/secrets/%s/variable/%s", c.baseURL, c.account, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	token, err := c.authenticate(ctx)
	if err != nil {
		return "", fmt.Errorf("conjur auth: %w", err)
	}
	req.Header.Set("Authorization", "Token token=\""+token+"\"")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("conjur: %s (status %d)", path, resp.StatusCode)
	}

	buf := make([]byte, 32*1024)
	n, _ := resp.Body.Read(buf)
	return string(buf[:n]), nil
}

func (c *ConjurSecretManager) Health(ctx context.Context) error {
	url := c.baseURL + "/health"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("conjur health: status %d", resp.StatusCode)
	}
	return nil
}

func (c *ConjurSecretManager) authenticate(ctx context.Context) (string, error) {
	url := fmt.Sprintf("%s/authn/%s/%s/authenticate", c.baseURL, c.account, c.login)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(c.apiKey))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "text/plain")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("conjur auth failed: status %d", resp.StatusCode)
	}

	buf := make([]byte, 32*1024)
	n, _ := resp.Body.Read(buf)
	return string(buf[:n]), nil
}
