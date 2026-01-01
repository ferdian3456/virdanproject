CREATE TABLE IF NOT EXISTS server_members(
    id uuid PRIMARY KEY,
    server_id uuid NOT NULL,
    user_id uuid NOT NULL,
    server_role_id uuid NOT NULL,
    status smallint,
    joined_at timestamptz NOT NULL,
    left_at timestamptz,
    -- Audit columns
    create_datetime timestamptz NOT NULL,
    update_datetime timestamptz NOT NULL,
    create_user_id uuid NOT NULL,
    update_user_id uuid NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE CASCADE,
    FOREIGN KEY (server_role_id) REFERENCES server_roles(id)
);

CREATE UNIQUE INDEX idx_server_members_uk_01 ON server_members(server_id, user_id);
