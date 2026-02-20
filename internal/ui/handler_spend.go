package ui

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/ui/pages"
)

func (h *UIHandler) handleSpend(w http.ResponseWriter, r *http.Request) {
	data := h.loadSpendData(r)
	render(r.Context(), w, pages.SpendPage(data))
}

func (h *UIHandler) handleSpendTable(w http.ResponseWriter, r *http.Request) {
	data := h.loadSpendData(r)
	render(r.Context(), w, pages.SpendTablePartial(data))
}

func (h *UIHandler) loadSpendData(r *http.Request) pages.SpendPageData {
	now := time.Now()
	startDate := r.URL.Query().Get("start_date")
	endDate := r.URL.Query().Get("end_date")
	groupBy := r.URL.Query().Get("group_by")

	if startDate == "" {
		startDate = now.AddDate(0, 0, -30).Format("2006-01-02")
	}
	if endDate == "" {
		endDate = now.Format("2006-01-02")
	}
	if groupBy == "" {
		groupBy = "model"
	}

	data := pages.SpendPageData{
		StartDate: startDate,
		EndDate:   endDate,
		GroupBy:   groupBy,
	}

	if h.DB == nil {
		return data
	}

	start, _ := time.Parse("2006-01-02", startDate)
	end, _ := time.Parse("2006-01-02", endDate)
	end = end.Add(24*time.Hour - time.Second)

	ctx := r.Context()
	tsStart := pgtype.Timestamptz{Time: start, Valid: true}
	tsEnd := pgtype.Timestamptz{Time: end, Valid: true}

	switch groupBy {
	case "model":
		rows, err := h.DB.GetGlobalSpendByProvider(ctx, db.GetGlobalSpendByProviderParams{
			Starttime:   tsStart,
			Starttime_2: tsEnd,
		})
		if err == nil {
			for _, row := range rows {
				data.Rows = append(data.Rows, pages.SpendRow{
					GroupKey:     row.Provider,
					TotalSpend:   float64(row.TotalSpend),
					RequestCount: row.RequestCount,
				})
			}
		}
	case "key":
		rows, err := h.DB.GetGlobalSpendReportByKey(ctx, db.GetGlobalSpendReportByKeyParams{
			Starttime:   tsStart,
			Starttime_2: tsEnd,
		})
		if err == nil {
			agg := aggregateByKey(rows, func(r db.GetGlobalSpendReportByKeyRow) (string, int64, int64) {
				return r.GroupKey, r.TotalSpend, r.RequestCount
			})
			data.Rows = agg
		}
	case "team":
		rows, err := h.DB.GetGlobalSpendReport(ctx, db.GetGlobalSpendReportParams{
			Starttime:   tsStart,
			Starttime_2: tsEnd,
		})
		if err == nil {
			agg := aggregateByKey(rows, func(r db.GetGlobalSpendReportRow) (string, int64, int64) {
				return r.GroupKey, r.TotalSpend, r.RequestCount
			})
			data.Rows = agg
		}
	}

	return data
}

func aggregateByKey[T any](rows []T, extract func(T) (string, int64, int64)) []pages.SpendRow {
	type acc struct {
		spend    int64
		requests int64
	}
	m := map[string]*acc{}
	var order []string
	for _, row := range rows {
		key, s, req := extract(row)
		if _, ok := m[key]; !ok {
			m[key] = &acc{}
			order = append(order, key)
		}
		m[key].spend += s
		m[key].requests += req
	}

	result := make([]pages.SpendRow, 0, len(order))
	for _, key := range order {
		a := m[key]
		result = append(result, pages.SpendRow{
			GroupKey:     key,
			TotalSpend:   float64(a.spend),
			RequestCount: a.requests,
		})
	}
	return result
}
