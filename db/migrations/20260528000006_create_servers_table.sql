CREATE TABLE IF NOT EXISTS servers (
    id              uuid PRIMARY KEY,
    owner_id        uuid NOT NULL,
    name            varchar(40) NOT NULL,
    short_name      varchar(10) NOT NULL,
    avatar_image_id uuid,
    banner_image_id uuid,
    category_id     int,
    description     text,
    settings        jsonb NOT NULL DEFAULT '{}',
    created_at      timestamptz NOT NULL,
    updated_at      timestamptz NOT NULL,
    created_by      uuid NOT NULL,
    updated_by      uuid NOT NULL,
    FOREIGN KEY (owner_id)        REFERENCES users(id)                ON DELETE RESTRICT,
    FOREIGN KEY (avatar_image_id) REFERENCES server_avatar_images(id) ON DELETE SET NULL,
    FOREIGN KEY (banner_image_id) REFERENCES server_banner_images(id) ON DELETE SET NULL,
    FOREIGN KEY (category_id)     REFERENCES server_categories(id)
);

CREATE INDEX IF NOT EXISTS idx_servers_pk_01 ON servers(owner_id);
CREATE INDEX IF NOT EXISTS idx_servers_pk_02 ON servers(category_id);
