-- Email + password reset (issue #78). `email` is nullable -- only
-- required if the admin wants password-reset-via-email to work at all.
ALTER TABLE users ADD COLUMN email TEXT;

-- Single-use, time-limited password reset tokens. Only the SHA-256 hash
-- of the raw token is stored (see internal/db/password_reset.go) so a
-- database leak alone can't be used to reset a password -- the raw
-- token only ever exists in the emailed link and in memory server-side
-- long enough to hash it.
CREATE TABLE password_reset_tokens (
    id         INTEGER PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at DATETIME NOT NULL,
    used_at    DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
