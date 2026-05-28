CREATE TABLE IF NOT EXISTS server_member_profiles (
    id              uuid PRIMARY KEY,
    server_id       uuid NOT NULL,
    user_id         uuid NOT NULL,
    nickname        varchar(50) NOT NULL,
    bio             text,
    avatar_image_id uuid,
    created_at      timestamptz NOT NULL,
    updated_at      timestamptz NOT NULL,
    created_by      uuid NOT NULL,
    updated_by      uuid NOT NULL,
    FOREIGN KEY (server_id)       REFERENCES servers(id)               ON DELETE CASCADE,
    FOREIGN KEY (user_id)         REFERENCES users(id)                 ON DELETE CASCADE,
    FOREIGN KEY (avatar_image_id) REFERENCES profile_avatar_images(id) ON DELETE SET NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_server_member_profiles_uk_01 ON server_member_profiles(server_id, user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_server_member_profiles_uk_02 ON server_member_profiles(server_id, nickname);
