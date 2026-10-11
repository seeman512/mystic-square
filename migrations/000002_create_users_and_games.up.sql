CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    full_name TEXT NOT NULL CHECK (btrim(full_name) <> ''),
    role TEXT NOT NULL CHECK (role IN ('admin', 'user')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_users_role ON users (role);

CREATE TABLE games (
    id BIGSERIAL PRIMARY KEY,
    level_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    init JSONB NOT NULL CHECK (jsonb_typeof(init) = 'array'),
    moves JSONB NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(moves) = 'array'),
    is_win BOOLEAN NOT NULL DEFAULT FALSE,
    CONSTRAINT fk_games_level
        FOREIGN KEY (level_id) REFERENCES levels (id) ON DELETE RESTRICT,
    CONSTRAINT fk_games_user
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT uq_games_user_level UNIQUE (user_id, level_id)
);

CREATE INDEX idx_games_level_id ON games (level_id);
CREATE INDEX idx_games_user_id ON games (user_id);
