package ui

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/ui/components/toast"
	"github.com/praxisllmlab/tianjiLLM/internal/ui/pages"
)

func (h *UIHandler) loadUserDetailData(r *http.Request, userID string) (pages.UserDetailData, bool) {
	if h.DB == nil {
		return pages.UserDetailData{}, false
	}

	ctx := r.Context()

	u, err := h.DB.GetUser(ctx, userID)
	if err != nil {
		return pages.UserDetailData{}, false
	}

	// Check if soft-deleted
	status := userStatusFromMetadata(u.Metadata)
	if status == "deleted" {
		return pages.UserDetailData{}, false
	}

	row := userTableRowFromDB(u)

	// Load team details
	var teamRows []pages.UserTeamRow
	for _, teamID := range u.Teams {
		t, err := h.DB.GetTeam(ctx, teamID)
		if err != nil {
			teamRows = append(teamRows, pages.UserTeamRow{TeamID: teamID})
			continue
		}
		alias := ""
		if t.TeamAlias != nil {
			alias = *t.TeamAlias
		}
		teamRows = append(teamRows, pages.UserTeamRow{TeamID: teamID, TeamAlias: alias})
	}

	// Pretty-print metadata
	metadataStr := "{}"
	if len(u.Metadata) > 0 {
		var v any
		if json.Unmarshal(u.Metadata, &v) == nil {
			b, _ := json.MarshalIndent(v, "", "  ")
			metadataStr = string(b)
		} else {
			metadataStr = string(u.Metadata)
		}
	}

	return pages.UserDetailData{
		User:     row,
		Teams:    teamRows,
		Metadata: metadataStr,
		Status:   status,
	}, true
}

func (h *UIHandler) handleUserDetail(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "user_id")

	data, ok := h.loadUserDetailData(r, userID)
	if !ok {
		http.Redirect(w, r, "/ui/users", http.StatusSeeOther)
		return
	}

	render(r.Context(), w, pages.UserDetailPage(data))
}

func (h *UIHandler) handleUserEdit(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "user_id")

	data, ok := h.loadUserDetailData(r, userID)
	if !ok {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	render(r.Context(), w, pages.UserDetailHeader(data))
}

func (h *UIHandler) handleUserUpdate(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "user_id")

	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Load current user for last admin protection
	current, err := h.DB.GetUser(r.Context(), userID)
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	newRole := r.FormValue("user_role")
	if newRole == "" {
		newRole = current.UserRole
	}

	// Last admin protection: if changing FROM proxy_admin to something else
	if current.UserRole == "proxy_admin" && newRole != "proxy_admin" {
		count, err := h.DB.CountUsersByRole(r.Context(), "proxy_admin")
		if err == nil && count <= 1 {
			data, ok := h.loadUserDetailData(r, userID)
			if !ok {
				http.Error(w, "user not found", http.StatusNotFound)
				return
			}
			render(r.Context(), w, pages.UserDetailHeaderWithToast(data, "Cannot change role: this is the last admin user", toast.VariantError))
			return
		}
	}

	userAlias := strings.TrimSpace(r.FormValue("user_alias"))
	var userAliasPtr *string
	if userAlias != "" {
		userAliasPtr = &userAlias
	}

	userEmail := strings.TrimSpace(r.FormValue("user_email"))
	var userEmailPtr *string
	if userEmail != "" {
		userEmailPtr = &userEmail
	}

	maxBudget := parseOptionalFloat(r.FormValue("max_budget"))
	tpmLimit := parseOptionalInt64(r.FormValue("tpm_limit"))
	rpmLimit := parseOptionalInt64(r.FormValue("rpm_limit"))

	params := db.UpdateUserParams{
		UserID:    userID,
		UserAlias: userAliasPtr,
		UserEmail: userEmailPtr,
		UserRole:  newRole,
		MaxBudget: maxBudget,
		Models:    current.Models,
		TpmLimit:  tpmLimit,
		RpmLimit:  rpmLimit,
		UpdatedBy: "admin",
	}

	if _, err := h.DB.UpdateUser(r.Context(), params); err != nil {
		data, ok := h.loadUserDetailData(r, userID)
		if !ok {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		render(r.Context(), w, pages.UserDetailHeaderWithToast(data, "Failed to update user: "+err.Error(), toast.VariantError))
		return
	}

	data, ok := h.loadUserDetailData(r, userID)
	if !ok {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	render(r.Context(), w, pages.UserDetailHeaderWithToast(data, "User updated successfully", toast.VariantSuccess))
}
