-- Create extension "pg_trgm"
CREATE EXTENSION IF NOT EXISTS "pg_trgm";
-- Create index "idx_server_posts_caption_trgm" to table: "server_posts"
CREATE INDEX "idx_server_posts_caption_trgm" ON "public"."server_posts" USING gin ("caption" gin_trgm_ops);
