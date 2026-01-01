CREATE TABLE IF NOT EXISTS server_member_profiles (
    id uuid PRIMARY KEY,
    server_member_id uuid NOT NULL,
    server_id uuid NOT NULL,
    user_id uuid NOT NULL,
    username varchar(20) NOT NULL,
    fullname varchar(40) NOT NULL,
    bio varchar(150),
    avatar_image_id uuid,
    create_datetime timestamptz NOT NULL,
    update_datetime timestamptz NOT NULL,
    create_user_id uuid NOT NULL,
    update_user_id uuid NOT NULL,
    FOREIGN KEY (server_member_id) REFERENCES server_members(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX idx_server_member_profiles_uk_01 ON server_member_profiles(server_id, username);
CREATE UNIQUE INDEX idx_server_member_profiles_uk_02 ON server_member_profiles(server_member_id);