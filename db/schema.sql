CREATE EXTENSION IF NOT EXISTS pg_trgm;
-- Add new schema named "public"
CREATE SCHEMA IF NOT EXISTS "public";
-- Set comment to schema: "public"
COMMENT ON SCHEMA "public" IS 'standard public schema';
-- Create "users" table
CREATE TABLE "public"."users" ("id" uuid NOT NULL, "email" character varying(255) NOT NULL, "password" text NOT NULL, "settings" jsonb NOT NULL DEFAULT '{}', "created_at" timestamptz NOT NULL, "updated_at" timestamptz NOT NULL, "created_by" uuid NOT NULL, "updated_by" uuid NOT NULL, "deleted_at" timestamptz NULL, PRIMARY KEY ("id"));
-- Create index "idx_users_uk_02" to table: "users"
CREATE UNIQUE INDEX "idx_users_uk_02" ON "public"."users" ("email") WHERE (deleted_at IS NULL);
-- Create "refresh_tokens" table
CREATE TABLE "public"."refresh_tokens" ("id" uuid NOT NULL, "user_id" uuid NOT NULL, "token_hash" character varying(255) NOT NULL, "token_family" character varying(255) NULL, "expires_at" timestamptz NOT NULL, "revoked_at" timestamptz NULL, "created_at" timestamptz NOT NULL, "updated_at" timestamptz NOT NULL, "created_by" uuid NOT NULL, "updated_by" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "refresh_tokens_user_id_fkey" FOREIGN KEY ("user_id") REFERENCES "public"."users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "idx_refresh_tokens_pk_01" to table: "refresh_tokens"
CREATE INDEX "idx_refresh_tokens_pk_01" ON "public"."refresh_tokens" ("user_id") WHERE (revoked_at IS NULL);
-- Create index "idx_refresh_tokens_uk_01" to table: "refresh_tokens"
CREATE UNIQUE INDEX "idx_refresh_tokens_uk_01" ON "public"."refresh_tokens" ("token_hash");
-- Create "server_avatar_images" table
CREATE TABLE "public"."server_avatar_images" ("id" uuid NOT NULL, "bucket" character varying(50) NOT NULL, "object_key" character varying(255) NOT NULL, "mime_type" character varying(50) NOT NULL, "size" bigint NOT NULL, "created_at" timestamptz NOT NULL, "updated_at" timestamptz NOT NULL, "created_by" uuid NOT NULL, "updated_by" uuid NOT NULL, PRIMARY KEY ("id"));
-- Create "server_banner_images" table
CREATE TABLE "public"."server_banner_images" ("id" uuid NOT NULL, "bucket" character varying(50) NOT NULL, "object_key" character varying(255) NOT NULL, "mime_type" character varying(50) NOT NULL, "size" bigint NOT NULL, "created_at" timestamptz NOT NULL, "updated_at" timestamptz NOT NULL, "created_by" uuid NOT NULL, "updated_by" uuid NOT NULL, PRIMARY KEY ("id"));
-- Create "server_categories" table
CREATE TABLE "public"."server_categories" ("id" serial NOT NULL, "name" character varying(50) NOT NULL, "is_active" boolean NOT NULL DEFAULT true, "created_at" timestamptz NOT NULL, "updated_at" timestamptz NOT NULL, PRIMARY KEY ("id"));
-- Create index "idx_server_categories_uk_01" to table: "server_categories"
CREATE UNIQUE INDEX "idx_server_categories_uk_01" ON "public"."server_categories" ("name");
-- Create "servers" table
CREATE TABLE "public"."servers" ("id" uuid NOT NULL, "owner_id" uuid NOT NULL, "name" character varying(40) NOT NULL, "short_name" character varying(10) NOT NULL, "avatar_image_id" uuid NULL, "banner_image_id" uuid NULL, "category_id" integer NULL, "description" text NULL, "settings" jsonb NOT NULL DEFAULT '{}', "created_at" timestamptz NOT NULL, "updated_at" timestamptz NOT NULL, "created_by" uuid NOT NULL, "updated_by" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "servers_avatar_image_id_fkey" FOREIGN KEY ("avatar_image_id") REFERENCES "public"."server_avatar_images" ("id") ON UPDATE NO ACTION ON DELETE SET NULL, CONSTRAINT "servers_banner_image_id_fkey" FOREIGN KEY ("banner_image_id") REFERENCES "public"."server_banner_images" ("id") ON UPDATE NO ACTION ON DELETE SET NULL, CONSTRAINT "servers_category_id_fkey" FOREIGN KEY ("category_id") REFERENCES "public"."server_categories" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT "servers_owner_id_fkey" FOREIGN KEY ("owner_id") REFERENCES "public"."users" ("id") ON UPDATE NO ACTION ON DELETE RESTRICT);
-- Create index "idx_servers_pk_01" to table: "servers"
CREATE INDEX "idx_servers_pk_01" ON "public"."servers" ("owner_id");
-- Create index "idx_servers_pk_02" to table: "servers"
CREATE INDEX "idx_servers_pk_02" ON "public"."servers" ("category_id");
-- Create "server_invites" table
CREATE TABLE "public"."server_invites" ("id" uuid NOT NULL, "server_id" uuid NOT NULL, "code" character varying(8) NOT NULL, "max_uses" integer NOT NULL, "used_count" integer NOT NULL, "expires_at" timestamptz NULL, "is_active" boolean NOT NULL, "created_at" timestamptz NOT NULL, "updated_at" timestamptz NOT NULL, "created_by" uuid NOT NULL, "updated_by" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "server_invites_server_id_fkey" FOREIGN KEY ("server_id") REFERENCES "public"."servers" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "idx_server_invites_pk_01" to table: "server_invites"
CREATE INDEX "idx_server_invites_pk_01" ON "public"."server_invites" ("server_id") WHERE (is_active = true);
-- Create index "idx_server_invites_uk_01" to table: "server_invites"
CREATE UNIQUE INDEX "idx_server_invites_uk_01" ON "public"."server_invites" ("code");
-- Create "profile_avatar_images" table
CREATE TABLE "public"."profile_avatar_images" ("id" uuid NOT NULL, "bucket" character varying(50) NOT NULL, "object_key" character varying(255) NOT NULL, "mime_type" character varying(50) NOT NULL, "size" bigint NOT NULL, "created_at" timestamptz NOT NULL, "updated_at" timestamptz NOT NULL, "created_by" uuid NOT NULL, "updated_by" uuid NOT NULL, PRIMARY KEY ("id"));
-- Create "server_member_profiles" table
CREATE TABLE "public"."server_member_profiles" ("id" uuid NOT NULL, "server_id" uuid NOT NULL, "user_id" uuid NOT NULL, "nickname" character varying(50) NOT NULL, "bio" text NULL, "avatar_image_id" uuid NULL, "created_at" timestamptz NOT NULL, "updated_at" timestamptz NOT NULL, "created_by" uuid NOT NULL, "updated_by" uuid NOT NULL, "username" character varying(22) NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "server_member_profiles_avatar_image_id_fkey" FOREIGN KEY ("avatar_image_id") REFERENCES "public"."profile_avatar_images" ("id") ON UPDATE NO ACTION ON DELETE SET NULL, CONSTRAINT "server_member_profiles_server_id_fkey" FOREIGN KEY ("server_id") REFERENCES "public"."servers" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT "server_member_profiles_user_id_fkey" FOREIGN KEY ("user_id") REFERENCES "public"."users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "idx_server_member_profiles_uk_01" to table: "server_member_profiles"
CREATE UNIQUE INDEX "idx_server_member_profiles_uk_01" ON "public"."server_member_profiles" ("server_id", "user_id");
-- Create index "idx_server_member_profiles_uk_02" to table: "server_member_profiles"
CREATE UNIQUE INDEX "idx_server_member_profiles_uk_02" ON "public"."server_member_profiles" ("server_id", "nickname");
-- Create index "idx_server_member_profiles_uk_03" to table: "server_member_profiles"
CREATE UNIQUE INDEX "idx_server_member_profiles_uk_03" ON "public"."server_member_profiles" ("server_id", "username");
-- Create "server_roles" table
CREATE TABLE "public"."server_roles" ("id" uuid NOT NULL, "server_id" uuid NOT NULL, "name" character varying(30) NOT NULL, "permissions" jsonb NOT NULL DEFAULT '{}', "created_at" timestamptz NOT NULL, "updated_at" timestamptz NOT NULL, "created_by" uuid NOT NULL, "updated_by" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "server_roles_server_id_fkey" FOREIGN KEY ("server_id") REFERENCES "public"."servers" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "idx_server_roles_uk_01" to table: "server_roles"
CREATE UNIQUE INDEX "idx_server_roles_uk_01" ON "public"."server_roles" ("server_id", "name");
-- Create "server_members" table
CREATE TABLE "public"."server_members" ("id" uuid NOT NULL, "server_id" uuid NOT NULL, "user_id" uuid NOT NULL, "server_role_id" uuid NOT NULL, "joined_at" timestamptz NOT NULL, "created_at" timestamptz NOT NULL, "updated_at" timestamptz NOT NULL, "created_by" uuid NOT NULL, "updated_by" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "server_members_server_id_fkey" FOREIGN KEY ("server_id") REFERENCES "public"."servers" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT "server_members_server_role_id_fkey" FOREIGN KEY ("server_role_id") REFERENCES "public"."server_roles" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT "server_members_user_id_fkey" FOREIGN KEY ("user_id") REFERENCES "public"."users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "idx_server_members_uk_01" to table: "server_members"
CREATE UNIQUE INDEX "idx_server_members_uk_01" ON "public"."server_members" ("server_id", "user_id");
-- Create "server_post_images" table
CREATE TABLE "public"."server_post_images" ("id" uuid NOT NULL, "bucket" character varying(50) NOT NULL, "object_key" character varying(255) NOT NULL, "mime_type" character varying(50) NOT NULL, "size" bigint NOT NULL, "created_at" timestamptz NOT NULL, "updated_at" timestamptz NOT NULL, "created_by" uuid NOT NULL, "updated_by" uuid NOT NULL, PRIMARY KEY ("id"));
-- Create "server_posts" table
CREATE TABLE "public"."server_posts" ("id" uuid NOT NULL, "server_id" uuid NOT NULL, "author_id" uuid NOT NULL, "post_image_id" uuid NULL, "caption" text NOT NULL, "created_at" timestamptz NOT NULL, "updated_at" timestamptz NOT NULL, "created_by" uuid NOT NULL, "updated_by" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "server_posts_author_id_fkey" FOREIGN KEY ("author_id") REFERENCES "public"."users" ("id") ON UPDATE NO ACTION ON DELETE RESTRICT, CONSTRAINT "server_posts_post_image_id_fkey" FOREIGN KEY ("post_image_id") REFERENCES "public"."server_post_images" ("id") ON UPDATE NO ACTION ON DELETE SET NULL, CONSTRAINT "server_posts_server_id_fkey" FOREIGN KEY ("server_id") REFERENCES "public"."servers" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "idx_server_posts_pk_01" to table: "server_posts"
CREATE INDEX "idx_server_posts_pk_01" ON "public"."server_posts" ("server_id", "created_at" DESC);
-- Create index "idx_server_posts_pk_02" to table: "server_posts"
CREATE INDEX "idx_server_posts_pk_02" ON "public"."server_posts" ("author_id");
-- Trigram index for caption search (GET /servers/:serverId/posts/search).
-- gin_trgm_ops lets `caption ILIKE '%q%'` use the index instead of a seq scan.
CREATE INDEX idx_server_posts_caption_trgm ON server_posts USING gin (caption gin_trgm_ops);
-- Create "server_post_comments" table
CREATE TABLE "public"."server_post_comments" ("id" uuid NOT NULL, "post_id" uuid NOT NULL, "author_id" uuid NOT NULL, "parent_id" uuid NULL, "content" text NOT NULL, "created_at" timestamptz NOT NULL, "updated_at" timestamptz NOT NULL, "created_by" uuid NOT NULL, "updated_by" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "server_post_comments_author_id_fkey" FOREIGN KEY ("author_id") REFERENCES "public"."users" ("id") ON UPDATE NO ACTION ON DELETE RESTRICT, CONSTRAINT "server_post_comments_parent_id_fkey" FOREIGN KEY ("parent_id") REFERENCES "public"."server_post_comments" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT "server_post_comments_post_id_fkey" FOREIGN KEY ("post_id") REFERENCES "public"."server_posts" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "idx_server_post_comments_pk_01" to table: "server_post_comments"
CREATE INDEX "idx_server_post_comments_pk_01" ON "public"."server_post_comments" ("post_id", "created_at");
-- Create index "idx_server_post_comments_pk_02" to table: "server_post_comments"
CREATE INDEX "idx_server_post_comments_pk_02" ON "public"."server_post_comments" ("author_id");
-- Create "server_post_likes" table
CREATE TABLE "public"."server_post_likes" ("id" uuid NOT NULL, "post_id" uuid NOT NULL, "user_id" uuid NOT NULL, "created_at" timestamptz NOT NULL, "updated_at" timestamptz NOT NULL, "created_by" uuid NOT NULL, "updated_by" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "server_post_likes_post_id_fkey" FOREIGN KEY ("post_id") REFERENCES "public"."server_posts" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT "server_post_likes_user_id_fkey" FOREIGN KEY ("user_id") REFERENCES "public"."users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "idx_server_post_likes_uk_01" to table: "server_post_likes"
CREATE UNIQUE INDEX "idx_server_post_likes_uk_01" ON "public"."server_post_likes" ("post_id", "user_id");
-- Create "server_post_saves" table
CREATE TABLE "public"."server_post_saves" ("id" uuid NOT NULL, "post_id" uuid NOT NULL, "user_id" uuid NOT NULL, "created_at" timestamptz NOT NULL, "updated_at" timestamptz NOT NULL, "created_by" uuid NOT NULL, "updated_by" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "server_post_saves_post_id_fkey" FOREIGN KEY ("post_id") REFERENCES "public"."server_posts" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT "server_post_saves_user_id_fkey" FOREIGN KEY ("user_id") REFERENCES "public"."users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "idx_server_post_saves_uk_01" to table: "server_post_saves"
CREATE UNIQUE INDEX "idx_server_post_saves_uk_01" ON "public"."server_post_saves" ("post_id", "user_id");
-- Create index "idx_server_post_saves_pk_01" to table: "server_post_saves"
CREATE INDEX "idx_server_post_saves_pk_01" ON "public"."server_post_saves" ("user_id", "created_at" DESC);
-- Create "device_tokens" table
CREATE TABLE IF NOT EXISTS device_tokens (
    id           uuid PRIMARY KEY,
    user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token        text NOT NULL,
    platform     varchar(10) NOT NULL,
    created_at   timestamptz NOT NULL,
    updated_at   timestamptz NOT NULL,
    created_by   uuid NOT NULL,
    updated_by   uuid NOT NULL
);
CREATE UNIQUE INDEX idx_device_tokens_uk_01 ON device_tokens(token);
CREATE INDEX        idx_device_tokens_pk_01 ON device_tokens(user_id);
-- Create "notifications" table
CREATE TABLE IF NOT EXISTS notifications (
    id                uuid PRIMARY KEY,
    recipient_user_id uuid NOT NULL REFERENCES users(id)                  ON DELETE CASCADE,
    actor_user_id     uuid NOT NULL REFERENCES users(id)                  ON DELETE CASCADE,
    actor_profile_id  uuid NOT NULL REFERENCES server_member_profiles(id) ON DELETE CASCADE,
    type              varchar(20) NOT NULL,
    server_id         uuid NOT NULL REFERENCES servers(id)                ON DELETE CASCADE,
    post_id           uuid NULL REFERENCES server_posts(id)               ON DELETE CASCADE,
    comment_id        uuid NULL REFERENCES server_post_comments(id)       ON DELETE CASCADE,
    read_at           timestamptz NULL,
    created_at        timestamptz NOT NULL,
    updated_at        timestamptz NOT NULL,
    created_by        uuid NOT NULL,
    updated_by        uuid NOT NULL
);
CREATE INDEX idx_notifications_pk_01 ON notifications(recipient_user_id, server_id, created_at DESC);
CREATE INDEX idx_notifications_pk_02 ON notifications(recipient_user_id, server_id) WHERE read_at IS NULL;

-- ── DM Chat (server-scoped 1:1) ──
CREATE TABLE IF NOT EXISTS dm_conversations (
    id              uuid PRIMARY KEY,
    server_id       uuid NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    user_low        uuid NOT NULL REFERENCES users(id)   ON DELETE CASCADE,
    user_high       uuid NOT NULL REFERENCES users(id)   ON DELETE CASCADE,
    last_message_at timestamptz,
    created_at      timestamptz NOT NULL,
    updated_at      timestamptz NOT NULL,
    created_by      uuid NOT NULL,
    updated_by      uuid NOT NULL,
    CONSTRAINT chk_dm_conversations_user_order CHECK (user_low < user_high)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_dm_conversations_uk_01
    ON dm_conversations(server_id, user_low, user_high);

CREATE TABLE IF NOT EXISTS dm_conversation_states (
    conversation_id      uuid NOT NULL REFERENCES dm_conversations(id) ON DELETE CASCADE,
    user_id              uuid NOT NULL REFERENCES users(id)   ON DELETE CASCADE,
    server_id            uuid NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    peer_user_id         uuid NOT NULL,
    last_read_message_id uuid,
    last_read_at         timestamptz,
    last_message_at      timestamptz,
    last_message_preview text,
    unread_count         integer NOT NULL DEFAULT 0,
    created_at           timestamptz NOT NULL,
    updated_at           timestamptz NOT NULL,
    PRIMARY KEY (conversation_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_dm_conversation_states_pk_01
    ON dm_conversation_states(user_id, server_id, last_message_at DESC, conversation_id DESC);

CREATE TABLE IF NOT EXISTS dm_messages (
    id                uuid PRIMARY KEY,
    conversation_id   uuid NOT NULL REFERENCES dm_conversations(id) ON DELETE CASCADE,
    sender_id         uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type              text NOT NULL DEFAULT 'text',
    content           text NOT NULL,
    client_message_id uuid NOT NULL,
    created_at        timestamptz NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_dm_messages_uk_01
    ON dm_messages(conversation_id, sender_id, client_message_id);
CREATE INDEX IF NOT EXISTS idx_dm_messages_pk_01
    ON dm_messages(conversation_id, created_at DESC, id DESC);
