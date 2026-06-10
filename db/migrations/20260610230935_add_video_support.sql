-- Modify "server_post_images" table
ALTER TABLE "public"."server_post_images" ADD COLUMN "width" integer NULL, ADD COLUMN "height" integer NULL;
-- Create "server_post_videos" table
CREATE TABLE "public"."server_post_videos" ("id" uuid NOT NULL, "bucket" character varying(50) NOT NULL, "object_key" character varying(255) NOT NULL, "mime_type" character varying(50) NOT NULL, "size" bigint NOT NULL, "duration" integer NOT NULL, "width" integer NOT NULL, "height" integer NOT NULL, "thumbnail_object_key" character varying(255) NOT NULL, "created_at" timestamptz NOT NULL, "updated_at" timestamptz NOT NULL, "created_by" uuid NOT NULL, "updated_by" uuid NOT NULL, PRIMARY KEY ("id"));
-- Modify "server_posts" table
ALTER TABLE "public"."server_posts" ADD CONSTRAINT "chk_post_media_exclusive" CHECK (((post_image_id IS NOT NULL) AND (post_video_id IS NULL)) OR ((post_image_id IS NULL) AND (post_video_id IS NOT NULL))), ADD COLUMN "post_video_id" uuid NULL, ADD CONSTRAINT "server_posts_post_video_id_fkey" FOREIGN KEY ("post_video_id") REFERENCES "public"."server_post_videos" ("id") ON UPDATE NO ACTION ON DELETE SET NULL;
