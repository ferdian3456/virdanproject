ALTER TABLE server_member_profiles
    ADD COLUMN username varchar(22) NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_server_member_profiles_uk_03
    ON server_member_profiles(server_id, username);
