package ui

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/ui/components/toast"
	"github.com/praxisllmlab/tianjiLLM/internal/ui/pages"
)

var (
	errUserNotFound    = errors.New("user not found")
	errLastActiveAdmin = errors.New("cannot mutate the last active admin")
	errInvalidUserRole = errors.New("invalid user role")
)

func (h *UIHandler) handleUsers(w http.ResponseWriter, r *http.Request) {
	data := h.loadUsersPageData(r)
	render(r.Context(), w, pages.UsersPage(data))
}

func (h *UIHandler) handleUsersTable(w http.ResponseWriter, r *http.Request) {
	data := h.loadUsersPageData(r)
	render(r.Context(), w, pages.UsersTablePartial(data))
}

func (h *UIHandler) loadUsersPageData(r *http.Request) pages.UsersPageData {
	page := parsePage(r.URL.Query().Get("page"))
	search := r.URL.Query().Get("search")
	filterRole := r.URL.Query().Get("filter_role")
	filterStatus := r.URL.Query().Get("filter_status")

	data := pages.UsersPageData{
		Page:         page,
		PerPage:      50,
		Search:       search,
		FilterRole:   filterRole,
		FilterStatus: filterStatus,
	}

	if h.DB == nil {
		return data
	}

	ctx := r.Context()

	// Count total matching users
	count, err := h.DB.CountUsers(ctx, db.CountUsersParams{
		Search:       search,
		RoleFilter:   filterRole,
		StatusFilter: filterStatus,
	})
	if err != nil {
		return data
	}
	data.TotalCount = int(count)
	data.TotalPages = (data.TotalCount + data.PerPage - 1) / data.PerPage
	if data.TotalPages < 1 {
		data.TotalPages = 1
	}

	// Load paginated users
	offset := (page - 1) * data.PerPage
	users, err := h.DB.ListUsersPaginated(ctx, db.ListUsersPaginatedParams{
		Search:       search,
		RoleFilter:   filterRole,
		StatusFilter: filterStatus,
		Limit:        int32(data.PerPage),
		Offset:       int32(offset),
	})
	if err != nil {
		return data
	}

	for _, u := range users {
		data.Users = append(data.Users, userTableRowFromDB(u))
	}

	return data
}

// userTableRowFromDB converts a db.UserTable to a pages.UserRow.
func userTableRowFromDB(u db.UserTable) pages.UserRow {
	alias := ""
	if u.UserAlias != nil {
		alias = *u.UserAlias
	}
	email := ""
	if u.UserEmail != nil {
		email = *u.UserEmail
	}

	status := userStatusFromMetadata(u.Metadata)

	row := pages.UserRow{
		UserID:    u.UserID,
		UserAlias: alias,
		UserEmail: email,
		UserRole:  u.UserRole,
		Teams:     u.Teams,
		Spend:     u.Spend,
		MaxBudget: u.MaxBudget,
		Models:    u.Models,
		Status:    status,
		TPMLimit:  u.TpmLimit,
		RPMLimit:  u.RpmLimit,
	}
	if u.CreatedAt.Valid {
		row.CreatedAt = u.CreatedAt.Time
	}
	return row
}

// userStatusFromMetadata extracts status from JSONB metadata.
// Returns "active" if metadata is nil/empty or has no status key.
func userStatusFromMetadata(metadata []byte) string {
	if len(metadata) == 0 {
		return "active"
	}
	var m map[string]any
	if err := json.Unmarshal(metadata, &m); err != nil {
		return "active"
	}
	s, ok := m["status"].(string)
	if !ok || s == "" {
		return "active"
	}
	return s
}

func userSocialAuthEnrollmentEnabled(metadata []byte) bool {
	if len(metadata) == 0 {
		return false
	}
	var m map[string]any
	if err := json.Unmarshal(metadata, &m); err != nil {
		return false
	}
	enabled, _ := m["social_auth_enrollment_enabled"].(bool)
	return enabled
}

func (h *UIHandler) handleUserCreate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	userEmail := strings.TrimSpace(r.FormValue("user_email"))
	if userEmail == "" {
		data := h.loadUsersPageData(r)
		render(r.Context(), w, pages.UsersTableWithToast(data, "Email is required", toast.VariantError))
		return
	}

	userRole := strings.TrimSpace(r.FormValue("user_role"))
	if userRole == "" {
		userRole = "internal_user"
	}
	if !isAssignableUIRole(userRole) {
		http.Error(w, "invalid user role", http.StatusBadRequest)
		return
	}

	// Check email uniqueness
	existing, err := h.DB.GetUserByEmail(r.Context(), userEmail)
	if err == nil && existing.UserID != "" {
		data := h.loadUsersPageData(r)
		render(r.Context(), w, pages.UsersTableWithToast(data, "A user with this email already exists", toast.VariantError))
		return
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "failed to verify email uniqueness", http.StatusInternalServerError)
		return
	}

	userAlias := strings.TrimSpace(r.FormValue("user_alias"))
	var userAliasPtr *string
	if userAlias != "" {
		userAliasPtr = &userAlias
	}

	maxBudget := parseOptionalFloat(r.FormValue("max_budget"))
	tpmLimit := parseOptionalInt64(r.FormValue("tpm_limit"))
	rpmLimit := parseOptionalInt64(r.FormValue("rpm_limit"))

	teams := []string{}
	models := []string{}

	userID := uuid.New().String()

	params := db.CreateUserParams{
		UserID:    userID,
		UserAlias: userAliasPtr,
		UserEmail: &userEmail,
		UserRole:  userRole,
		Teams:     teams,
		MaxBudget: maxBudget,
		Models:    models,
		TpmLimit:  tpmLimit,
		RpmLimit:  rpmLimit,
		CreatedBy: "admin",
	}

	_, err = h.DB.CreateUser(r.Context(), params)
	if err != nil {
		data := h.loadUsersPageData(r)
		render(r.Context(), w, pages.UsersTableWithToast(data, "Failed to create user: "+err.Error(), toast.VariantError))
		return
	}

	data := h.loadUsersPageData(r)
	render(r.Context(), w, pages.UsersTableWithToast(data, "User created successfully", toast.VariantSuccess))
}

