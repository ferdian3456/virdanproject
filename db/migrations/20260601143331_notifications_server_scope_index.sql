-- Drop index "idx_notifications_pk_01" from table: "notifications"
DROP INDEX "public"."idx_notifications_pk_01";
-- Drop index "idx_notifications_pk_02" from table: "notifications"
DROP INDEX "public"."idx_notifications_pk_02";
-- Create index "idx_notifications_pk_01" to table: "notifications"
CREATE INDEX "idx_notifications_pk_01" ON "public"."notifications" ("recipient_user_id", "server_id", "created_at" DESC);
-- Create index "idx_notifications_pk_02" to table: "notifications"
CREATE INDEX "idx_notifications_pk_02" ON "public"."notifications" ("recipient_user_id", "server_id") WHERE (read_at IS NULL);
