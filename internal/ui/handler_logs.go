package ui

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
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
	if v := q.Get("upstream_token"); v != "" {
		data.FilterUpstreamToken = &v
	}

	if h.DB == nil {
		return data
	}

	sd := pgtype.Timestamptz{Time: startDate, Valid: true}
	ed := pgtype.Timestamptz{Time: endDate, Valid: true}

	countParams := db.CountRequestLogsParams{
		StartDate:           sd,
		EndDate:             ed,
		FilterApiKey:        data.FilterApiKey,
		FilterTeamID:        data.FilterTeamID,
		FilterModel:         data.FilterModel,
		FilterRequestID:     data.FilterRequestID,
		FilterUpstreamToken: data.FilterUpstreamToken,
		FilterStatus:        data.FilterStatus,
	}
	totalCount, err := h.DB.CountRequestLogs(r.Context(), countParams)
	if err != nil {
		log.Printf("error: failed to count request logs: %v", err)
		data.Error = "database query failed"
		return data
	}
	data.TotalCount = int(totalCount)
	data.TotalPages = (data.TotalCount + logsPerPage - 1) / logsPerPage
	if data.TotalPages < 1 {
		data.TotalPages = 1
	}

	listParams := db.ListRequestLogsParams{
		StartDate:           sd,
		EndDate:             ed,
		FilterApiKey:        data.FilterApiKey,
		FilterTeamID:        data.FilterTeamID,
		FilterModel:         data.FilterModel,
		FilterRequestID:     data.FilterRequestID,
		FilterUpstreamToken: data.FilterUpstreamToken,
		FilterStatus:        data.FilterStatus,
		QueryOffset:         int32((page - 1) * logsPerPage),
		QueryLimit:          logsPerPage,
	}
	rows, err := h.DB.ListRequestLogs(r.Context(), listParams)
	if err != nil {
		log.Printf("error: failed to list request logs: %v", err)
		data.Error = "database query failed"
		return data
	}

	for _, row := range rows {
		data.Logs = append(data.Logs, toLogRow(row))
	}

	return data
}

// extractProvider returns the provider prefix from "provider/model" format, or nil.
func extractProvider(model string) *string {
	if idx := strings.Index(model, "/"); idx > 0 {
		p := model[:idx]
		return &p
	}
	return nil
}

// resolveProvider prefers the stored provider field; falls back to parsing the model name.
func resolveProvider(stored string, model string) *string {
	if stored != "" {
		return &stored
	}
	return extractProvider(model)
}

func toLogRow(row db.ListRequestLogsRow) pages.RequestLogRow {
	lr := pages.RequestLogRow{
		RequestID:        row.RequestID,
		Model:            row.Model,
		ReasoningEffort:  row.ReasoningEffort,
		Spend:            row.Spend,
		TotalTokens:      int(row.TotalTokens),
		PromptTokens:     int(row.PromptTokens),
		CompletionTokens: int(row.CompletionTokens),
		CacheHit:         row.CacheHit,
		KeyHash:          row.KeyHash,
		KeyAlias:         row.KeyAlias,
		UpstreamTokenKey: row.UpstreamTokenKey,
		TeamID:           row.TeamID,
		EndUser:          row.EndUser,
		Provider:         extractProvider(row.Model),
	}

	if row.Ts.Valid {
		lr.Timestamp = row.Ts.Time
	}
	if row.Ts.Valid && row.Endtime.Valid {
		lr.DurationSec = row.Endtime.Time.Sub(row.Ts.Time).Seconds()
	}

	if row.ErrorStatusCode != nil {
		lr.Status = "Failed"
		lr.StatusCode = row.ErrorStatusCode
		lr.ErrorType = row.ErrorType
	} else {
		lr.Status = "Success"
	}

	return lr
}

func (h *UIHandler) handleLogDetail(w http.ResponseWriter, r *http.Request) {
	requestID := r.URL.Query().Get("request_id")
	if requestID == "" {
		http.Error(w, "request_id is required", http.StatusBadRequest)
		return
	}
	if len(requestID) > 128 {
		http.Error(w, "request_id too long", http.StatusBadRequest)
		return
	}
	if h.DB == nil {
		http.Error(w, "database not available", http.StatusInternalServerError)
		return
	}

	row, err := h.DB.GetSpendLogDetail(r.Context(), requestID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "log not found", http.StatusNotFound)
		} else {
			log.Printf("error: failed to load log detail for request_id=%q: %v", requestID, err)
			http.Error(w, "failed to load log details", http.StatusInternalServerError)
		}
		return
	}

	status := "Success"
	if row.ErrorStatusCode != nil {
		status = "Failed"
	}

	data := pages.LogDetailData{
		RequestID:        row.RequestID,
		CallType:         row.CallType,
		Status:           status,
		Model:            row.Model,
		ReasoningEffort:  row.ReasoningEffort,
		Provider:         resolveProvider(row.Provider, row.Model),
		ApiBase:          row.ApiBase,
		Spend:            row.Spend,
		TotalTokens:      int(row.TotalTokens),
		PromptTokens:     int(row.PromptTokens),
		CompletionTokens: int(row.CompletionTokens),
		CacheHit:         row.CacheHit,
		KeyHash:          row.ApiKey,
		KeyAlias:         row.KeyAlias,
		UpstreamTokenKey: row.UpstreamTokenKey,
		TeamID:           row.TeamID,
		EndUser:          row.EndUser,
		User:             row.User,
		Metadata:         string(row.Metadata),
		ErrorStatusCode:  row.ErrorStatusCode,
		ErrorType:        row.ErrorType,
		ErrorMessage:     row.ErrorMessage,
		ErrorTraceback:   row.ErrorTraceback,
	}

	if row.Starttime.Valid {
		data.StartTime = row.Starttime.Time
	}
	if row.Endtime.Valid {
		data.EndTime = row.Endtime.Time
	}
	if row.RequestDurationMs != nil {
		data.DurationSec = float64(*row.RequestDurationMs) / 1000.0
	} else if row.Starttime.Valid && row.Endtime.Valid {
		data.DurationSec = row.Endtime.Time.Sub(row.Starttime.Time).Seconds()
	}

	// Payload (if store_prompts_in_spend_logs enabled)
	if h.Config != nil && h.Config.GeneralSettings.StorePromptsInSpendLogs {
		data.StorePromptsEnabled = true
		payload, err := h.DB.GetRequestPayload(r.Context(), requestID)
		switch {
		case err == nil:
			data.PayloadMessages = payload.Messages
			data.PayloadResponse = payload.Response
		case errors.Is(err, pgx.ErrNoRows):
			// no payload stored for this request — expected for pre-migration data
		default:
			log.Printf("error: failed to load payload for request_id=%q: %v", requestID, err)
		}
	}

	render(r.Context(), w, pages.LogDetailPanel(data))
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
