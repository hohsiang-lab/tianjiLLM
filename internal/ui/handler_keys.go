package ui

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/ui/components/toast"
	"github.com/praxisllmlab/tianjiLLM/internal/ui/pages"
)

const keysPerPage = 50

func (h *UIHandler) handleKeys(w http.ResponseWriter, r *http.Request) {
	data := h.loadKeysPageData(r)
	render(r.Context(), w, pages.KeysPage(data))
}

func (h *UIHandler) handleKeysTable(w http.ResponseWriter, r *http.Request) {
	data := h.loadKeysPageData(r)
	render(r.Context(), w, pages.KeysTablePartial(data))
}

func (h *UIHandler) loadKeysPageData(r *http.Request) pages.KeysPageData {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	q := r.URL.Query()
	data := pages.KeysPageData{
		Page:           page,
		Search:         q.Get("search"),
		FilterTeamID:   q.Get("team_id"),
		FilterKeyAlias: q.Get("key_alias"),
		FilterUserID:   q.Get("user_id"),
		FilterKeyHash:  q.Get("key_hash"),
	}

	// Load available model names for the models selector (works even without DB — falls back to config).
	data.AvailableModels = h.loadAvailableModelNames(r.Context())

	if h.DB == nil {
		return data
	}

	// Load teams for dropdown
	teams, _ := h.DB.ListTeams(r.Context())
	for _, t := range teams {
		opt := pages.TeamOption{ID: t.TeamID}
		if t.TeamAlias != nil {
			opt.Alias = *t.TeamAlias
		}
		data.Teams = append(data.Teams, opt)
	}

	// Load users for dropdown
	users, _ := h.DB.ListUsers(r.Context())
	for _, u := range users {
		data.Users = append(data.Users, pages.UserOption{ID: u.UserID})
	}

	// Build filter params
	filterParams := db.ListVerificationTokensFilteredParams{
		QueryOffset: int32((page - 1) * keysPerPage),
		QueryLimit:  keysPerPage,
	}
	countParams := db.CountVerificationTokensFilteredParams{}

	if data.FilterTeamID != "" {
		filterParams.FilterTeamID = &data.FilterTeamID
		countParams.FilterTeamID = &data.FilterTeamID
	}
	if data.FilterKeyAlias != "" {
		filterParams.FilterKeyAlias = &data.FilterKeyAlias
		countParams.FilterKeyAlias = &data.FilterKeyAlias
	}
	if data.FilterUserID != "" {
		filterParams.FilterUserID = &data.FilterUserID
		countParams.FilterUserID = &data.FilterUserID
	}
	if data.FilterKeyHash != "" {
		filterParams.FilterToken = &data.FilterKeyHash
		countParams.FilterToken = &data.FilterKeyHash
	}

	// Get total count
	totalCount, err := h.DB.CountVerificationTokensFiltered(r.Context(), countParams)
	if err != nil {
		return data
	}
	data.TotalCount = int(totalCount)
	data.TotalPages = (data.TotalCount + keysPerPage - 1) / keysPerPage
	if data.TotalPages < 1 {
		data.TotalPages = 1
	}

	// Get filtered tokens
	tokens, err := h.DB.ListVerificationTokensFiltered(r.Context(), filterParams)
	if err != nil {
		return data
	}

	// Collect unique team IDs for alias lookup
	teamIDSet := map[string]bool{}
	for _, t := range tokens {
		if t.TeamID != nil && *t.TeamID != "" {
			teamIDSet[*t.TeamID] = true
		}
	}
	teamAliasMap := map[string]string{}
	if len(teamIDSet) > 0 {
		ids := make([]string, 0, len(teamIDSet))
		for id := range teamIDSet {
			ids = append(ids, id)
		}
		aliases, err := h.DB.ListTeamAliases(r.Context(), ids)
		if err == nil {
			for _, a := range aliases {
				if a.TeamAlias != nil {
					teamAliasMap[a.TeamID] = *a.TeamAlias
				}
			}
		}
	}

	// Convert DB rows to UI rows
	for _, t := range tokens {
		row := pages.KeyRow{
			Token:     t.Token,
			Spend:     t.Spend,
			Models:    t.Models,
			MaxBudget: t.MaxBudget,
		}
		if t.KeyName != nil {
			row.KeyName = *t.KeyName
		}
		if t.KeyAlias != nil {
			row.KeyAlias = *t.KeyAlias
		}
		if t.Blocked != nil {
			row.Blocked = *t.Blocked
		}
		if t.CreatedAt.Valid {
			row.CreatedAt = t.CreatedAt.Time
		}
		if t.Expires.Valid {
			row.Expires = &t.Expires.Time
		}
		if t.TeamID != nil {
			row.TeamID = *t.TeamID
			row.TeamAlias = teamAliasMap[*t.TeamID]
		}
		if t.UserID != nil {
			row.UserID = *t.UserID
		}
		row.TPMLimit = t.TpmLimit
		row.RPMLimit = t.RpmLimit
		if t.BudgetDuration != nil {
			row.BudgetDuration = *t.BudgetDuration
		}
		if t.BudgetResetAt.Valid {
			row.BudgetResetAt = &t.BudgetResetAt.Time
		}

		data.Keys = append(data.Keys, row)
	}

	return data
}

