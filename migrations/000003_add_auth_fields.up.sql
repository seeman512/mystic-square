-- Role was introduced by migration 000002; this migration adds credential fields.
ALTER TABLE users
    ADD COLUMN email TEXT,
    ADD COLUMN password_hash TEXT NOT NULL DEFAULT '';

-- Preserve pre-auth users as inaccessible legacy records while satisfying the
-- new non-null, unique email contract. They must complete account recovery to log in.
UPDATE users
SET email = 'legacy-user-' || id || '@invalid.local'
WHERE email IS NULL;

ALTER TABLE users
    ALTER COLUMN email SET NOT NULL,
    ALTER COLUMN password_hash DROP DEFAULT;

CREATE UNIQUE INDEX idx_users_email ON users (email);
