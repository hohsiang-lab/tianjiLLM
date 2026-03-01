package ui

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/ui/pages"
)

// parseDateRange extracts start/end timestamps from query params.
// Convention: [start, end) — inclusive start, exclusive end.
func parseDateRange(r *http.Request) (preset, startDate, endDate string, tsStart, tsEnd pgtype.Timestamptz) {
	q := r.URL.Query()
	preset = q.Get("preset")
	if preset == "" {
		preset = "7d"
	}

	now := time.Now()
	var start, end time.Time

	switch preset {
	case "24h":
		start = now.Add(-24 * time.Hour)
		end = now
	case "30d":
		start = now.AddDate(0, 0, -30)
		end = now
	case "custom":
		sd := q.Get("start_date")
		ed := q.Get("end_date")
		if sd != "" && ed != "" {
			start, _ = time.Parse("2006-01-02", sd)
			e, _ := time.Parse("2006-01-02", ed)
			end = e.Add(24 * time.Hour) // exclusive end: start-of-next-day
		} else {
			start = now.AddDate(0, 0, -7)
			end = now
			preset = "7d"
		}
	default: // "7d"
		preset = "7d"
		start = now.AddDate(0, 0, -7)
		end = now
	}

	startDate = start.Format("2006-01-02")
	endDate = end.Format("2006-01-02")
	if preset == "custom" {
		// For custom, show the user-selected end date (not the exclusive boundary)
		endDate = end.Add(-24 * time.Hour).Format("2006-01-02")
	}

	tsStart = pgtype.Timestamptz{Time: start, Valid: true}
	tsEnd = pgtype.Timestamptz{Time: end, Valid: true}
	return
}

func activeTab(r *http.Request) string {
	tab := r.URL.Query().Get("tab")
	switch tab {
	case "cost", "model-activity", "key-activity", "endpoint-activity":
		return tab
	default:
		return "cost"
	}
}

// --- Handlers ---

func (h *UIHandler) handleUsage(w http.ResponseWriter, r *http.Request) {
	preset, startDate, endDate, tsStart, tsEnd := parseDateRange(r)
	tab := activeTab(r)

	base := pages.UsagePageData{
		ActiveTab: tab,
		StartDate: startDate,
		EndDate:   endDate,
		Preset:    preset,
	}

	var tabContent pages.UsageTabContent
	switch tab {
	case "model-activity":
		tabContent = h.loadModelActivityData(r, base, tsStart, tsEnd)
	case "key-activity":
		tabContent = h.loadKeyActivityData(r, base, tsStart, tsEnd)
	case "endpoint-activity":
		tabContent = h.loadEndpointActivityData(r, base, tsStart, tsEnd)
	default:
		tabContent = h.loadCostTabData(r, base, tsStart, tsEnd)
	}

	rlTokens := h.buildRateLimitWidgetData()
	render(r.Context(), w, pages.UsagePage(base, tabContent, rlTokens))
}

func (h *UIHandler) handleUsageTab(w http.ResponseWriter, r *http.Request) {
	preset, startDate, endDate, tsStart, tsEnd := parseDateRange(r)
	tab := activeTab(r)

	base := pages.UsagePageData{
		ActiveTab: tab,
		StartDate: startDate,
		EndDate:   endDate,
		Preset:    preset,
	}

	switch tab {
	case "model-activity":
		data := h.loadModelActivityData(r, base, tsStart, tsEnd)
		render(r.Context(), w, pages.UsageModelActivityTab(data))
	case "key-activity":
		data := h.loadKeyActivityData(r, base, tsStart, tsEnd)
		render(r.Context(), w, pages.UsageKeyActivityTab(data))
	case "endpoint-activity":
		data := h.loadEndpointActivityData(r, base, tsStart, tsEnd)
		render(r.Context(), w, pages.UsageEndpointActivityTab(data))
	default:
		data := h.loadCostTabData(r, base, tsStart, tsEnd)
		render(r.Context(), w, pages.UsageCostTab(data))
	}
}

func (h *UIHandler) handleUsageTopKeys(w http.ResponseWriter, r *http.Request) {
	_, _, _, tsStart, tsEnd := parseDateRange(r)

	limit := 5
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 && v <= 50 {
		limit = v
	}

	var keys []pages.TopKey
	if h.DB != nil {
		rows, err := h.DB.GetTopKeysBySpend(r.Context(), db.GetTopKeysBySpendParams{
			StartDate:  tsStart,
			EndDate:    tsEnd,
			QueryLimit: int32(limit),
		})
		if err == nil {
			keys = toTopKeys(rows)
		}
	}

	render(r.Context(), w, pages.UsageTopKeysPartial(keys, limit))
}

