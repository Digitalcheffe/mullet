-- Two-factor authentication (issue #114). totp_secret is encrypted at
-- rest (see internal/db/totp_encryption.go), the same pattern as the
-- SMTP password. totp_enabled only ever flips to true once enrollment
-- has been confirmed with one valid code -- an unconfirmed secret sits
-- in totp_secret with totp_enabled still 0, and is overwritten by the
-- next enrollment attempt rather than left dangling.
ALTER TABLE users ADD COLUMN totp_secret TEXT;
ALTER TABLE users ADD COLUMN totp_enabled INTEGER NOT NULL DEFAULT 0;

CREATE TABLE totp_backup_codes (
    id         INTEGER PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash  TEXT NOT NULL,
    used_at    DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_totp_backup_codes_user_id ON totp_backup_codes(user_id);
