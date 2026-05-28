CREATE TABLE IF NOT EXISTS server_members (
    id              uuid PRIMARY KEY,
    server_id       uuid NOT NULL,
    user_id         uuid NOT NULL,
    server_role_id  uuid NOT NULL,
    joined_at       timestamptz NOT NULL,
    created_at      timestamptz NOT NULL,
    updated_at      timestamptz NOT NULL,
    created_by      uuid NOT NULL,
    updated_by      uuid NOT NULL,
    FOREIGN KEY (server_id)      REFERENCES servers(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id)        REFERENCES users(id)   ON DELETE CASCADE,
    FOREIGN KEY (server_role_id) REFERENCES server_roles(id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_server_members_uk_01 ON server_members(server_id, user_id);