func (h *UIHandler) handleUsageExport(w http.ResponseWriter, r *http.Request) {
	tab := activeTab(r)
	preset, startDate, endDate, tsStart, tsEnd := parseDateRange(r)

	base := pages.UsagePageData{
		ActiveTab: tab,
		StartDate: startDate,
		EndDate:   endDate,
		Preset:    preset,
	}

	filename := fmt.Sprintf("usage-%s-%s-%s.csv", tab, startDate, endDate)
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

	cw := csv.NewWriter(w)
	defer cw.Flush()

	switch tab {
	case "cost":
		data := h.loadCostTabData(r, base, tsStart, tsEnd)
		writeCostCSV(cw, data)
	case "model-activity":
		data := h.loadModelActivityData(r, base, tsStart, tsEnd)
		writeModelActivityCSV(cw, data)
	case "key-activity":
		data := h.loadKeyActivityData(r, base, tsStart, tsEnd)
		writeKeyActivityCSV(cw, data)
	case "endpoint-activity":
		data := h.loadEndpointActivityData(r, base, tsStart, tsEnd)
		writeEndpointActivityCSV(cw, data)
	default:
		http.Error(w, "invalid tab: must be one of cost, model-activity, key-activity, endpoint-activity", http.StatusBadRequest)
		return
	}
}

// --- Data Loaders ---

func (h *UIHandler) loadCostTabData(r *http.Request, base pages.UsagePageData, tsStart, tsEnd pgtype.Timestamptz) pages.CostTabData {
	data := pages.CostTabData{
		UsagePageData: base,
		TopKeyLimit:   5,
	}

	if h.DB == nil {
		return data
	}

	ctx := r.Context()

	// Metrics
	data.Metrics = h.loadUsageMetrics(r, tsStart, tsEnd)

	// Daily spend
	rows, err := h.DB.GetGlobalActivity(ctx, db.GetGlobalActivityParams{
		Starttime:   tsStart,
		Starttime_2: tsEnd,
	})
	if err == nil {
		for _, row := range rows {
			if row.Date.Valid {
				data.DailySpend = append(data.DailySpend, pages.DailySpend{
					Date:  row.Date.Time.Format("2006-01-02"),
					Spend: float64(row.TotalSpend),
				})
			}
		}
		sort.Slice(data.DailySpend, func(i, j int) bool {
			return data.DailySpend[i].Date < data.DailySpend[j].Date
		})
		data.DailySpend = fillMissingDates(data.DailySpend, tsStart.Time, tsEnd.Time)
	}

	// Top keys
	keyRows, err := h.DB.GetTopKeysBySpend(ctx, db.GetTopKeysBySpendParams{
		StartDate:  tsStart,
		EndDate:    tsEnd,
		QueryLimit: int32(data.TopKeyLimit),
	})
	if err == nil {
		data.TopKeys = toTopKeys(keyRows)
	}

	// Top models
	modelRows, err := h.DB.GetTopModelsBySpend(ctx, db.GetTopModelsBySpendParams{
		StartDate:  tsStart,
		EndDate:    tsEnd,
		QueryLimit: 10,
	})
	if err == nil {
		for _, row := range modelRows {
			if row.Model == "" {
				continue
			}
			data.TopModels = append(data.TopModels, pages.TopModel{
				Model:        row.Model,
				TotalSpend:   float64(row.TotalSpend),
				TotalTokens:  row.TotalTokens,
				RequestCount: row.RequestCount,
			})
		}
	}

	return data
}

func (h *UIHandler) loadUsageMetrics(r *http.Request, tsStart, tsEnd pgtype.Timestamptz) pages.UsageMetrics {
	if h.DB == nil {
		return pages.UsageMetrics{}
	}

	ctx := r.Context()

	metricsRow, err := h.DB.GetUsageMetrics(ctx, db.GetUsageMetricsParams{
		StartDate: tsStart,
		EndDate:   tsEnd,
	})
	if err != nil {
		return pages.UsageMetrics{}
	}

	failedCount, err := h.DB.GetFailedRequestCount(ctx, db.GetFailedRequestCountParams{
		StartDate: tsStart,
		EndDate:   tsEnd,
	})
	if err != nil {
		failedCount = 0
	}

	m := pages.UsageMetrics{
		TotalRequests:      metricsRow.TotalRequests,
		FailedRequests:     failedCount,
		SuccessfulRequests: metricsRow.TotalRequests - failedCount,
		TotalTokens:        metricsRow.TotalTokens,
		TotalSpend:         metricsRow.TotalSpend,
	}
	if m.TotalRequests > 0 {
		m.AvgCostPerRequest = m.TotalSpend / float64(m.TotalRequests)
	}
	return m
}

