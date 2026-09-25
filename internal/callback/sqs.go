package callback

import (
	"context"
	"encoding/json"
	"fmt"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/google/uuid"
)

// SQSLogger sends batched log data to an SQS queue.
type SQSLogger struct {
	*BatchLogger
	client   *sqs.Client
	queueURL string
}

// NewSQSLogger creates an SQS logger. Needs Queue URL (not ARN).
func NewSQSLogger(queueURL, region string) (*SQSLogger, error) {
	opts := []func(*awsconfig.LoadOptions) error{}
	if region != "" {
		opts = append(opts, awsconfig.WithRegion(region))
	}

	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("sqs logger: aws config: %w", err)
	}

	l := &SQSLogger{
		client:   sqs.NewFromConfig(cfg),
		queueURL: queueURL,
	}
	l.BatchLogger = NewBatchLogger(l.flush)
	return l, nil
}

func (l *SQSLogger) flush(batch []LogData) error {
	// SQS SendMessageBatch max 10 messages per call
	for i := 0; i < len(batch); i += 10 {
		end := i + 10
		if end > len(batch) {
			end = len(batch)
		}
		chunk := batch[i:end]

		entries := make([]types.SendMessageBatchRequestEntry, 0, len(chunk))
		for _, d := range chunk {
			body, err := json.Marshal(d)
			if err != nil {
				return fmt.Errorf("sqs marshal: %w", err)
			}
			id := uuid.New().String()
			bodyStr := string(body)
			entries = append(entries, types.SendMessageBatchRequestEntry{
				Id:          &id,
				MessageBody: &bodyStr,
			})
		}

		_, err := l.client.SendMessageBatch(context.Background(), &sqs.SendMessageBatchInput{
			QueueUrl: &l.queueURL,
			Entries:  entries,
		})
		if err != nil {
			return fmt.Errorf("sqs batch send: %w", err)
		}
	}
	return nil
}
