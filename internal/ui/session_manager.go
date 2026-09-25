package ui

import (
	"net/http"
	"time"

	"github.com/alexedwards/scs/pgxstore"
	"github.com/alexedwards/scs/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	sessionCookieName = "tianji_session"
	sessionLifetime   = 8 * time.Hour
	sessionIdleTime   = 30 * time.Minute
)

// NewSessionManager creates the server-side session manager used by the admin
// UI. PostgreSQL is the production store; a nil pool intentionally leaves SCS's
// in-memory store in place for database-free development and focused tests.
//
// The returned cleanup function must be called when a PostgreSQL store is used.
func NewSessionManager(pool *pgxpool.Pool, secure bool) (*scs.SessionManager, func()) {
	manager := scs.New()
	manager.Lifetime = sessionLifetime
	manager.IdleTimeout = sessionIdleTime
	manager.Cookie.Name = sessionCookieName
	manager.Cookie.Path = "/"
	manager.Cookie.HttpOnly = true
	manager.Cookie.Persist = true
	manager.Cookie.SameSite = http.SameSiteLaxMode
	manager.Cookie.Secure = secure

	if pool == nil {
		return manager, func() {}
	}

	store := pgxstore.NewWithConfig(pool, pgxstore.Config{
		CleanUpInterval: 5 * time.Minute,
		TableName:       "ui_sessions",
	})
	manager.Store = store

	return manager, store.StopCleanup
}