func (h *UIHandler) loadModelActivityData(r *http.Request, base pages.UsagePageData, tsStart, tsEnd pgtype.Timestamptz) pages.ModelActivityData {
	data := pages.ModelActivityData{UsagePageData: base}

	if h.DB == nil {
		return data
	}

	rows, err := h.DB.GetGlobalActivityByModel(r.Context(), db.GetGlobalActivityByModelParams{
		Starttime:   tsStart,
		Starttime_2: tsEnd,
	})
	if err != nil {
		return data
	}

	modelMap := map[string]*pages.ModelDailyActivity{}
	var order []string
	for _, row := range rows {
		if !row.Date.Valid {
			continue
		}
		m, ok := modelMap[row.Model]
		if !ok {
			m = &pages.ModelDailyActivity{Model: row.Model}
			modelMap[row.Model] = m
			order = append(order, row.Model)
		}
		m.DailyData = append(m.DailyData, pages.DailyActivity{
			Date:         row.Date.Time.Format("2006-01-02"),
			RequestCount: row.RequestCount,
			TotalTokens:  row.TotalPromptTokens + row.TotalCompletionTokens,
		})
		m.SumRequests += row.RequestCount
		m.SumTokens += row.TotalPromptTokens + row.TotalCompletionTokens
	}

	for _, model := range order {
		m := modelMap[model]
		sort.Slice(m.DailyData, func(i, j int) bool {
			return m.DailyData[i].Date < m.DailyData[j].Date
		})
		data.Models = append(data.Models, *m)
	}

	return data
}

func (h *UIHandler) loadKeyActivityData(r *http.Request, base pages.UsagePageData, tsStart, tsEnd pgtype.Timestamptz) pages.KeyActivityData {
	data := pages.KeyActivityData{UsagePageData: base}

	if h.DB == nil {
		return data
	}

	rows, err := h.DB.GetDailyActivityByKey(r.Context(), db.GetDailyActivityByKeyParams{
		StartDate: tsStart,
		EndDate:   tsEnd,
		KeyLimit:  10,
	})
	if err != nil {
		return data
	}

	keyMap := map[string]*pages.KeyDailyActivity{}
	var order []string
	for _, row := range rows {
		if !row.Day.Valid {
			continue
		}
		k, ok := keyMap[row.ApiKey]
		if !ok {
			k = &pages.KeyDailyActivity{APIKey: truncateKey(row.ApiKey)}
			keyMap[row.ApiKey] = k
			order = append(order, row.ApiKey)
		}
		k.DailyData = append(k.DailyData, pages.DailyActivity{
			Date:         row.Day.Time.Format("2006-01-02"),
			RequestCount: row.RequestCount,
			TotalTokens:  row.TotalTokens,
		})
		k.SumSpend += float64(row.TotalSpend)
		k.SumRequests += row.RequestCount
	}

	for _, key := range order {
		k := keyMap[key]
		sort.Slice(k.DailyData, func(i, j int) bool {
			return k.DailyData[i].Date < k.DailyData[j].Date
		})
		data.Keys = append(data.Keys, *k)
	}

	return data
}

func (h *UIHandler) loadEndpointActivityData(r *http.Request, base pages.UsagePageData, tsStart, tsEnd pgtype.Timestamptz) pages.EndpointActivityData {
	data := pages.EndpointActivityData{UsagePageData: base}

	if h.DB == nil {
		return data
	}

	rows, err := h.DB.GetDailySpendByCallType(r.Context(), db.GetDailySpendByCallTypeParams{
		StartDate: tsStart,
		EndDate:   tsEnd,
	})
	if err != nil {
		return data
	}

	epMap := map[string]*pages.EndpointDailyActivity{}
	var order []string
	for _, row := range rows {
		if !row.Day.Valid {
			continue
		}
		e, ok := epMap[row.CallType]
		if !ok {
			e = &pages.EndpointDailyActivity{CallType: row.CallType}
			epMap[row.CallType] = e
			order = append(order, row.CallType)
		}
		e.DailyData = append(e.DailyData, pages.DailyActivity{
			Date:         row.Day.Time.Format("2006-01-02"),
			RequestCount: row.RequestCount,
			TotalTokens:  row.TotalTokens,
		})
		e.SumRequests += row.RequestCount
	}

	for _, ct := range order {
		e := epMap[ct]
		sort.Slice(e.DailyData, func(i, j int) bool {
			return e.DailyData[i].Date < e.DailyData[j].Date
		})
		data.Endpoints = append(data.Endpoints, *e)
	}

	return data
}

