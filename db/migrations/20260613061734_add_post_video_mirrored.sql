-- Modify "server_post_videos" table
ALTER TABLE "public"."server_post_videos" ADD COLUMN "mirrored" boolean NOT NULL DEFAULT false;
