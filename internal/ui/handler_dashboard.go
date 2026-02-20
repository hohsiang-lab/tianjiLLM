package ui

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/ui/pages"
)

func (h *UIHandler) handleDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	stats := pages.DashboardStats{
		ActiveModels: len(h.Config.ModelList),
	}

	if h.DB != nil {
		// Count keys
		keys, err := h.DB.ListVerificationTokens(ctx, db.ListVerificationTokensParams{
			Limit:  1,
			Offset: 0,
		})
		if err == nil {
			// ListVerificationTokens returns paginated results.
			// For total count we do a simple list with high limit.
			allKeys, _ := h.DB.ListVerificationTokens(ctx, db.ListVerificationTokensParams{
				Limit:  100000,
				Offset: 0,
			})
			_ = keys
			stats.TotalKeys = len(allKeys)
		}

		// Spend last 30 days
		now := time.Now()
		thirtyDaysAgo := now.AddDate(0, 0, -30)
		spendRow, err := h.DB.GetGlobalSpend(ctx, db.GetGlobalSpendParams{
			Starttime:   pgtype.Timestamptz{Time: thirtyDaysAgo, Valid: true},
			Starttime_2: pgtype.Timestamptz{Time: now, Valid: true},
		})
		if err == nil {
			if v, ok := spendRow.TotalSpend.(float64); ok {
				stats.TotalSpend = v
			}
		}

		// Requests last 24h
		twentyFourHoursAgo := now.Add(-24 * time.Hour)
		count, err := h.DB.CountSpendLogsByDateRange(ctx, db.CountSpendLogsByDateRangeParams{
			Starttime:   pgtype.Timestamptz{Time: twentyFourHoursAgo, Valid: true},
			Starttime_2: pgtype.Timestamptz{Time: now, Valid: true},
		})
		if err == nil {
			stats.Requests24h = count
		}
	}

	render(ctx, w, pages.DashboardPage(stats))
}
