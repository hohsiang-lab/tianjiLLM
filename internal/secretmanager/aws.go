package secretmanager

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

// AWSSecretManager reads secrets from AWS Secrets Manager.
type AWSSecretManager struct {
	client *secretsmanager.Client
}

func init() {
	Register("aws_secrets_manager", newAWSSecretManager)
}

func newAWSSecretManager(cfg map[string]any) (SecretManager, error) {
	ctx := context.Background()
	opts := []func(*awsconfig.LoadOptions) error{}

	if region, ok := cfg["region"].(string); ok && region != "" {
		opts = append(opts, awsconfig.WithRegion(region))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}

	return &AWSSecretManager{
		client: secretsmanager.NewFromConfig(awsCfg),
	}, nil
}

func (a *AWSSecretManager) Name() string { return "aws_secrets_manager" }

func (a *AWSSecretManager) Get(ctx context.Context, path string) (string, error) {
	out, err := a.client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(path),
	})
	if err != nil {
		return "", fmt.Errorf("aws secrets manager get %q: %w", path, err)
	}

	// Check SecretString first, then SecretBinary
	if out.SecretString != nil {
		return *out.SecretString, nil
	}
	if out.SecretBinary != nil {
		return base64.StdEncoding.EncodeToString(out.SecretBinary), nil
	}
	return "", fmt.Errorf("aws secrets manager %q: empty secret", path)
}

func (a *AWSSecretManager) Health(ctx context.Context) error {
	_, err := a.client.ListSecrets(ctx, &secretsmanager.ListSecretsInput{
		MaxResults: aws.Int32(1),
	})
	if err != nil {
		return fmt.Errorf("aws secrets manager health: %w", err)
	}
	return nil
}
