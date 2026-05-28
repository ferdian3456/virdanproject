CREATE TABLE IF NOT EXISTS refresh_tokens (
    id            uuid PRIMARY KEY,
    user_id       uuid NOT NULL,
    token_hash    varchar(255) NOT NULL,
    token_family  varchar(255),
    expires_at    timestamptz NOT NULL,
    revoked_at    timestamptz,
    created_at    timestamptz NOT NULL,
    updated_at    timestamptz NOT NULL,
    created_by    uuid NOT NULL,
    updated_by    uuid NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_refresh_tokens_uk_01 ON refresh_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_pk_01 ON refresh_tokens(user_id) WHERE revoked_at IS NULL;