// --- Helpers ---

func toTopKeys(rows []db.GetTopKeysBySpendRow) []pages.TopKey {
	keys := make([]pages.TopKey, 0, len(rows))
	for _, row := range rows {
		keys = append(keys, pages.TopKey{
			APIKey:       truncateKey(row.ApiKey),
			KeyAlias:     row.KeyAlias,
			TotalSpend:   float64(row.TotalSpend),
			RequestCount: row.RequestCount,
		})
	}
	return keys
}

func truncateKey(key string) string {
	if len(key) <= 12 {
		return key
	}
	return key[:8] + "..."
}

func fillMissingDates(data []pages.DailySpend, start, end time.Time) []pages.DailySpend {
	if len(data) == 0 && start.IsZero() {
		return data
	}

	existing := map[string]float64{}
	for _, d := range data {
		existing[d.Date] = d.Spend
	}

	startDay := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	endDay := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, end.Location())

	var filled []pages.DailySpend
	for d := startDay; !d.After(endDay); d = d.AddDate(0, 0, 1) {
		ds := d.Format("2006-01-02")
		filled = append(filled, pages.DailySpend{
			Date:  ds,
			Spend: existing[ds],
		})
	}
	return filled
}

// FormatCurrency formats a float64 as "$1,234.56".
func FormatCurrency(v float64) string {
	if v == 0 {
		return "$0.00"
	}
	neg := v < 0
	if neg {
		v = -v
	}

	whole := int64(v)
	frac := int64((v - float64(whole)) * 100)

	s := strconv.FormatInt(whole, 10)
	// Insert commas
	if len(s) > 3 {
		var parts []string
		for len(s) > 3 {
			parts = append([]string{s[len(s)-3:]}, parts...)
			s = s[:len(s)-3]
		}
		parts = append([]string{s}, parts...)
		s = ""
		for i, p := range parts {
			if i > 0 {
				s += ","
			}
			s += p
		}
	}

	result := fmt.Sprintf("$%s.%02d", s, frac)
	if neg {
		result = "-" + result
	}
	return result
}

// FormatCompact formats large numbers as "1.5K", "2.3M", "1.1B", etc.
func FormatCompact(v int64) string {
	if v < 0 {
		return "-" + FormatCompact(-v)
	}
	switch {
	case v >= 1_000_000_000:
		return fmt.Sprintf("%.1fB", float64(v)/1_000_000_000)
	case v >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(v)/1_000_000)
	case v >= 1_000:
		return fmt.Sprintf("%.1fK", float64(v)/1_000)
	default:
		return strconv.FormatInt(v, 10)
	}
}

// --- CSV Export ---

func writeCostCSV(cw *csv.Writer, data pages.CostTabData) {
	_ = cw.Write([]string{"Date", "Spend (USD)"})
	for _, d := range data.DailySpend {
		_ = cw.Write([]string{d.Date, fmt.Sprintf("%.6f", d.Spend)})
	}
}

func writeModelActivityCSV(cw *csv.Writer, data pages.ModelActivityData) {
	_ = cw.Write([]string{"Model", "Date", "Requests", "Tokens"})
	for _, m := range data.Models {
		for _, d := range m.DailyData {
			_ = cw.Write([]string{m.Model, d.Date, strconv.FormatInt(d.RequestCount, 10), strconv.FormatInt(d.TotalTokens, 10)})
		}
	}
}

func writeKeyActivityCSV(cw *csv.Writer, data pages.KeyActivityData) {
	_ = cw.Write([]string{"Key", "Date", "Requests", "Spend (USD)", "Tokens"})
	for _, k := range data.Keys {
		for _, d := range k.DailyData {
			_ = cw.Write([]string{k.APIKey, d.Date, strconv.FormatInt(d.RequestCount, 10), fmt.Sprintf("%.6f", k.SumSpend), strconv.FormatInt(d.TotalTokens, 10)})
		}
	}
}

func writeEndpointActivityCSV(cw *csv.Writer, data pages.EndpointActivityData) {
	_ = cw.Write([]string{"Endpoint", "Date", "Requests", "Tokens"})
	for _, e := range data.Endpoints {
		for _, d := range e.DailyData {
			_ = cw.Write([]string{e.CallType, d.Date, strconv.FormatInt(d.RequestCount, 10), strconv.FormatInt(d.TotalTokens, 10)})
		}
	}
}
