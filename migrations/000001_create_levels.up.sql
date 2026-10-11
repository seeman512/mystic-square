CREATE TABLE levels (
    id BIGSERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    theme TEXT NOT NULL,
    objective TEXT NOT NULL,
    description TEXT,
    difficulty TEXT NOT NULL,
    columns INTEGER NOT NULL DEFAULT 4 CHECK (columns BETWEEN 2 AND 100),
    rows INTEGER NOT NULL DEFAULT 4 CHECK (rows BETWEEN 2 AND 100),
    time_limit INTEGER CHECK (time_limit BETWEEN -1 AND 3600),
    move_limit INTEGER CHECK (move_limit BETWEEN -1 AND 10000),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_levels_difficulty ON levels (difficulty);
