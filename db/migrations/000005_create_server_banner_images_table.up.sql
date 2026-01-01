CREATE TABLE IF NOT EXISTS server_banner_images(
    id uuid PRIMARY KEY,
    server_id uuid NOT NULL,
    bucket varchar(50) NOT NULL,
    object_key varchar(255) NOT NULL,
    mime_type varchar(50) NOT NULL,
    size bigint NOT NULL,
    -- Audit Columns
    create_datetime timestamptz NOT NULL,
    update_datetime timestamptz NOT NULL,
    create_user_id uuid NOT NULL,
    update_user_id uuid NOT NULL,
    FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE CASCADE
);