-- Create "server_post_saves" table
CREATE TABLE "public"."server_post_saves" ("id" uuid NOT NULL, "post_id" uuid NOT NULL, "user_id" uuid NOT NULL, "created_at" timestamptz NOT NULL, "updated_at" timestamptz NOT NULL, "created_by" uuid NOT NULL, "updated_by" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "server_post_saves_post_id_fkey" FOREIGN KEY ("post_id") REFERENCES "public"."server_posts" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT "server_post_saves_user_id_fkey" FOREIGN KEY ("user_id") REFERENCES "public"."users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "idx_server_post_saves_pk_01" to table: "server_post_saves"
CREATE INDEX "idx_server_post_saves_pk_01" ON "public"."server_post_saves" ("user_id", "created_at" DESC);
-- Create index "idx_server_post_saves_uk_01" to table: "server_post_saves"
CREATE UNIQUE INDEX "idx_server_post_saves_uk_01" ON "public"."server_post_saves" ("post_id", "user_id");
