package callback

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

// S3Logger writes batched log data to S3 as JSON.
type S3Logger struct {
	*BatchLogger
	client *s3.Client
	bucket string
	prefix string
}

// NewS3Logger creates an S3 logger with the given bucket and prefix.
func NewS3Logger(bucket, prefix, region string) (*S3Logger, error) {
	opts := []func(*awsconfig.LoadOptions) error{}
	if region != "" {
		opts = append(opts, awsconfig.WithRegion(region))
	}

	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("s3 logger: aws config: %w", err)
	}

	l := &S3Logger{
		client: s3.NewFromConfig(cfg),
		bucket: bucket,
		prefix: prefix,
	}
	l.BatchLogger = NewBatchLogger(l.flush)
	return l, nil
}

func (l *S3Logger) flush(batch []LogData) error {
	data, err := json.Marshal(batch)
	if err != nil {
		return fmt.Errorf("s3 marshal: %w", err)
	}

	now := time.Now().UTC()
	key := fmt.Sprintf("%s/%s/%d-%s.json",
		l.prefix, now.Format("2006-01-02"), now.UnixMilli(), uuid.New().String()[:8])

	_, err = l.client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(l.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String("application/json"),
	})
	if err != nil {
		return fmt.Errorf("s3 put: %w", err)
	}
	return nil
}
