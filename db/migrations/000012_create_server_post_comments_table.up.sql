CREATE TABLE IF NOT EXISTS server_post_comments (
    id              uuid PRIMARY KEY,
    post_id         uuid NOT NULL,
    author_id       uuid NOT NULL,
    parent_id       uuid NULL,
    content         text NOT NULL,
    created_by uuid NOT NULL,
    updated_by uuid NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    FOREIGN KEY (author_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (parent_id) REFERENCES server_post_comments(id) ON DELETE CASCADE,
    FOREIGN KEY (post_id) REFERENCES server_posts(id) ON DELETE CASCADE
);
