CREATE TABLE IF NOT EXISTS users(
    id uuid PRIMARY KEY,
    username  varchar(22) NOT NULL,
    email  varchar(255) NOT NULL,
    password text NOT NULL,
    settings JSONB NOT NULL DEFAULT '{}',
    -- Audit columns
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    created_by uuid NOT NULL,
    updated_by uuid NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_uk_01 ON users(username);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_uk_02 ON users(email);