func (h *UIHandler) handleUserBlock(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	userID := chi.URLParam(r, "user_id")
	if userID == "" {
		http.Error(w, "user_id required", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	err := h.withActiveAdminMutationLock(r.Context(), func(q *db.Queries) error {
		user, err := q.GetUser(r.Context(), userID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errUserNotFound
		}
		if err != nil {
			return err
		}
		if user.UserRole == "proxy_admin" && userStatusFromMetadata(user.Metadata) == "active" {
			count, err := q.CountUsersByRole(r.Context(), "proxy_admin")
			if err != nil {
				return err
			}
			if count <= 1 {
				return errLastActiveAdmin
			}
		}
		return q.SetUserStatus(r.Context(), db.SetUserStatusParams{
			UserID:    userID,
			Status:    "disabled",
			UpdatedBy: "admin",
		})
	})
	if errors.Is(err, errUserNotFound) {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	if errors.Is(err, errLastActiveAdmin) {
		if r.FormValue("return_to") == "detail" {
			http.Redirect(w, r, "/ui/users/"+userID, http.StatusSeeOther)
			return
		}
		data := h.loadUsersPageData(r)
		render(r.Context(), w, pages.UsersTableWithToast(data, "Cannot disable the last admin user", toast.VariantError))
		return
	}
	if err != nil {
		http.Error(w, "failed to verify admin safety", http.StatusInternalServerError)
		return
	}
	if r.FormValue("return_to") == "detail" {
		http.Redirect(w, r, "/ui/users/"+userID, http.StatusSeeOther)
		return
	}

	data := h.loadUsersPageData(r)
	render(r.Context(), w, pages.UsersTableWithToast(data, "User disabled successfully", toast.VariantSuccess))
}

func (h *UIHandler) handleUserUnblock(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	userID := chi.URLParam(r, "user_id")
	if userID == "" {
		http.Error(w, "user_id required", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if err := h.DB.SetUserStatus(r.Context(), db.SetUserStatusParams{
		UserID:    userID,
		Status:    "active",
		UpdatedBy: "admin",
	}); err != nil {
		data := h.loadUsersPageData(r)
		render(r.Context(), w, pages.UsersTableWithToast(data, "Failed to enable user: "+err.Error(), toast.VariantError))
		return
	}
	if r.FormValue("return_to") == "detail" {
		http.Redirect(w, r, "/ui/users/"+userID, http.StatusSeeOther)
		return
	}

	data := h.loadUsersPageData(r)
	render(r.Context(), w, pages.UsersTableWithToast(data, "User enabled successfully", toast.VariantSuccess))
}

func (h *UIHandler) handleUserDelete(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	userID := chi.URLParam(r, "user_id")
	if userID == "" {
		http.Error(w, "user_id required", http.StatusBadRequest)
		return
	}

	err := h.withActiveAdminMutationLock(r.Context(), func(q *db.Queries) error {
		user, err := q.GetUser(r.Context(), userID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errUserNotFound
		}
		if err != nil {
			return err
		}
		if user.UserRole == "proxy_admin" && userStatusFromMetadata(user.Metadata) == "active" {
			count, err := q.CountUsersByRole(r.Context(), "proxy_admin")
			if err != nil {
				return err
			}
			if count <= 1 {
				return errLastActiveAdmin
			}
		}
		return q.SoftDeleteUser(r.Context(), db.SoftDeleteUserParams{
			UserID:    userID,
			UpdatedBy: "admin",
		})
	})
	if errors.Is(err, errUserNotFound) {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	if errors.Is(err, errLastActiveAdmin) {
		data := h.loadUsersPageData(r)
		w.Header().Set("HX-Retarget", "#users-table")
		w.Header().Set("HX-Reswap", "innerHTML")
		render(r.Context(), w, pages.UsersTableWithToast(data, "Cannot delete the last admin user", toast.VariantError))
		return
	}
	if err != nil {
		http.Error(w, "failed to verify admin safety", http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Redirect", "/ui/users")
	w.WriteHeader(http.StatusOK)
}
