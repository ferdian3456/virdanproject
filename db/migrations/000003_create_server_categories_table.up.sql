CREATE TABLE server_categories (
   id serial PRIMARY KEY,
   name varchar(50) NOT NULL,
   is_active boolean NOT NULL DEFAULT true,
   created_at timestamptz NOT NULL DEFAULT now(),
   updated_at timestamptz NOT NULL DEFAULT now(),
   created_by uuid, -- Nullable karena di-seed via migration
   updated_by uuid
);

CREATE UNIQUE INDEX idx_server_categories_uk_01
    ON server_categories(name);

INSERT INTO server_categories (id, name, created_at, updated_at)
VALUES
    (1, 'Education', now(), now()),
    (2, 'Music', now(), now()),
    (3, 'Gaming', now(), now()),
    (4, 'Technology', now(), now()),
    (5, 'Community', now(), now());
