package ui

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/ui/components/toast"
	"github.com/praxisllmlab/tianjiLLM/internal/ui/pages"
)

var (
	errIdentityNotFound          = errors.New("identity not found")
	errCannotUnlinkSelfLastLogin = errors.New("cannot unlink your last sign-in identity")
	errCannotUnlinkLastAdmin     = errors.New("cannot unlink the last active admin identity")
)

func (h *UIHandler) renderUserDetailToast(
	w http.ResponseWriter,
	r *http.Request,
	userID string,
	message string,
	variant toast.Variant,
) {
	data, ok := h.loadUserDetailData(r, userID)
	if !ok {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	render(r.Context(), w, pages.UserDetailHeaderWithToast(data, message, variant))
}

func (h *UIHandler) handleUserSocialAuthEnrollment(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	userID := chi.URLParam(r, "user_id")
	enabledValue := strings.TrimSpace(r.FormValue("enabled"))
	if userID == "" || (enabledValue != "true" && enabledValue != "false") {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	enabled := enabledValue == "true"

	user, err := h.DB.GetUser(r.Context(), userID)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "failed to load user", http.StatusInternalServerError)
		return
	}
	if enabled {
		if userStatusFromMetadata(user.Metadata) != "active" ||
			user.UserEmail == nil ||
			strings.TrimSpace(*user.UserEmail) == "" ||
			!isAssignableUIRole(user.UserRole) {
			h.renderUserDetailToast(
				w,
				r,
				userID,
				"Enrollment requires an active dashboard user with an email address",
				toast.VariantError,
			)
			return
		}
		if !h.socialAuthEnabled() {
			h.renderUserDetailToast(w, r, userID, "Social authentication is not configured", toast.VariantError)
			return
		}
	}

	if _, err := h.DB.SetUserSocialAuthEnrollment(r.Context(), db.SetUserSocialAuthEnrollmentParams{
		Enabled:   enabled,
		UpdatedBy: "admin",
		UserID:    userID,
	}); err != nil {
		http.Error(w, "failed to update enrollment", http.StatusInternalServerError)
		return
	}

	message := "Social sign-in enrollment closed"
	if enabled {
		message = "Next verified provider enrollment enabled"
	}
	h.renderUserDetailToast(w, r, userID, message, toast.VariantSuccess)
}

func (h *UIHandler) handleUserIdentityDelete(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	userID := chi.URLParam(r, "user_id")
	identityID := chi.URLParam(r, "identity_id")
	if userID == "" || identityID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	principal, _ := principalFromContext(r.Context())
	err := h.withActiveAdminMutationLock(r.Context(), func(q *db.Queries) error {
		user, err := q.GetUser(r.Context(), userID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errUserNotFound
		}
		if err != nil {
			return err
		}

		identity, err := q.GetUserIdentity(r.Context(), identityID)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && identity.UserID != userID) {
			return errIdentityNotFound
		}
		if err != nil {
			return err
		}

		identityCount, err := q.CountUserIdentitiesByUser(r.Context(), userID)
		if err != nil {
			return err
		}
		if identityCount <= 1 {
			if principal != nil && principal.UserID == userID {
				return errCannotUnlinkSelfLastLogin
			}
			if user.UserRole == "proxy_admin" && userStatusFromMetadata(user.Metadata) == "active" {
				adminCount, countErr := q.CountUsersByRole(r.Context(), "proxy_admin")
				if countErr != nil {
					return countErr
				}
				if adminCount <= 1 {
					return errCannotUnlinkLastAdmin
				}
			}
		}

		deleted, err := q.DeleteUserIdentityForUser(r.Context(), db.DeleteUserIdentityForUserParams{
			IdentityID: identityID,
			UserID:     userID,
		})
		if err != nil {
			return err
		}
		if deleted != 1 {
			return errIdentityNotFound
		}
		return q.BumpUserAuthVersion(r.Context(), db.BumpUserAuthVersionParams{
			UpdatedBy: "admin",
			UserID:    userID,
		})
	})

	switch {
	case errors.Is(err, errUserNotFound), errors.Is(err, errIdentityNotFound):
		http.Error(w, "identity not found", http.StatusNotFound)
	case errors.Is(err, errCannotUnlinkSelfLastLogin):
		h.renderUserDetailToast(w, r, userID, "You cannot unlink your last sign-in identity", toast.VariantError)
	case errors.Is(err, errCannotUnlinkLastAdmin):
		h.renderUserDetailToast(w, r, userID, "Cannot unlink the last active admin identity", toast.VariantError)
	case err != nil:
		http.Error(w, "failed to unlink identity", http.StatusInternalServerError)
	default:
		if principal != nil && principal.UserID == userID {
			w.Header().Set("HX-Redirect", "/ui/login")
			w.WriteHeader(http.StatusOK)
			return
		}
		h.renderUserDetailToast(w, r, userID, "Provider identity unlinked", toast.VariantSuccess)
	}
}
