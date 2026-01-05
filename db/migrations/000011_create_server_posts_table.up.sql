CREATE TABLE IF NOT EXISTS server_posts (
    id              uuid PRIMARY KEY,
    server_id       uuid NOT NULL,
    author_id       uuid NOT NULL,
    post_image_id   uuid NOT NULL,
    caption         text NOT NULL,
    create_user_id uuid NOT NULL,
    update_user_id uuid NOT NULL,
    create_datetime timestamptz NOT NULL,
    update_datetime timestamptz NOT NULL,
    FOREIGN KEY (author_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE CASCADE,
    FOREIGN KEY (post_image_id) REFERENCES server_post_images(id) ON DELETE CASCADE
);
