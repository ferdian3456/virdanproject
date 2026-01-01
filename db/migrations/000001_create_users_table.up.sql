CREATE TABLE IF NOT EXISTS users(
    id uuid PRIMARY KEY,
    username  varchar(22) NOT NULL,
    fullname varchar(40) NOT NULL,
    bio varchar(150),
    avatar_image_id uuid,
    email  varchar(255) NOT NULL,
    password text NOT NULL,
    settings JSONB NOT NULL DEFAULT '{}',
    -- Audit columns
    create_datetime timestamptz NOT NULL,
    update_datetime timestamptz NOT NULL,
    create_user_id uuid NOT NULL,
    update_user_id uuid NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_uk_01 ON users(username);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_uk_02 ON users(email);