package spend

import (
	"bytes"
	"log"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/pricing"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecord_FallbackToPricingDefault(t *testing.T) {
	tracker := NewTracker(nil, nil, false)

	model := "gpt-4o"
	info := pricing.Default().GetModelInfo(model)
	require.NotNil(t, info, "gpt-4o must exist in embedded pricing data")

	rec := SpendRecord{
		Model:            model,
		PromptTokens:     100,
		CompletionTokens: 50,
	}

	expectedCost := pricing.Default().TotalCost(model, pricing.TokenUsage{PromptTokens: 100, CompletionTokens: 50})
	assert.Greater(t, expectedCost, 0.0)

	got := tracker.calculateCost(rec)
	assert.Equal(t, expectedCost, got)
}

func TestRecord_StripProviderPrefix(t *testing.T) {
	model := "anthropic/claude-sonnet-4-20250514"
	cost := pricing.Default().TotalCost(model, pricing.TokenUsage{PromptTokens: 1000, CompletionTokens: 500})
	require.Greater(t, cost, 0.0, "provider-prefixed model must exist in embedded pricing data")
}

func TestTracker_StoresDurationMs(t *testing.T) {
	// Tracker with nil DB — won't actually write, but we can verify duration computation
	tracker := NewTracker(nil, nil, false)

	start := time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC)
	end := start.Add(1500 * time.Millisecond)

	rec := SpendRecord{
		Model:     "gpt-4o",
		StartTime: start,
		EndTime:   end,
	}

	// Capture log output to verify duration is logged
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	tracker.Record(t.Context(), rec)

	output := buf.String()
	assert.Contains(t, output, "duration_ms=1500")
}

func TestTracker_LogsRequestMetrics(t *testing.T) {
	tracker := NewTracker(nil, nil, false)

	rec := SpendRecord{
		Model:            "anthropic/claude-opus-4-6",
		APIKey:           "sk-test-key-hash-12345",
		PromptTokens:     1000,
		CompletionTokens: 200,
		TotalTokens:      1200,
		Cost:             0.05,
		StartTime:        time.Now(),
		EndTime:          time.Now().Add(500 * time.Millisecond),
	}

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	tracker.Record(t.Context(), rec)

	output := buf.String()
	assert.Contains(t, output, "request-log:")
	assert.Contains(t, output, "model=anthropic/claude-opus-4-6")
	assert.Contains(t, output, "prompt_tokens=1000")
	assert.Contains(t, output, "completion_tokens=200")
	assert.Contains(t, output, "total_tokens=1200")
	assert.Contains(t, output, "cost=0.050000")
	assert.Contains(t, output, "status=success")
	assert.Contains(t, output, "key=sk-test-key-")
}

func TestLogSuccess_PropagatesRequesterIPAddress(t *testing.T) {
	// captureSpy records the SpendRecord passed to Record via a nil-DB tracker.
	// Since db is nil, Record() skips DB write but we can verify the mapping
	// by inspecting the log output which includes key fields.
	tracker := NewTracker(nil, nil, false)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	data := callback.LogData{
		Model:              "claude-sonnet-4-6",
		Provider:           "anthropic",
		APIKey:             "testhash",
		PromptTokens:       10,
		CompletionTokens:   5,
		TotalTokens:        15,
		StartTime:          time.Now(),
		EndTime:            time.Now().Add(100 * time.Millisecond),
		RequesterIPAddress: "203.0.113.42",
	}
	tracker.LogSuccess(data)

	// Verify Record was called (log line emitted) and RequesterIPAddress
	// was mapped from LogData → SpendRecord → Record params correctly.
	// The tracker logs a request-log line when Record is called.
	output := buf.String()
	assert.Contains(t, output, "request-log:")
	assert.Contains(t, output, "model=claude-sonnet-4-6")
	assert.Contains(t, output, "ip=203.0.113.42")
}

func TestSpendRecordFromLogData_PropagatesRawPayloads(t *testing.T) {
	requestPayload := map[string]any{"type": "image_generation"}
	responsePayload := map[string]any{"data_len": 1}

	rec := spendRecordFromLogData(callback.LogData{
		Model:           "gpt-image-2",
		RequestPayload:  requestPayload,
		ResponsePayload: responsePayload,
	})

	assert.Equal(t, requestPayload, rec.RequestPayload)
	assert.Equal(t, responsePayload, rec.ResponsePayload)
}

