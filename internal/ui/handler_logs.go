package ui

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/ui/pages"
)

const logsPerPage = 50

func (h *UIHandler) handleLogs(w http.ResponseWriter, r *http.Request) {
	data := h.loadLogsPageData(r)
	render(r.Context(), w, pages.LogsPage(data))
}

func (h *UIHandler) handleLogsTable(w http.ResponseWriter, r *http.Request) {
	data := h.loadLogsPageData(r)
	w.Header().Set("HX-Push-Url", "/ui/logs?"+data.FilterQueryString())
	render(r.Context(), w, pages.LogsTablePartial(data))
}

func (h *UIHandler) loadLogsPageData(r *http.Request) pages.LogsPageData {
	q := r.URL.Query()

	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}

	timeRange := q.Get("time_range")
	if timeRange == "" {
		timeRange = "24h"
	}
	startDate, endDate := timeRangeToDates(timeRange)

	liveTail := q.Get("live_tail") == "true"

	data := pages.LogsPageData{
		Page:      page,
		TimeRange: timeRange,
		LiveTail:  liveTail,
	}

	// Parse optional filters
	if v := q.Get("status"); v == "success" || v == "failed" {
		data.FilterStatus = &v
	}
	if v := q.Get("model"); v != "" {
		data.FilterModel = &v
	}
	if v := q.Get("api_key"); v != "" {
		data.FilterApiKey = &v
	}
	if v := q.Get("team_id"); v != "" {
		data.FilterTeamID = &v
	}
	if v := q.Get("request_id"); v != "" {
		data.FilterRequestID = &v
	}

	if h.DB == nil {
		return data
	}

	sd := pgtype.Timestamptz{Time: startDate, Valid: true}
	ed := pgtype.Timestamptz{Time: endDate, Valid: true}

	countParams := db.CountRequestLogsParams{
		StartDate:       sd,
		EndDate:         ed,
		FilterApiKey:    data.FilterApiKey,
		FilterTeamID:    data.FilterTeamID,
		FilterModel:     data.FilterModel,
		FilterRequestID: data.FilterRequestID,
		FilterStatus:    data.FilterStatus,
	}
	totalCount, err := h.DB.CountRequestLogs(r.Context(), countParams)
	if err != nil {
		return data
	}
	data.TotalCount = int(totalCount)
	data.TotalPages = (data.TotalCount + logsPerPage - 1) / logsPerPage
	if data.TotalPages < 1 {
		data.TotalPages = 1
	}

	listParams := db.ListRequestLogsParams{
		StartDate:       sd,
		EndDate:         ed,
		FilterApiKey:    data.FilterApiKey,
		FilterTeamID:    data.FilterTeamID,
		FilterModel:     data.FilterModel,
		FilterRequestID: data.FilterRequestID,
		FilterStatus:    data.FilterStatus,
		QueryOffset:     int32((page - 1) * logsPerPage),
		QueryLimit:      logsPerPage,
	}
	rows, err := h.DB.ListRequestLogs(r.Context(), listParams)
	if err != nil {
		return data
	}

	for _, row := range rows {
		data.Logs = append(data.Logs, toLogRow(row))
	}

	return data
}

func toLogRow(row db.ListRequestLogsRow) pages.RequestLogRow {
	lr := pages.RequestLogRow{
		RequestID:        row.RequestID,
		Model:            row.Model,
		Spend:            row.Spend,
		TotalTokens:      int(row.TotalTokens),
		PromptTokens:     int(row.PromptTokens),
		CompletionTokens: int(row.CompletionTokens),
		CacheHit:         row.CacheHit,
		KeyHash:          row.ApiKey,
		TeamID:           row.TeamID,
		EndUser:          row.EndUser,
	}

	if row.Starttime.Valid {
		lr.Timestamp = row.Starttime.Time
	}

	// Duration = endtime - starttime
	if row.Starttime.Valid && row.Endtime.Valid {
		lr.DurationSec = row.Endtime.Time.Sub(row.Starttime.Time).Seconds()
	}

	// Status: Failed if ErrorLogs row exists
	if row.ErrorStatusCode != nil {
		lr.Status = "Failed"
		lr.StatusCode = row.ErrorStatusCode
		lr.ErrorType = row.ErrorType
	} else {
		lr.Status = "Success"
	}

	// Provider: extract from "provider/model" format
	if idx := strings.Index(row.Model, "/"); idx > 0 {
		p := row.Model[:idx]
		lr.Provider = &p
	}

	return lr
}

func timeRangeToDates(tr string) (start, end time.Time) {
	end = time.Now()
	switch tr {
	case "1h":
		start = end.Add(-1 * time.Hour)
	case "7d":
		start = end.Add(-7 * 24 * time.Hour)
	case "30d":
		start = end.Add(-30 * 24 * time.Hour)
	default: // "24h"
		start = end.Add(-24 * time.Hour)
	}
	return start, end
}
