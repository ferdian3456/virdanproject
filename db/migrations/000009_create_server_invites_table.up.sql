CREATE TABLE IF NOT EXISTS server_invites (
  id uuid PRIMARY KEY,
  server_id uuid NOT NULL,
  code varchar(8) NOT NULL,
  max_uses int NOT NULL,
  used_count int NOT NULL,
  expires_at timestamptz NULL,
  is_active boolean NOT NULL,
  created_by uuid NOT NULL,
  updated_by uuid NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL
);

CREATE UNIQUE INDEX idx_server_invites_uk_01 ON server_invites(code);
