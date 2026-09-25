package spend

import (
	"bytes"
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Backend writes archived spend logs to AWS S3.
type S3Backend struct {
	Client *s3.Client
	Bucket string
}

func (b *S3Backend) Name() string { return "s3" }

func (b *S3Backend) Upload(ctx context.Context, key string, data []byte) (string, error) {
	_, err := b.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(b.Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String("application/json"),
	})
	if err != nil {
		return "", fmt.Errorf("s3 put: %w", err)
	}
	return fmt.Sprintf("s3://%s/%s", b.Bucket, key), nil
}
