-- Create "device_tokens" table
CREATE TABLE "public"."device_tokens" ("id" uuid NOT NULL, "user_id" uuid NOT NULL, "token" text NOT NULL, "platform" character varying(10) NOT NULL, "created_at" timestamptz NOT NULL, "updated_at" timestamptz NOT NULL, "created_by" uuid NOT NULL, "updated_by" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "device_tokens_user_id_fkey" FOREIGN KEY ("user_id") REFERENCES "public"."users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "idx_device_tokens_pk_01" to table: "device_tokens"
CREATE INDEX "idx_device_tokens_pk_01" ON "public"."device_tokens" ("user_id");
-- Create index "idx_device_tokens_uk_01" to table: "device_tokens"
CREATE UNIQUE INDEX "idx_device_tokens_uk_01" ON "public"."device_tokens" ("token");
