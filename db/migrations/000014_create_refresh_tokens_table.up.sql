-- Create refresh_tokens table for handling refresh token rotation
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL,
    token_family VARCHAR(255),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    created_by UUID NOT NULL,
    updated_by UUID NOT NULL
);

-- Unique indexes (uk = unique key)
CREATE UNIQUE INDEX idx_refresh_tokens_uk_01 ON refresh_tokens(token_hash);

-- Performance indexes (commented for now, uncomment when needed)
-- CREATE INDEX idx_refresh_tokens_01 ON refresh_tokens(user_id);
-- CREATE INDEX idx_refresh_tokens_02 ON refresh_tokens(token_family);
-- CREATE INDEX idx_refresh_tokens_03 ON refresh_tokens(user_id, revoked_at) WHERE revoked_at IS NULL AND deleted_at IS NULL;
