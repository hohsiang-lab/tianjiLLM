package ui

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

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

	identities, err := h.DB.ListUserIdentitiesByUser(ctx, userID)
	if err != nil {
		return pages.UserDetailData{}, false
	}
	identityRows := make([]pages.UserIdentityRow, 0, len(identities))
	for _, identity := range identities {
		identityRow := pages.UserIdentityRow{
			IdentityID:     identity.IdentityID,
			Provider:       identity.Provider,
			ProviderUserID: identity.ProviderUserID,
			EmailVerified:  identity.EmailVerified,
		}
		if identity.ProviderEmail != nil {
			identityRow.ProviderEmail = *identity.ProviderEmail
		}
		if identity.DisplayName != nil {
			identityRow.DisplayName = *identity.DisplayName
		}
		if identity.LastLoginAt.Valid {
			identityRow.LastLoginAt = identity.LastLoginAt.Time
		}
		identityRows = append(identityRows, identityRow)
	}

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
		User:                        row,
		Teams:                       teamRows,
		Identities:                  identityRows,
		SocialProviders:             h.socialProviders(),
		SocialAuthEnrollmentEnabled: userSocialAuthEnrollmentEnabled(u.Metadata),
		Metadata:                    metadataStr,
		Status:                      status,
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

	err := h.withActiveAdminMutationLock(r.Context(), func(q *db.Queries) error {
		current, err := q.GetUser(r.Context(), userID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errUserNotFound
		}
		if err != nil {
			return err
		}

		newRole := strings.TrimSpace(r.FormValue("user_role"))
		if newRole == "" {
			newRole = current.UserRole
		}
		if !isAssignableUIRole(newRole) {
			return errInvalidUserRole
		}

		if current.UserRole == "proxy_admin" &&
			userStatusFromMetadata(current.Metadata) == "active" &&
			newRole != "proxy_admin" {
			count, countErr := q.CountUsersByRole(r.Context(), "proxy_admin")
			if countErr != nil {
				return countErr
			}
			if count <= 1 {
				return errLastActiveAdmin
			}
		}

		_, err = q.UpdateUser(r.Context(), db.UpdateUserParams{
			UserID:    userID,
			UserAlias: userAliasPtr,
			UserEmail: userEmailPtr,
			UserRole:  newRole,
			MaxBudget: maxBudget,
			Models:    current.Models,
			TpmLimit:  tpmLimit,
			RpmLimit:  rpmLimit,
			UpdatedBy: "admin",
		})
		return err
	})
	if errors.Is(err, errInvalidUserRole) {
		http.Error(w, "invalid user role", http.StatusBadRequest)
		return
	}
	if errors.Is(err, errUserNotFound) {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	if errors.Is(err, errLastActiveAdmin) {
		data, ok := h.loadUserDetailData(r, userID)
		if !ok {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		render(r.Context(), w, pages.UserDetailHeaderWithToast(data, "Cannot change role: this is the last admin user", toast.VariantError))
		return
	}
	if err != nil {
		http.Error(w, "failed to verify admin safety", http.StatusInternalServerError)
		return
	}
	data, ok := h.loadUserDetailData(r, userID)
	if !ok {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	render(r.Context(), w, pages.UserDetailHeaderWithToast(data, "User updated successfully", toast.VariantSuccess))
}
