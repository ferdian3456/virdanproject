CREATE TABLE server_categories (
   id serial PRIMARY KEY,
   name varchar(50) NOT NULL,
   is_active boolean NOT NULL DEFAULT true,
   create_datetime timestamptz NOT NULL,
   update_datetime timestamptz NOT NULL
);

CREATE UNIQUE INDEX idx_server_categories_uk_01
    ON server_categories(name);

INSERT INTO server_categories (id, name, create_datetime, update_datetime)
VALUES
    (1, 'Education', now(), now()),
    (2, 'Music', now(), now()),
    (3, 'Gaming', now(), now()),
    (4, 'Technology', now(), now()),
    (5, 'Community', now(), now());
