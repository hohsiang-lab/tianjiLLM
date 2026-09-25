CREATE TABLE IF NOT EXISTS ui_sessions (
    token TEXT PRIMARY KEY,
    data BYTEA NOT NULL,
    expiry TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_ui_sessions_expiry
    ON ui_sessions (expiry);
