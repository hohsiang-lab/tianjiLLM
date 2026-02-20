package callback

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
)

// DynamoDBLogger writes batched log data to DynamoDB.
type DynamoDBLogger struct {
	*BatchLogger
	client    *dynamodb.Client
	tableName string
}

// NewDynamoDBLogger creates a DynamoDB logger.
func NewDynamoDBLogger(tableName, region string) (*DynamoDBLogger, error) {
	opts := []func(*awsconfig.LoadOptions) error{}
	if region != "" {
		opts = append(opts, awsconfig.WithRegion(region))
	}

	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("dynamodb logger: aws config: %w", err)
	}

	l := &DynamoDBLogger{
		client:    dynamodb.NewFromConfig(cfg),
		tableName: tableName,
	}
	l.BatchLogger = NewBatchLogger(l.flush)
	return l, nil
}

// dynamoLogEntry is the DynamoDB item structure.
type dynamoLogEntry struct {
	ID               string  `dynamodbav:"id"`
	Model            string  `dynamodbav:"model"`
	Provider         string  `dynamodbav:"provider"`
	StartTime        string  `dynamodbav:"start_time"`
	EndTime          string  `dynamodbav:"end_time"`
	LatencyMs        int64   `dynamodbav:"latency_ms"`
	PromptTokens     int     `dynamodbav:"prompt_tokens"`
	CompletionTokens int     `dynamodbav:"completion_tokens"`
	TotalTokens      int     `dynamodbav:"total_tokens"`
	Cost             float64 `dynamodbav:"cost"`
	UserID           string  `dynamodbav:"user_id,omitempty"`
	TeamID           string  `dynamodbav:"team_id,omitempty"`
	Error            string  `dynamodbav:"error,omitempty"`
}

func (l *DynamoDBLogger) flush(batch []LogData) error {
	// DynamoDB BatchWriteItem max 25 items per call
	for i := 0; i < len(batch); i += 25 {
		end := i + 25
		if end > len(batch) {
			end = len(batch)
		}
		chunk := batch[i:end]

		requests := make([]types.WriteRequest, 0, len(chunk))
		for _, d := range chunk {
			entry := dynamoLogEntry{
				ID:               uuid.New().String(),
				Model:            d.Model,
				Provider:         d.Provider,
				StartTime:        d.StartTime.Format(time.RFC3339),
				EndTime:          d.EndTime.Format(time.RFC3339),
				LatencyMs:        d.Latency.Milliseconds(),
				PromptTokens:     d.PromptTokens,
				CompletionTokens: d.CompletionTokens,
				TotalTokens:      d.TotalTokens,
				Cost:             d.Cost,
				UserID:           d.UserID,
				TeamID:           d.TeamID,
			}
			if d.Error != nil {
				entry.Error = d.Error.Error()
			}

			item, err := attributevalue.MarshalMap(entry)
			if err != nil {
				return fmt.Errorf("dynamodb marshal: %w", err)
			}
			requests = append(requests, types.WriteRequest{
				PutRequest: &types.PutRequest{Item: item},
			})
		}

		_, err := l.client.BatchWriteItem(context.Background(), &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{
				l.tableName: requests,
			},
		})
		if err != nil {
			return fmt.Errorf("dynamodb batch write: %w", err)
		}
	}
	return nil
}

// Ensure uuid is used
var _ = aws.String
