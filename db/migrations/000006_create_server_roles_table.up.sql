CREATE TABLE IF NOT EXISTS server_roles (
   id UUID PRIMARY KEY,
   server_id UUID NOT NULL,
   name VARCHAR(30) NOT NULL,
   permissions JSONB NOT NULL DEFAULT '{}',
   create_datetime timestamptz NOT NULL,
   update_datetime timestamptz NOT NULL,
   create_user_id UUID NOT NULL,
   update_user_id UUID NOT NULL,
   FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_roles_uk_01 ON server_roles(server_id, name);