func TestSpendRecordFromLogData_PropagatesReasoningEffort(t *testing.T) {
	rec := spendRecordFromLogData(callback.LogData{ReasoningEffort: "xhigh"})
	assert.Equal(t, "xhigh", rec.ReasoningEffort)
}

func TestSpendRecordFromLogData_PropagatesCacheHit(t *testing.T) {
	rec := spendRecordFromLogData(callback.LogData{
		Model:    "gpt-5.5",
		CacheHit: true,
	})

	assert.True(t, rec.CacheHit)
}

func TestTracker_Record_IncrementsTPMCounter(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	tracker := NewTracker(nil, nil, false)
	tracker.SetRedis(rdb)

	// Suppress log output
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	rec := SpendRecord{
		Model:            "gpt-4o",
		APIKey:           "sk-test-key-hash-abcdef123456",
		PromptTokens:     500,
		CompletionTokens: 200,
		TotalTokens:      700,
		StartTime:        time.Now(),
		EndTime:          time.Now().Add(100 * time.Millisecond),
	}

	tracker.Record(t.Context(), rec)

	// TPM key uses full API key hash to match DynamicRateLimiter.CheckFull().
	tpmKey := "tianji:dynamic_tpm:" + rec.APIKey

	val, err := mr.Get(tpmKey)
	require.NoError(t, err)
	assert.Equal(t, "700", val, "TPM counter should equal TotalTokens")

	// Verify TTL is set (miniredis stores TTL)
	ttl := mr.TTL(tpmKey)
	assert.Greater(t, ttl, time.Duration(0), "TPM key should have a TTL")
	assert.LessOrEqual(t, ttl, 60*time.Second, "TPM key TTL should be <= 60s")
}

func TestTracker_Record_NoTPMUpdate_ZeroTokens(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	tracker := NewTracker(nil, nil, false)
	tracker.SetRedis(rdb)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	rec := SpendRecord{
		Model:       "gpt-4o",
		APIKey:      "sk-test-key-hash-abcdef123456",
		TotalTokens: 0,
		StartTime:   time.Now(),
		EndTime:     time.Now().Add(100 * time.Millisecond),
	}

	tracker.Record(t.Context(), rec)

	tpmKey := "tianji:dynamic_tpm:" + rec.APIKey
	assert.False(t, mr.Exists(tpmKey), "TPM key should not be created for zero tokens")
}

func TestTracker_Record_NoTPMUpdate_NilRedis(t *testing.T) {
	tracker := NewTracker(nil, nil, false)
	// No SetRedis call — rdb is nil

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	rec := SpendRecord{
		Model:       "gpt-4o",
		APIKey:      "sk-test-key-hash-abcdef123456",
		TotalTokens: 500,
		StartTime:   time.Now(),
		EndTime:     time.Now().Add(100 * time.Millisecond),
	}

	// Should not panic
	tracker.Record(t.Context(), rec)
}

func TestTracker_Record_TPMAccumulates(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	tracker := NewTracker(nil, nil, false)
	tracker.SetRedis(rdb)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	apiKey := "sk-test-key-hash-abcdef123456"

	// First call: 500 tokens
	tracker.Record(t.Context(), SpendRecord{
		Model: "gpt-4o", APIKey: apiKey, TotalTokens: 500,
		StartTime: time.Now(), EndTime: time.Now().Add(100 * time.Millisecond),
	})

	// Second call: 300 tokens
	tracker.Record(t.Context(), SpendRecord{
		Model: "gpt-4o", APIKey: apiKey, TotalTokens: 300,
		StartTime: time.Now(), EndTime: time.Now().Add(100 * time.Millisecond),
	})

	tpmKey := "tianji:dynamic_tpm:" + apiKey

	val, err := mr.Get(tpmKey)
	require.NoError(t, err)
	assert.Equal(t, "800", val, "TPM counter should accumulate across calls")
}
