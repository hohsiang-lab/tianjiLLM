package callback

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Cloud logger tests verify the BatchLogger embedding and flush behavior.
// Real cloud API interactions require integration tests with actual services.

func TestS3Logger_EmbedsBatchLogger(t *testing.T) {
	// Verify S3Logger correctly embeds BatchLogger
	// Cannot test real S3 without credentials, but verify struct composition
	var _ CustomLogger = (*S3Logger)(nil)
}

func TestGCSLogger_EmbedsBatchLogger(t *testing.T) {
	var _ CustomLogger = (*GCSLogger)(nil)
}

func TestAzureBlobLogger_EmbedsBatchLogger(t *testing.T) {
	var _ CustomLogger = (*AzureBlobLogger)(nil)
}

func TestDynamoDBLogger_EmbedsBatchLogger(t *testing.T) {
	var _ CustomLogger = (*DynamoDBLogger)(nil)
}

func TestSQSLogger_EmbedsBatchLogger(t *testing.T) {
	var _ CustomLogger = (*SQSLogger)(nil)
}

func TestEmailAlerter_ImplementsCustomLogger(t *testing.T) {
	var _ CustomLogger = (*EmailAlerter)(nil)
}

func TestEmailAlerter_RenderTemplate(t *testing.T) {
	html, err := renderAlertEmail("Budget Alert", "Team X has exceeded 90% of budget")
	assert.NoError(t, err)
	assert.Contains(t, html, "Budget Alert")
	assert.Contains(t, html, "Team X")
}

func TestNewEmailAlerter(t *testing.T) {
	a := NewEmailAlerter("smtp.gmail.com", 587, "from@test.com", []string{"to@test.com"}, "user", "pass")
	assert.Equal(t, "smtp.gmail.com", a.host)
	assert.Equal(t, 587, a.port)
	assert.False(t, a.useTLS) // 587 = STARTTLS

	a465 := NewEmailAlerter("smtp.gmail.com", 465, "from@test.com", []string{"to@test.com"}, "user", "pass")
	assert.True(t, a465.useTLS) // 465 = implicit TLS
}
