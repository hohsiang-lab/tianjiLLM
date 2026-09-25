package spend

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// FOCUSRecord represents a FinOps FOCUS 1.2 compatible record.
type FOCUSRecord struct {
	BilledCost    float64 `json:"BilledCost"`
	EffectiveCost float64 `json:"EffectiveCost"`
	Provider      string  `json:"Provider"`
	Service       string  `json:"Service"`
	ResourceID    string  `json:"ResourceId"`
	UsageQuantity int     `json:"UsageQuantity"`
	UsageUnit     string  `json:"UsageUnit"`
	BillingPeriod string  `json:"BillingPeriod"`
	ChargeType    string  `json:"ChargeType"`
}

// ExportFOCUS exports spend data in FOCUS 1.2 JSON format.
// Parquet output deferred per TD-010.
func ExportFOCUS(ctx context.Context, pool *pgxpool.Pool, start, end time.Time) ([]byte, error) {
	query := `
		SELECT model, COALESCE(provider, 'unknown') as provider,
		       SUM(spend) AS total_spend, SUM(total_tokens) AS total_tokens,
		       COUNT(*) AS request_count
		FROM spend_logs
		WHERE created_at >= $1 AND created_at <= $2
		GROUP BY model, provider
		ORDER BY total_spend DESC
	`

	rows, err := pool.Query(ctx, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("focus export: %w", err)
	}
	defer rows.Close()

	period := fmt.Sprintf("%s/%s", start.Format("2006-01-02"), end.Format("2006-01-02"))

	var records []FOCUSRecord
	for rows.Next() {
		var model, provider string
		var totalSpend float64
		var totalTokens, requestCount int

		if err := rows.Scan(&model, &provider, &totalSpend, &totalTokens, &requestCount); err != nil {
			return nil, err
		}

		records = append(records, FOCUSRecord{
			BilledCost:    totalSpend,
			EffectiveCost: totalSpend,
			Provider:      provider,
			Service:       "LLM API",
			ResourceID:    model,
			UsageQuantity: totalTokens,
			UsageUnit:     "tokens",
			BillingPeriod: period,
			ChargeType:    "Usage",
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return json.MarshalIndent(records, "", "  ")
}
