package spend

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/pricing"
)

// Tracker records spend after each LLM call and updates key/team/user budgets.
type Tracker struct {
	db           *db.Queries
	buffer       *RedisBuffer
	rdb          redis.UniversalClient
	storePrompts bool
}

// NewTracker creates a spend tracker. When storePrompts is true,
// request/response payloads are persisted to the RequestPayloads table.
func NewTracker(database *db.Queries, buffer *RedisBuffer, storePrompts bool) *Tracker {
	return &Tracker{
		db:           database,
		buffer:       buffer,
		storePrompts: storePrompts,
	}
}

// SetRedis attaches a Redis client used for post-call TPM counter updates.
func (t *Tracker) SetRedis(rdb redis.UniversalClient) {
	t.rdb = rdb
}

// SpendRecord holds the data needed to record spend.
type SpendRecord struct {
	Model                    string
	ModelGroup               string
	APIBase                  string
	APIKey                   string
	PromptTokens             int
	CompletionTokens         int
	TotalTokens              int
	StartTime                time.Time
	EndTime                  time.Time
	User                     string
	TeamID                   string
	OrganizationID           string
	Provider                 string
	Tags                     []string
	Metadata                 map[string]any
	Cost                     float64
	CacheHit                 bool
	CacheReadInputTokens     int
	CacheCreationInputTokens int
	CallType                 string
	Request                  *model.ChatCompletionRequest
	Response                 *model.ModelResponse
	RequestPayload           any
	ResponsePayload          any
	RequesterIPAddress       string
	UpstreamTokenKey         string
	ReasoningEffort          string
	OpenAISubscription       *callback.OpenAISubscriptionAttribution
}

// LogSuccess implements callback.CustomLogger — writes spend to DB.
func (t *Tracker) LogSuccess(data callback.LogData) {
	t.Record(context.Background(), spendRecordFromLogData(data))
}

func spendRecordFromLogData(data callback.LogData) SpendRecord {
	return SpendRecord{
		Model:                    data.Model,
		APIKey:                   data.APIKey,
		PromptTokens:             data.PromptTokens,
		CompletionTokens:         data.CompletionTokens,
		TotalTokens:              data.TotalTokens,
		CacheReadInputTokens:     data.CacheReadInputTokens,
		CacheCreationInputTokens: data.CacheCreationInputTokens,
		StartTime:                data.StartTime,
		EndTime:                  data.EndTime,
		User:                     data.UserID,
		TeamID:                   data.TeamID,
		OrganizationID:           data.OrganizationID,
		Provider:                 data.Provider,
		Tags:                     data.RequestTags,
		Cost:                     data.Cost,
		CacheHit:                 data.CacheHit,
		CallType:                 data.CallType,
		Request:                  data.Request,
		Response:                 data.Response,
		RequestPayload:           data.RequestPayload,
		ResponsePayload:          data.ResponsePayload,
		RequesterIPAddress:       data.RequesterIPAddress,
		UpstreamTokenKey:         data.UpstreamTokenKey,
		ReasoningEffort:          data.ReasoningEffort,
		OpenAISubscription:       data.OpenAISubscription,
	}
}

// LogFailure implements callback.CustomLogger — no-op for failed requests.
func (t *Tracker) LogFailure(callback.LogData) {}

// calculateCost computes the cost using pricing.Default().Cost() with cache token support.
// SpendRecord.PromptTokens is total; TokenUsage.PromptTokens is regular (non-cache) input.
func (t *Tracker) calculateCost(rec SpendRecord) float64 {
	if rec.Cost != 0 {
		return rec.Cost
	}
	regularInput := rec.PromptTokens - rec.CacheReadInputTokens - rec.CacheCreationInputTokens
	if regularInput < 0 {
		regularInput = 0
	}
	usage := pricing.TokenUsage{
		PromptTokens:             regularInput,
		CompletionTokens:         rec.CompletionTokens,
		CacheReadInputTokens:     rec.CacheReadInputTokens,
		CacheCreationInputTokens: rec.CacheCreationInputTokens,
	}
	prompt, completion := pricing.Default().Cost(rec.Model, usage)
	return prompt + completion
}

