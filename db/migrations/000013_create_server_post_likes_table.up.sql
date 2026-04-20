CREATE TABLE IF NOT EXISTS server_post_likes (
    id          uuid PRIMARY KEY,
    post_id     uuid NOT NULL,
    user_id     uuid NOT NULL,
    created_by uuid NOT NULL,
    updated_by uuid NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    FOREIGN KEY (post_id) REFERENCES server_posts(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
