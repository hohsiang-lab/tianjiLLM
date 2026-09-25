package ui

import "github.com/alexedwards/scs/v2"

const (
	sessionAuthenticatedKey = "authenticated"
	sessionRoleKey          = "role"
	sessionUserIDKey        = "user_id"
	sessionAuthVersionKey   = "auth_version"
)

func (h *UIHandler) getSessionManager() *scs.SessionManager {
	h.sessionManagerOnce.Do(func() {
		if h.SessionManager == nil {
			h.SessionManager, _ = NewSessionManager(nil, false)
		}
	})
	return h.SessionManager
}