func (h *UIHandler) handleKeyCreate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	keyAlias := r.FormValue("key_alias")
	maxBudgetStr := r.FormValue("max_budget")
	budgetDuration := r.FormValue("budget_duration")
	tpmStr := r.FormValue("tpm_limit")
	rpmStr := r.FormValue("rpm_limit")
	teamID := r.FormValue("team_id")
	userID := r.FormValue("user_id")
	durationStr := r.FormValue("duration")
	metadataStr := r.FormValue("metadata")

	// Validate key_alias required
	if strings.TrimSpace(keyAlias) == "" {
		data := h.loadKeysPageData(r)
		render(r.Context(), w, pages.KeysTableWithToast(data, "Key alias is required", toast.VariantError))
		return
	}

	// Check alias uniqueness
	if _, aliasErr := h.DB.GetVerificationTokenByAlias(r.Context(), db.GetVerificationTokenByAliasParams{
		Alias: &keyAlias,
	}); aliasErr == nil {
		data := h.loadKeysPageData(r)
		render(r.Context(), w, pages.KeysTableWithToast(data, "Key alias already exists", toast.VariantError))
		return
	}

	// Generate raw key and hash
	rawKey := generateAPIKey()
	hashedKey := hashKey(rawKey)

	maxBudget := parseOptionalFloat(maxBudgetStr)

	// Parse model selection from multi-select checkboxes.
	// all_models="1" means unrestricted (no model restriction).
	// all_models="0" reads r.Form["models"] (repeated checkbox values).
	// Fallback: if user unchecks All Models but selects nothing, treat as unrestricted (by design per FR-008)
	models := parseModelSelection(r.FormValue("all_models"), r.Form["models"])

	tpmLimit := parseOptionalInt64(tpmStr)
	rpmLimit := parseOptionalInt64(rpmStr)

	var budgetDurationPtr *string
	if budgetDuration != "" {
		budgetDurationPtr = &budgetDuration
	}

	var teamIDPtr *string
	if teamID != "" {
		teamIDPtr = &teamID
	}

	var userIDPtr *string
	if userID != "" {
		userIDPtr = &userID
	}

	// Parse duration → expires
	var expires time.Time
	if durationStr != "" {
		dur := parseDuration(durationStr)
		if dur > 0 {
			expires = time.Now().Add(dur)
		}
	}

	// Build metadata
	meta := map[string]any{"generated_by": "ui"}
	if metadataStr != "" {
		var custom map[string]any
		if json.Unmarshal([]byte(metadataStr), &custom) == nil {
			for k, v := range custom {
				meta[k] = v
			}
		}
	}

	params := db.CreateVerificationTokenParams{
		Token:          hashedKey,
		KeyName:        &keyAlias,
		KeyAlias:       &keyAlias,
		Spend:          0,
		MaxBudget:      maxBudget,
		Models:         models,
		UserID:         userIDPtr,
		TeamID:         teamIDPtr,
		Permissions:    []byte("{}"),
		Metadata:       mustJSON(meta),
		TpmLimit:       tpmLimit,
		RpmLimit:       rpmLimit,
		BudgetDuration: budgetDurationPtr,
	}
	if !expires.IsZero() {
		params.Expires.Time = expires
		params.Expires.Valid = true
	}

	_, err := h.DB.CreateVerificationToken(r.Context(), params)
	if err != nil {
		data := h.loadKeysPageData(r)
		render(r.Context(), w, pages.KeysTableWithToast(data, "Failed to create key: "+err.Error(), toast.VariantError))
		return
	}

	data := h.loadKeysPageData(r)
	render(r.Context(), w, pages.KeysTableWithKeyReveal(data, rawKey))
}

