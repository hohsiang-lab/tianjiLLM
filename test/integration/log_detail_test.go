package integration

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

// T013: Integration test for GetSpendLogDetail with duration.
func TestGetSpendLogDetail_IncludesDuration(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()
	pool := getPool(t, q)

	requestID := "test-detail-" + time.Now().Format("20060102150405")
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM "SpendLogs" WHERE request_id = $1`, requestID)
	})

	durationMs := int32(1500)
	err := q.CreateSpendLog(ctx, db.CreateSpendLogParams{
		RequestID:         requestID,
		CallType:          "completion",
		ApiKey:            "sk-test-detail",
		Spend:             0.05,
		TotalTokens:       1200,
		PromptTokens:      1000,
		CompletionTokens:  200,
		Starttime:         pgtype.Timestamptz{Time: time.Now().Add(-2 * time.Second), Valid: true},
		Endtime:           pgtype.Timestamptz{Time: time.Now(), Valid: true},
		Model:             "anthropic/claude-opus-4-6",
		ReasoningEffort:   "high",
		Metadata:          []byte(`{"test": true}`),
		RequestTags:       []string{},
		RequestDurationMs: &durationMs,
	})
	require.NoError(t, err)

	row, err := q.GetSpendLogDetail(ctx, requestID)
	require.NoError(t, err)

	assert.Equal(t, requestID, row.RequestID)
	assert.Equal(t, "anthropic/claude-opus-4-6", row.Model)
	assert.Equal(t, "high", row.ReasoningEffort)
	assert.Equal(t, int32(1000), row.PromptTokens)
	assert.Equal(t, int32(200), row.CompletionTokens)
	assert.NotNil(t, row.RequestDurationMs)
	assert.Equal(t, int32(1500), *row.RequestDurationMs)

	// No ErrorLog inserted — error fields should be nil
	assert.Nil(t, row.ErrorStatusCode)
	assert.Nil(t, row.ErrorType)
	assert.Nil(t, row.ErrorMessage)
}

// T013 continued: Test with an associated ErrorLog.
func TestGetSpendLogDetail_WithErrorLog(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()
	pool := getPool(t, q)

	requestID := "test-detail-err-" + time.Now().Format("20060102150405")
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM "SpendLogs" WHERE request_id = $1`, requestID)
		_, _ = pool.Exec(ctx, `DELETE FROM "ErrorLogs" WHERE request_id = $1`, requestID)
	})

	// Insert SpendLog
	err := q.CreateSpendLog(ctx, db.CreateSpendLogParams{
		RequestID:   requestID,
		CallType:    "completion",
		ApiKey:      "sk-test-detail-err",
		Model:       "openai/gpt-4o",
		Metadata:    []byte(`{}`),
		RequestTags: []string{},
		Starttime:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
		Endtime:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	require.NoError(t, err)

	// Insert ErrorLog
	_, err = pool.Exec(ctx,
		`INSERT INTO "ErrorLogs" (request_id, api_key_hash, model, provider, status_code, error_type, error_message, traceback)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		requestID, "sk-test", "openai/gpt-4o", "openai", 429, "rate_limit_error", "Rate limit exceeded", "traceback line 1\ntraceback line 2")
	require.NoError(t, err)

	row, err := q.GetSpendLogDetail(ctx, requestID)
	require.NoError(t, err)

	assert.Equal(t, requestID, row.RequestID)
	assert.NotNil(t, row.ErrorStatusCode)
	assert.Equal(t, int32(429), *row.ErrorStatusCode)
	assert.NotNil(t, row.ErrorType)
	assert.Equal(t, "rate_limit_error", *row.ErrorType)
	assert.NotNil(t, row.ErrorMessage)
	assert.Equal(t, "Rate limit exceeded", *row.ErrorMessage)
	assert.NotNil(t, row.ErrorTraceback)
	assert.Contains(t, *row.ErrorTraceback, "traceback line 1")
}

// T022: Payload DB roundtrip test.
func TestCreateRequestPayload(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()
	pool := getPool(t, q)

	requestID := "test-payload-" + time.Now().Format("20060102150405")
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM "RequestPayloads" WHERE request_id = $1`, requestID)
		_, _ = pool.Exec(ctx, `DELETE FROM "SpendLogs" WHERE request_id = $1`, requestID)
	})

	// Need a SpendLog first (RequestPayloads.request_id is PK, not FK — but good practice)
	err := q.CreateSpendLog(ctx, db.CreateSpendLogParams{
		RequestID:   requestID,
		CallType:    "completion",
		ApiKey:      "sk-test-payload",
		Model:       "anthropic/claude-opus-4-6",
		Metadata:    []byte(`{}`),
		RequestTags: []string{},
		Starttime:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
		Endtime:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	require.NoError(t, err)

	// Insert payload
	messages := `[{"role":"user","content":"hello"}]`
	response := `{"id":"chatcmpl-123","choices":[{"message":{"role":"assistant","content":"hi"}}]}`

	err = q.CreateRequestPayload(ctx, db.CreateRequestPayloadParams{
		RequestID: requestID,
		Messages:  messages,
		Response:  response,
	})
	require.NoError(t, err)

	// Read back
	payload, err := q.GetRequestPayload(ctx, requestID)
	require.NoError(t, err)

	assert.JSONEq(t, messages, payload.Messages)
	assert.JSONEq(t, response, payload.Response)
}
