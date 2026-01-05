CREATE TABLE IF NOT EXISTS servers(
    id uuid PRIMARY KEY,
    owner_id uuid NOT NULL,
    name varchar(40) NOT NULL,
    short_name varchar(10) NOT NULL,
    avatar_image_id uuid,
    banner_image_id uuid,
    category_id int,
    description text,
    settings JSONB NOT NULL DEFAULT '{}',
    -- Audit columns
    create_datetime timestamptz NOT NULL,
    update_datetime timestamptz NOT NULL,
    create_user_id uuid NOT NULL,
    update_user_id uuid NOT NULL,
    FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (category_id) REFERENCES server_categories(id) ON DELETE CASCADE,
    FOREIGN KEY (avatar_image_id) REFERENCES server_avatar_images(id) ON DELETE CASCADE,
    FOREIGN KEY (banner_image_id) REFERENCES server_banner_images(id) ON DELETE CASCADE
);