// Record records spend for a completed LLM call.
func (t *Tracker) Record(ctx context.Context, rec SpendRecord) {
	cost := t.calculateCost(rec)
	if rec.PromptTokens == 0 && rec.CompletionTokens == 0 {
		log.Printf("warn: spend record for model %q has zero tokens — usage may not have been extracted", rec.Model)
	}

	metadataJSON, err := json.Marshal(rec.metadataWithOpenAISubscriptionAttribution())
	if err != nil {
		log.Printf("error: failed to marshal metadata for model %q: %v", rec.Model, err)
		metadataJSON = []byte("{}")
	}
	if rec.Tags == nil {
		rec.Tags = []string{}
	}

	callType := rec.CallType
	if callType == "" {
		callType = "completion"
	}

	durationMs := int32(rec.EndTime.Sub(rec.StartTime).Milliseconds())

	requestID := uuid.New().String()

	params := db.CreateSpendLogParams{
		RequestID:         requestID,
		CallType:          callType,
		ApiKey:            rec.APIKey,
		Spend:             cost,
		TotalTokens:       int32(rec.TotalTokens),
		PromptTokens:      int32(rec.PromptTokens),
		CompletionTokens:  int32(rec.CompletionTokens),
		Starttime:         pgtype.Timestamptz{Time: rec.StartTime, Valid: true},
		Endtime:           pgtype.Timestamptz{Time: rec.EndTime, Valid: true},
		Model:             rec.Model,
		ModelGroup:        rec.ModelGroup,
		ApiBase:           rec.APIBase,
		User:              rec.User,
		Metadata:          metadataJSON,
		RequestTags:       rec.Tags,
		RequestDurationMs: &durationMs,
		Provider:          rec.Provider,
		UpstreamTokenKey:  rec.UpstreamTokenKey,
		ReasoningEffort:   rec.ReasoningEffort,
	}
	if rec.CacheHit {
		params.CacheHit = "True"
	}

	if rec.TeamID != "" {
		params.TeamID = &rec.TeamID
	}
	if rec.OrganizationID != "" {
		params.OrganizationID = &rec.OrganizationID
	}
	if rec.RequesterIPAddress != "" {
		params.RequesterIpAddress = &rec.RequesterIPAddress
	}

	keyHash := rec.APIKey
	if len(keyHash) > 12 {
		keyHash = keyHash[:12]
	}
	log.Printf("request-log: request_id=%s model=%s reasoning_effort=%s prompt_tokens=%d completion_tokens=%d total_tokens=%d cost=%.6f duration_ms=%d status=success key=%s ip=%s",
		requestID, rec.Model, rec.ReasoningEffort, rec.PromptTokens, rec.CompletionTokens, rec.TotalTokens, cost, durationMs, keyHash, rec.RequesterIPAddress)

	// If Redis buffer is available, batch writes
	if t.buffer != nil {
		t.buffer.Push(params)
		t.storePayload(ctx, requestID, rec)
	} else if t.db != nil {
		// Direct DB write
		if err := t.db.CreateSpendLog(ctx, params); err != nil {
			log.Printf("warn: failed to write spend log: %v", err)
		}

		t.storePayload(ctx, requestID, rec)

		// Update key spend
		if rec.APIKey != "" {
			if err := t.db.UpdateVerificationTokenSpend(ctx, db.UpdateVerificationTokenSpendParams{
				Token: rec.APIKey,
				Spend: cost,
			}); err != nil {
				log.Printf("warn: failed to update key spend: %v", err)
			}
		}
	}

	// Post-call TPM counter update for dynamic rate limiting.
	// Uses full API key hash (not truncated log keyHash) to match DynamicRateLimiter.CheckFull().
	// Note: does not include model group suffix — matches the base key format only.
	if t.rdb != nil && rec.TotalTokens > 0 && rec.APIKey != "" {
		tpmKey := "tianji:dynamic_tpm:" + rec.APIKey
		if err := t.rdb.IncrBy(ctx, tpmKey, int64(rec.TotalTokens)).Err(); err != nil {
			log.Printf("warn: failed to increment TPM counter %s: %v", tpmKey[:min(len(tpmKey), 40)], err)
		}
		if err := t.rdb.Expire(ctx, tpmKey, 60*time.Second).Err(); err != nil {
			log.Printf("warn: failed to set TPM counter expiry %s: %v", tpmKey[:min(len(tpmKey), 40)], err)
		}
	}
}

func (rec SpendRecord) metadataWithOpenAISubscriptionAttribution() map[string]any {
	if rec.OpenAISubscription == nil {
		return rec.Metadata
	}
	metadata := make(map[string]any, len(rec.Metadata)+1)
	for key, value := range rec.Metadata {
		metadata[key] = value
	}
	metadata["openai_subscription"] = rec.OpenAISubscription.SafeMetadata()
	return metadata
}

// marshalTruncated serializes v to JSON and truncates to maxLen.
// Returns []byte("{}") when v is nil.
func marshalTruncated(v any, label string, maxLen int) string {
	if v == nil {
		return "{}"
	}
	raw, err := json.Marshal(v)
	if err != nil {
		log.Printf("error: failed to marshal %s: %v", label, err)
		errJSON, _ := json.Marshal(map[string]string{"_error": fmt.Sprintf("marshal %s failed: %s", label, err.Error())})
		return string(errJSON)
	}
	return callback.TruncatePayload(string(raw), maxLen)
}

// storePayload writes request/response payloads to the RequestPayloads table when enabled.
func (t *Tracker) storePayload(ctx context.Context, requestID string, rec SpendRecord) {
	if !t.storePrompts || t.db == nil {
		return
	}

	maxLen := callback.MaxStringLengthPromptInDB()

	var messages any
	switch {
	case rec.RequestPayload != nil:
		messages = rec.RequestPayload
	case rec.Request != nil:
		messages = rec.Request.Messages
	}
	response := rec.ResponsePayload
	if response == nil {
		response = rec.Response
	}

	if err := t.db.CreateRequestPayload(ctx, db.CreateRequestPayloadParams{
		RequestID: requestID,
		Messages:  marshalTruncated(messages, "request messages", maxLen),
		Response:  marshalTruncated(response, "response", maxLen),
	}); err != nil {
		log.Printf("warn: failed to store request payload: %v", err)
	}
}
