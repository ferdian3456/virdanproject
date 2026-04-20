CREATE TABLE IF NOT EXISTS server_member_profiles(
    id uuid PRIMARY KEY,
    server_id uuid NOT NULL,
    user_id uuid NOT NULL,
    nickname varchar(255) NOT NULL,
    bio text,
    avatar_image_id uuid,
    -- Audit columns
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    created_by uuid NOT NULL,
    updated_by uuid NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE CASCADE,
    FOREIGN KEY (avatar_image_id) REFERENCES server_member_profile_avatar_images(id)
);

CREATE UNIQUE INDEX idx_server_member_profiles_uk_01 ON server_member_profiles(server_id, user_id);
