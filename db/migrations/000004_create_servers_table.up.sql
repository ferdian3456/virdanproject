CREATE TABLE IF NOT EXISTS servers(
    id uuid PRIMARY KEY,
    owner_id uuid NOT NULL,
    name varchar(40) NOT NULL,
    short_name varchar(10) NOT NULL,
    category_id int,
    avatar_image_id uuid,
    banner_image_id uuid,
    description text,
    settings JSONB NOT NULL DEFAULT '{}',
    -- Audit columns
    create_datetime timestamptz NOT NULL,
    update_datetime timestamptz NOT NULL,
    create_user_id uuid NOT NULL,
    update_user_id uuid NOT NULL,
    FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (category_id) REFERENCES server_categories(id) ON DELETE CASCADE
);