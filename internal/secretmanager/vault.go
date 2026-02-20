package secretmanager

import (
	"context"
	"fmt"
	"os"

	vaultapi "github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/api/auth/approle"
)

// HashiCorpVault reads secrets from HashiCorp Vault KV v2.
type HashiCorpVault struct {
	client *vaultapi.Client
	mount  string
}

func init() {
	Register("hashicorp_vault", newHashiCorpVault)
}

func newHashiCorpVault(cfg map[string]any) (SecretManager, error) {
	vaultCfg := vaultapi.DefaultConfig()

	if addr, ok := cfg["vault_url"].(string); ok && addr != "" {
		vaultCfg.Address = addr
	}

	client, err := vaultapi.NewClient(vaultCfg)
	if err != nil {
		return nil, fmt.Errorf("vault client: %w", err)
	}

	// Auth: token or approle
	if token, ok := cfg["token"].(string); ok && token != "" {
		client.SetToken(token)
	} else if token := os.Getenv("VAULT_TOKEN"); token != "" {
		client.SetToken(token)
	} else if roleID, ok := cfg["role_id"].(string); ok && roleID != "" {
		secretID, _ := cfg["secret_id"].(string)
		auth, err := approle.NewAppRoleAuth(roleID, &approle.SecretID{FromString: secretID})
		if err != nil {
			return nil, fmt.Errorf("vault approle: %w", err)
		}
		_, err = client.Auth().Login(context.Background(), auth)
		if err != nil {
			return nil, fmt.Errorf("vault login: %w", err)
		}
	}

	mount := "secret"
	if m, ok := cfg["mount"].(string); ok && m != "" {
		mount = m
	}

	return &HashiCorpVault{
		client: client,
		mount:  mount,
	}, nil
}

func (h *HashiCorpVault) Name() string { return "hashicorp_vault" }

func (h *HashiCorpVault) Get(ctx context.Context, path string) (string, error) {
	kv := h.client.KVv2(h.mount)
	secret, err := kv.Get(ctx, path)
	if err != nil {
		return "", fmt.Errorf("vault get %q: %w", path, err)
	}

	// Return the first value if path points to a single key,
	// or the "value" key if it exists
	if val, ok := secret.Data["value"]; ok {
		return fmt.Sprint(val), nil
	}

	// If only one key, return its value
	if len(secret.Data) == 1 {
		for _, v := range secret.Data {
			return fmt.Sprint(v), nil
		}
	}

	return "", fmt.Errorf("vault %q: ambiguous secret with %d keys, expected 'value' key", path, len(secret.Data))
}

func (h *HashiCorpVault) Health(ctx context.Context) error {
	health, err := h.client.Sys().HealthWithContext(ctx)
	if err != nil {
		return fmt.Errorf("vault health: %w", err)
	}
	if !health.Initialized {
		return fmt.Errorf("vault not initialized")
	}
	if health.Sealed {
		return fmt.Errorf("vault is sealed")
	}
	return nil
}
