CREATE TABLE IF NOT EXISTS server_banner_images (
    id          uuid PRIMARY KEY,
    bucket      varchar(50) NOT NULL,
    object_key  varchar(255) NOT NULL,
    mime_type   varchar(50) NOT NULL,
    size        bigint NOT NULL,
    created_at  timestamptz NOT NULL,
    updated_at  timestamptz NOT NULL,
    created_by  uuid NOT NULL,
    updated_by  uuid NOT NULL
);
