DROP INDEX IF EXISTS idx_games_user_status_access;

ALTER TABLE games
    DROP COLUMN IF EXISTS is_over,
    DROP COLUMN IF EXISTS completed_at,
    DROP COLUMN IF EXISTS last_accessed_at,
    DROP COLUMN IF EXISTS started_at;
