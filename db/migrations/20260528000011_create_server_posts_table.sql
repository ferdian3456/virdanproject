CREATE TABLE IF NOT EXISTS server_posts (
    id              uuid PRIMARY KEY,
    server_id       uuid NOT NULL,
    author_id       uuid NOT NULL,
    post_image_id   uuid,
    caption         text NOT NULL,
    created_at      timestamptz NOT NULL,
    updated_at      timestamptz NOT NULL,
    created_by      uuid NOT NULL,
    updated_by      uuid NOT NULL,
    FOREIGN KEY (server_id)     REFERENCES servers(id)            ON DELETE CASCADE,
    FOREIGN KEY (author_id)     REFERENCES users(id)              ON DELETE RESTRICT,
    FOREIGN KEY (post_image_id) REFERENCES server_post_images(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_server_posts_pk_01 ON server_posts(server_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_server_posts_pk_02 ON server_posts(author_id);
