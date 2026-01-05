CREATE TABLE IF NOT EXISTS server_post_likes (
    post_id     uuid NOT NULL,
    user_id     uuid NOT NULL,
    create_user_id uuid NOT NULL,
    update_user_id uuid NOT NULL,
    create_datetime timestamptz NOT NULL,
    update_datetime timestamptz NOT NULL,
    FOREIGN KEY (post_id) REFERENCES server_posts(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
