package callback

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/google/uuid"
)

// AzureBlobLogger writes batched log data to Azure Blob Storage as JSON.
type AzureBlobLogger struct {
	*BatchLogger
	client    *azblob.Client
	container string
	prefix    string
}

// NewAzureBlobLogger creates an Azure Blob logger.
func NewAzureBlobLogger(accountURL, container, prefix string) (*AzureBlobLogger, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("azure blob logger: credential: %w", err)
	}

	client, err := azblob.NewClient(accountURL, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("azure blob logger: client: %w", err)
	}

	l := &AzureBlobLogger{
		client:    client,
		container: container,
		prefix:    prefix,
	}
	l.BatchLogger = NewBatchLogger(l.flush)
	return l, nil
}

func (l *AzureBlobLogger) flush(batch []LogData) error {
	data, err := json.Marshal(batch)
	if err != nil {
		return fmt.Errorf("azure blob marshal: %w", err)
	}

	now := time.Now().UTC()
	name := fmt.Sprintf("%s/%s/%d-%s.json",
		l.prefix, now.Format("2006-01-02"), now.UnixMilli(), uuid.New().String()[:8])

	_, err = l.client.UploadBuffer(context.Background(), l.container, name, data, nil)
	if err != nil {
		return fmt.Errorf("azure blob upload: %w", err)
	}
	return nil
}