func (h *UIHandler) handleKeyDelete(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	token := r.FormValue("token")
	if token == "" {
		http.Error(w, "token required", http.StatusBadRequest)
		return
	}

	_ = h.DB.DeleteVerificationToken(r.Context(), token)

	data := h.loadKeysPageData(r)
	render(r.Context(), w, pages.KeysTableWithToast(data, "Key deleted successfully", toast.VariantSuccess))
}

func (h *UIHandler) handleKeyBlock(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	token := r.FormValue("token")
	if token == "" {
		http.Error(w, "token required", http.StatusBadRequest)
		return
	}

	if err := h.DB.BlockVerificationToken(r.Context(), token); err != nil {
		data := h.loadKeysPageData(r)
		render(r.Context(), w, pages.KeysTableWithToast(data, "Failed to block key: "+err.Error(), toast.VariantError))
		return
	}

	data := h.loadKeysPageData(r)
	render(r.Context(), w, pages.KeysTableWithToast(data, "Key blocked successfully", toast.VariantSuccess))
}

func (h *UIHandler) handleKeyUnblock(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	token := r.FormValue("token")
	if token == "" {
		http.Error(w, "token required", http.StatusBadRequest)
		return
	}

	if err := h.DB.UnblockVerificationToken(r.Context(), token); err != nil {
		data := h.loadKeysPageData(r)
		render(r.Context(), w, pages.KeysTableWithToast(data, "Failed to unblock key: "+err.Error(), toast.VariantError))
		return
	}

	data := h.loadKeysPageData(r)
	render(r.Context(), w, pages.KeysTableWithToast(data, "Key unblocked successfully", toast.VariantSuccess))
}

// --- Key Detail handlers ---

func (h *UIHandler) handleKeyDetail(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")

	if h.DB == nil {
		render(r.Context(), w, pages.KeyNotFoundPage())
		return
	}

	vt, err := h.DB.GetVerificationToken(r.Context(), token)
	if err != nil {
		render(r.Context(), w, pages.KeyNotFoundPage())
		return
	}

	data := buildKeyDetailData(vt)
	data.Teams, data.Users = h.loadTeamsAndUsers(r)
	data.AvailableModels = h.loadAvailableModelNames(r.Context())
	render(r.Context(), w, pages.KeyDetailPage(data))
}

func (h *UIHandler) handleKeyEdit(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")

	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	vt, err := h.DB.GetVerificationToken(r.Context(), token)
	if err != nil {
		http.Error(w, "key not found", http.StatusNotFound)
		return
	}

	data := buildKeyDetailData(vt)
	data.Teams, data.Users = h.loadTeamsAndUsers(r)
	data.AvailableModels = h.loadAvailableModelNames(r.Context())
	render(r.Context(), w, pages.EditSettingsForm(data))
}

func (h *UIHandler) handleKeySettings(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")

	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	vt, err := h.DB.GetVerificationToken(r.Context(), token)
	if err != nil {
		http.Error(w, "key not found", http.StatusNotFound)
		return
	}

	data := buildKeyDetailData(vt)
	render(r.Context(), w, pages.SettingsTab(data))
}

