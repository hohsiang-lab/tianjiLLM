package callback

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cloud.google.com/go/storage"
	"github.com/google/uuid"
)

// GCSLogger writes batched log data to Google Cloud Storage as JSON.
type GCSLogger struct {
	*BatchLogger
	client *storage.Client
	bucket string
	prefix string
}

// NewGCSLogger creates a GCS logger.
func NewGCSLogger(bucket, prefix string) (*GCSLogger, error) {
	client, err := storage.NewClient(context.Background())
	if err != nil {
		return nil, fmt.Errorf("gcs logger: client: %w", err)
	}

	l := &GCSLogger{
		client: client,
		bucket: bucket,
		prefix: prefix,
	}
	l.BatchLogger = NewBatchLogger(l.flush)
	return l, nil
}

func (l *GCSLogger) flush(batch []LogData) error {
	data, err := json.Marshal(batch)
	if err != nil {
		return fmt.Errorf("gcs marshal: %w", err)
	}

	now := time.Now().UTC()
	name := fmt.Sprintf("%s/%s/%d-%s.json",
		l.prefix, now.Format("2006-01-02"), now.UnixMilli(), uuid.New().String()[:8])

	ctx := context.Background()
	w := l.client.Bucket(l.bucket).Object(name).NewWriter(ctx)
	w.ContentType = "application/json"

	if _, err := w.Write(data); err != nil {
		w.Close()
		return fmt.Errorf("gcs write: %w", err)
	}
	// Close() is the actual upload — must check error
	if err := w.Close(); err != nil {
		return fmt.Errorf("gcs close: %w", err)
	}
	return nil
}
