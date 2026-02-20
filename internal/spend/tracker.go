package spend

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

// Tracker records spend after each LLM call and updates key/team/user budgets.
type Tracker struct {
	db         *db.Queries
	calculator *Calculator
	buffer     *RedisBuffer
}

// NewTracker creates a spend tracker.
func NewTracker(database *db.Queries, calculator *Calculator, buffer *RedisBuffer) *Tracker {
	return &Tracker{
		db:         database,
		calculator: calculator,
		buffer:     buffer,
	}
}

// SpendRecord holds the data needed to record spend.
type SpendRecord struct {
	Model            string
	ModelGroup       string
	APIBase          string
	APIKey           string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	StartTime        time.Time
	EndTime          time.Time
	User             string
	TeamID           string
	Tags             []string
	Metadata         map[string]any
	Cost             float64
}

// LogSuccess implements callback.CustomLogger — writes spend to DB.
func (t *Tracker) LogSuccess(data callback.LogData) {
	t.Record(context.Background(), SpendRecord{
		Model:            data.Model,
		APIKey:           data.APIKey,
		PromptTokens:     data.PromptTokens,
		CompletionTokens: data.CompletionTokens,
		TotalTokens:      data.TotalTokens,
		StartTime:        data.StartTime,
		EndTime:          data.EndTime,
		User:             data.UserID,
		TeamID:           data.TeamID,
		Tags:             data.RequestTags,
		Cost:             data.Cost,
	})
}

// LogFailure implements callback.CustomLogger — no-op for failed requests.
func (t *Tracker) LogFailure(callback.LogData) {}

// Record records spend for a completed LLM call.
func (t *Tracker) Record(ctx context.Context, rec SpendRecord) {
	cost := rec.Cost
	if cost == 0 && t.calculator != nil {
		cost = t.calculator.Calculate(rec.Model, rec.PromptTokens, rec.CompletionTokens)
	}

	metadataJSON, _ := json.Marshal(rec.Metadata)
	if rec.Tags == nil {
		rec.Tags = []string{}
	}

	params := db.CreateSpendLogParams{
		RequestID:        uuid.New().String(),
		CallType:         "completion",
		ApiKey:           rec.APIKey,
		Spend:            cost,
		TotalTokens:      int32(rec.TotalTokens),
		PromptTokens:     int32(rec.PromptTokens),
		CompletionTokens: int32(rec.CompletionTokens),
		Starttime:        pgtype.Timestamptz{Time: rec.StartTime, Valid: true},
		Endtime:          pgtype.Timestamptz{Time: rec.EndTime, Valid: true},
		Model:            rec.Model,
		ModelGroup:       rec.ModelGroup,
		ApiBase:          rec.APIBase,
		User:             rec.User,
		Metadata:         metadataJSON,
		RequestTags:      rec.Tags,
	}

	if rec.TeamID != "" {
		params.TeamID = &rec.TeamID
	}

	// If Redis buffer is available, batch writes
	if t.buffer != nil {
		t.buffer.Push(params)
		return
	}

	// Direct DB write
	if t.db != nil {
		if err := t.db.CreateSpendLog(ctx, params); err != nil {
			log.Printf("warn: failed to write spend log: %v", err)
		}

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
}