func (h *UIHandler) handleKeyUpdate(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")

	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	keyAlias := r.FormValue("key_alias")
	maxBudgetStr := r.FormValue("max_budget")
	budgetDuration := r.FormValue("budget_duration")
	tpmStr := r.FormValue("tpm_limit")
	rpmStr := r.FormValue("rpm_limit")
	metadataStr := r.FormValue("metadata")

	params := db.UpdateVerificationTokenParams{Token: token}

	if keyAlias != "" {
		params.KeyAlias = &keyAlias
	}
	params.MaxBudget = parseOptionalFloat(maxBudgetStr)
	if budgetDuration != "" {
		params.BudgetDuration = &budgetDuration
	}
	params.TpmLimit = parseOptionalInt64(tpmStr)
	params.RpmLimit = parseOptionalInt64(rpmStr)

	// Parse model selection from multi-select checkboxes.
	// all_models="1" means unrestricted; "0" reads r.Form["models"].
	params.Models = parseModelSelection(r.FormValue("all_models"), r.Form["models"])

	if metadataStr != "" {
		var v any
		if json.Unmarshal([]byte(metadataStr), &v) == nil {
			params.Metadata = []byte(metadataStr)
		}
	}

	_, err := h.DB.UpdateVerificationToken(r.Context(), params)
	if err != nil {
		vt, _ := h.DB.GetVerificationToken(r.Context(), token)
		data := buildKeyDetailData(vt)
		data.Teams, data.Users = h.loadTeamsAndUsers(r)
		data.AvailableModels = h.loadAvailableModelNames(r.Context())
		render(r.Context(), w, pages.EditSettingsFormWithToast(data, "Failed to update key: "+err.Error(), toast.VariantError))
		return
	}

	vt, _ := h.DB.GetVerificationToken(r.Context(), token)
	data := buildKeyDetailData(vt)
	render(r.Context(), w, pages.SettingsTabWithToast(data, "Settings updated successfully", toast.VariantSuccess))
}

func (h *UIHandler) handleKeyDetailDelete(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")

	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	_ = h.DB.DeleteVerificationToken(r.Context(), token)

	w.Header().Set("HX-Redirect", "/ui/keys")
	w.WriteHeader(http.StatusOK)
}

func (h *UIHandler) handleKeyRegenerate(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")

	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	rawKey := generateAPIKey()
	newHash := hashKey(rawKey)

	params := db.RegenerateVerificationTokenWithParamsParams{
		OldToken: token,
		NewToken: newHash,
	}

	if v := r.FormValue("max_budget"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err == nil {
			params.NewMaxBudget = &f
		}
	}
	if v := r.FormValue("tpm_limit"); v != "" {
		i, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			params.NewTpmLimit = &i
		}
	}
	if v := r.FormValue("rpm_limit"); v != "" {
		i, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			params.NewRpmLimit = &i
		}
	}
	if v := r.FormValue("budget_duration"); v != "" {
		params.NewBudgetDuration = &v
	}

	_, err := h.DB.RegenerateVerificationTokenWithParams(r.Context(), params)
	if err != nil {
		http.Error(w, "failed to regenerate: "+err.Error(), http.StatusInternalServerError)
		return
	}

	render(r.Context(), w, pages.RegenerateResultDialog(rawKey))
}

// --- helpers ---

// loadAvailableModelNames returns deduplicated model names from DB + YAML config.
// Follows the same merge logic as loadModelsPageData in handler_models.go.
func (h *UIHandler) loadAvailableModelNames(ctx context.Context) []string {
	seen := map[string]struct{}{}
	var names []string

	// DB models (authoritative source when DB is available).
	if h.DB != nil {
		rows, err := h.DB.ListProxyModels(ctx)
		if err == nil {
			for _, m := range rows {
				if m.ModelName == "" {
					continue
				}
				if _, ok := seen[m.ModelName]; !ok {
					seen[m.ModelName] = struct{}{}
					names = append(names, m.ModelName)
				}
			}
		}
	}

	// YAML config models (fill in any not already in DB list).
	if h.Config != nil {
		for _, m := range h.Config.ModelList {
			if m.ModelName == "" {
				continue
			}
			if _, ok := seen[m.ModelName]; !ok {
				seen[m.ModelName] = struct{}{}
				names = append(names, m.ModelName)
			}
		}
	}

	if names == nil {
		return []string{}
	}
	return names
}

