ALTER TABLE games
    ADD COLUMN started_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ADD COLUMN last_accessed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ADD COLUMN completed_at TIMESTAMPTZ,
    ADD COLUMN is_over BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX idx_games_user_status_access
    ON games (user_id, is_over, is_win, last_accessed_at DESC, id DESC);
