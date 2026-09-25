package spend

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AnalyticsQuery defines parameters for spend analytics.
type AnalyticsQuery struct {
	GroupBy   string // team, tag, model, user, key
	StartDate time.Time
	EndDate   time.Time
	TopN      int
}

// AnalyticsResult holds grouped spend data.
type AnalyticsResult struct {
	Groups []SpendGroup
	Total  float64
}

// SpendGroup is a single group in analytics results.
type SpendGroup struct {
	Name  string  `json:"name"`
	Spend float64 `json:"spend"`
	Count int     `json:"count"`
}

// QueryByGroup returns spend grouped by the specified dimension.
func QueryByGroup(ctx context.Context, pool *pgxpool.Pool, q AnalyticsQuery) (*AnalyticsResult, error) {
	column, err := groupByColumn(q.GroupBy)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`
		SELECT %s AS name, SUM(spend) AS spend, COUNT(*) AS count
		FROM spend_logs
		WHERE created_at >= $1 AND created_at <= $2
		GROUP BY %s
		ORDER BY spend DESC
	`, column, column)

	rows, err := pool.Query(ctx, query, q.StartDate, q.EndDate)
	if err != nil {
		return nil, fmt.Errorf("analytics query: %w", err)
	}
	defer rows.Close()

	var result AnalyticsResult
	for rows.Next() {
		var g SpendGroup
		if err := rows.Scan(&g.Name, &g.Spend, &g.Count); err != nil {
			return nil, err
		}
		result.Total += g.Spend
		result.Groups = append(result.Groups, g)
	}
	return &result, rows.Err()
}

// QueryTopN returns the top N spenders in the given dimension.
func QueryTopN(ctx context.Context, pool *pgxpool.Pool, q AnalyticsQuery) (*AnalyticsResult, error) {
	column, err := groupByColumn(q.GroupBy)
	if err != nil {
		return nil, err
	}

	topN := q.TopN
	if topN <= 0 {
		topN = 10
	}

	query := fmt.Sprintf(`
		SELECT %s AS name, SUM(spend) AS spend, COUNT(*) AS count
		FROM spend_logs
		WHERE created_at >= $1 AND created_at <= $2
		GROUP BY %s
		ORDER BY spend DESC
		LIMIT $3
	`, column, column)

	rows, err := pool.Query(ctx, query, q.StartDate, q.EndDate, topN)
	if err != nil {
		return nil, fmt.Errorf("analytics top-n: %w", err)
	}
	defer rows.Close()

	var result AnalyticsResult
	for rows.Next() {
		var g SpendGroup
		if err := rows.Scan(&g.Name, &g.Spend, &g.Count); err != nil {
			return nil, err
		}
		result.Total += g.Spend
		result.Groups = append(result.Groups, g)
	}
	return &result, rows.Err()
}

// QueryTrend returns spend over time periods.
func QueryTrend(ctx context.Context, pool *pgxpool.Pool, q AnalyticsQuery) (*AnalyticsResult, error) {
	query := `
		SELECT date_trunc('day', created_at)::text AS name, SUM(spend) AS spend, COUNT(*) AS count
		FROM spend_logs
		WHERE created_at >= $1 AND created_at <= $2
		GROUP BY date_trunc('day', created_at)
		ORDER BY name
	`

	rows, err := pool.Query(ctx, query, q.StartDate, q.EndDate)
	if err != nil {
		return nil, fmt.Errorf("analytics trend: %w", err)
	}
	defer rows.Close()

	var result AnalyticsResult
	for rows.Next() {
		var g SpendGroup
		if err := rows.Scan(&g.Name, &g.Spend, &g.Count); err != nil {
			return nil, err
		}
		result.Total += g.Spend
		result.Groups = append(result.Groups, g)
	}
	return &result, rows.Err()
}

func groupByColumn(groupBy string) (string, error) {
	switch groupBy {
	case "team":
		return "team_id", nil
	case "tag":
		return "request_tags[1]", nil
	case "model":
		return "model", nil
	case "user":
		return "user_id", nil
	case "key":
		return "api_key", nil
	default:
		return "", fmt.Errorf("invalid group_by: %s (must be team, tag, model, user, or key)", groupBy)
	}
}