func (h *UIHandler) loadTeamsAndUsers(r *http.Request) ([]pages.TeamOption, []pages.UserOption) {
	var teamOpts []pages.TeamOption
	var userOpts []pages.UserOption

	if h.DB == nil {
		return teamOpts, userOpts
	}

	teams, _ := h.DB.ListTeams(r.Context())
	for _, t := range teams {
		opt := pages.TeamOption{ID: t.TeamID}
		if t.TeamAlias != nil {
			opt.Alias = *t.TeamAlias
		}
		teamOpts = append(teamOpts, opt)
	}

	users, _ := h.DB.ListUsers(r.Context())
	for _, u := range users {
		userOpts = append(userOpts, pages.UserOption{ID: u.UserID})
	}

	return teamOpts, userOpts
}

func buildKeyDetailData(vt db.VerificationToken) pages.KeyDetailData {
	data := pages.KeyDetailData{
		Token:          vt.Token,
		KeyName:        vt.KeyName,
		KeyAlias:       vt.KeyAlias,
		Spend:          vt.Spend,
		MaxBudget:      vt.MaxBudget,
		Models:         vt.Models,
		UserID:         vt.UserID,
		TeamID:         vt.TeamID,
		OrganizationID: vt.OrganizationID,
		Blocked:        vt.Blocked,
		TPMLimit:       vt.TpmLimit,
		RPMLimit:       vt.RpmLimit,
		BudgetDuration: vt.BudgetDuration,
	}

	if vt.Expires.Valid {
		data.Expires = &vt.Expires.Time
	}
	if vt.BudgetResetAt.Valid {
		data.BudgetResetAt = &vt.BudgetResetAt.Time
	}
	if vt.CreatedAt.Valid {
		data.CreatedAt = vt.CreatedAt.Time
	}
	if vt.UpdatedAt.Valid {
		data.UpdatedAt = vt.UpdatedAt.Time
	}
	data.CreatedBy = vt.CreatedBy
	data.UpdatedBy = vt.UpdatedBy

	// Metadata
	if len(vt.Metadata) > 0 {
		data.Metadata = string(vt.Metadata)
	}

	// Computed fields
	data.IsExpired = data.Expires != nil && data.Expires.Before(time.Now())
	data.IsBlocked = vt.Blocked != nil && *vt.Blocked

	if vt.KeyAlias != nil && *vt.KeyAlias != "" {
		data.DisplayAlias = *vt.KeyAlias
	} else {
		data.DisplayAlias = "Virtual Key"
	}

	if vt.MaxBudget != nil && *vt.MaxBudget > 0 {
		data.BudgetProgress = (vt.Spend / *vt.MaxBudget) * 100
		if data.BudgetProgress > 100 {
			data.BudgetProgress = 100
		}
	}

	return data
}


func parseDuration(s string) time.Duration {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	last := s[len(s)-1]
	numStr := s[:len(s)-1]
	n, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil || n <= 0 {
		return 0
	}
	switch last {
	case 's':
		return time.Duration(n) * time.Second
	case 'm':
		return time.Duration(n) * time.Minute
	case 'h':
		return time.Duration(n) * time.Hour
	case 'd':
		return time.Duration(n) * 24 * time.Hour
	}
	return 0
}

func generateAPIKey() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return "sk-" + hex.EncodeToString(b)
}

func hashKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func mustJSON(v any) []byte {
	data, _ := json.Marshal(v)
	return data
}

func parseOptionalFloat(s string) *float64 {
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &v
}

func parseOptionalInt64(s string) *int64 {
	if s == "" {
		return nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil
	}
	return &v
}

// parseCSV has been removed: the models multi-select uses r.Form["models"] directly.

// parseModelSelection converts multi-select form values into the models slice to store.
// allModels is the value of the "all_models" hidden field ("1" = unrestricted, "0" = specific).
// formModels is the r.Form["models"] repeated values from checked checkboxes.
// Returns []string{} for unrestricted (empty means no model restriction in the DB schema).
func parseModelSelection(allModels string, formModels []string) []string {
	if allModels == "1" {
		return []string{}
	}
	if len(formModels) == 0 {
		// Fallback: all_models=0 but no individual models selected → treat as unrestricted (per FR-008).
		return []string{}
	}
	return formModels
}
