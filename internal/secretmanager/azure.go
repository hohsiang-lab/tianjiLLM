package secretmanager

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
)

// AzureKeyVault reads secrets from Azure Key Vault.
type AzureKeyVault struct {
	client *azsecrets.Client
}

func init() {
	Register("azure_key_vault", newAzureKeyVault)
}

func newAzureKeyVault(cfg map[string]any) (SecretManager, error) {
	vaultURL, _ := cfg["vault_url"].(string)
	if vaultURL == "" {
		return nil, fmt.Errorf("azure key vault: vault_url required")
	}
	// No trailing slash
	vaultURL = strings.TrimRight(vaultURL, "/")

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("azure credential: %w", err)
	}

	client, err := azsecrets.NewClient(vaultURL, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("azure key vault client: %w", err)
	}

	return &AzureKeyVault{client: client}, nil
}

func (a *AzureKeyVault) Name() string { return "azure_key_vault" }

func (a *AzureKeyVault) Get(ctx context.Context, path string) (string, error) {
	resp, err := a.client.GetSecret(ctx, path, "", nil)
	if err != nil {
		return "", fmt.Errorf("azure key vault get %q: %w", path, err)
	}
	if resp.Value == nil {
		return "", fmt.Errorf("azure key vault %q: nil value", path)
	}
	return *resp.Value, nil
}

func (a *AzureKeyVault) Health(ctx context.Context) error {
	pager := a.client.NewListSecretPropertiesPager(nil)
	if pager.More() {
		_, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("azure key vault health: %w", err)
		}
	}
	return nil
}
