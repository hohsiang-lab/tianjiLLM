package secretmanager

import (
	"context"
	"fmt"
	"os"

	"cloud.google.com/go/auth/credentials"
	gcpsm "cloud.google.com/go/secretmanager/apiv1"
	smpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"google.golang.org/api/option"
)

// GoogleSecretManager reads secrets from Google Cloud Secret Manager.
type GoogleSecretManager struct {
	client    *gcpsm.Client
	projectID string
}

func init() {
	Register("google_secret_manager", newGoogleSecretManager)
}

func newGoogleSecretManager(cfg map[string]any) (SecretManager, error) {
	ctx := context.Background()
	projectID, _ := cfg["project_id"].(string)
	if projectID == "" {
		return nil, fmt.Errorf("google secret manager: project_id required")
	}

	var opts []option.ClientOption
	if credsFile, ok := cfg["credentials_file"].(string); ok && credsFile != "" {
		data, err := os.ReadFile(credsFile)
		if err != nil {
			return nil, fmt.Errorf("google secret manager: read credentials: %w", err)
		}
		creds, err := credentials.DetectDefault(&credentials.DetectOptions{
			Scopes:          []string{"https://www.googleapis.com/auth/cloud-platform"},
			CredentialsJSON: data,
		})
		if err != nil {
			return nil, fmt.Errorf("google secret manager: detect credentials: %w", err)
		}
		opts = append(opts, option.WithAuthCredentials(creds))
	}

	client, err := gcpsm.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("google secret manager client: %w", err)
	}

	return &GoogleSecretManager{
		client:    client,
		projectID: projectID,
	}, nil
}

func (g *GoogleSecretManager) Name() string { return "google_secret_manager" }

func (g *GoogleSecretManager) Get(ctx context.Context, path string) (string, error) {
	name := fmt.Sprintf("projects/%s/secrets/%s/versions/latest", g.projectID, path)

	resp, err := g.client.AccessSecretVersion(ctx, &smpb.AccessSecretVersionRequest{
		Name: name,
	})
	if err != nil {
		return "", fmt.Errorf("google secret manager get %q: %w", path, err)
	}

	return string(resp.Payload.Data), nil
}

func (g *GoogleSecretManager) Health(ctx context.Context) error {
	it := g.client.ListSecrets(ctx, &smpb.ListSecretsRequest{
		Parent:   fmt.Sprintf("projects/%s", g.projectID),
		PageSize: 1,
	})
	_, err := it.Next()
	if err != nil && err.Error() != "no more items in iterator" {
		return fmt.Errorf("google secret manager health: %w", err)
	}
	return nil
}
