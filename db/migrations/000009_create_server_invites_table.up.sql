CREATE TABLE IF NOT EXISTS server_invites (
  id uuid PRIMARY KEY,
  server_id uuid NOT NULL,
  code varchar(8) NOT NULL,
  max_uses int NOT NULL,
  used_count int NOT NULL,
  expires_datetime timestamptz NULL,
  is_active boolean NOT NULL,
  create_user_id uuid NOT NULL,
  update_user_id uuid NOT NULL,
  create_datetime timestamptz NOT NULL,
  update_datetime timestamptz NOT NULL
);

CREATE UNIQUE INDEX idx_server_invites_uk_01 ON server_invites(code);
