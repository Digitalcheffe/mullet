-- Access/refresh tokens for OAuth2 data plugins (issue #24). One row per
-- plugin instance -- an instance either has a token or doesn't, there's
-- no history kept, so UPSERT on plugin_instance_id replaces it wholesale
-- on every authorize or refresh.
CREATE TABLE oauth_tokens (
    id                  INTEGER PRIMARY KEY,
    plugin_instance_id  INTEGER NOT NULL UNIQUE REFERENCES data_plugin_instances(id) ON DELETE CASCADE,
    access_token        TEXT NOT NULL,
    refresh_token       TEXT,
    expires_at          DATETIME NOT NULL,
    scopes              TEXT NOT NULL,
    updated_at          DATETIME DEFAULT CURRENT_TIMESTAMP
);
