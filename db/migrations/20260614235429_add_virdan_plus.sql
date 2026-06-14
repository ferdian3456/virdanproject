-- Create "xendit_webhook_events" table
CREATE TABLE "public"."xendit_webhook_events" ("id" uuid NOT NULL, "event_id" character varying(255) NOT NULL, "event_type" character varying(100) NOT NULL, "reference_id" character varying(255) NULL, "payload" jsonb NOT NULL, "status" character varying(20) NOT NULL, "received_at" timestamptz NOT NULL, "processed_at" timestamptz NULL, PRIMARY KEY ("id"));
-- Create index "idx_xendit_webhook_events_uk_01" to table: "xendit_webhook_events"
CREATE UNIQUE INDEX "idx_xendit_webhook_events_uk_01" ON "public"."xendit_webhook_events" ("event_id");
-- Create "server_plus_orders" table
CREATE TABLE "public"."server_plus_orders" ("id" uuid NOT NULL, "server_id" uuid NOT NULL, "user_id" uuid NOT NULL, "reference_id" character varying(255) NOT NULL, "xendit_session_id" character varying(255) NULL, "xendit_payment_id" character varying(255) NULL, "base_idr" bigint NOT NULL, "tax_idr" bigint NOT NULL, "total_idr" bigint NOT NULL, "status" character varying(20) NOT NULL, "paid_at" timestamptz NULL, "plus_expires_at" timestamptz NULL, "created_at" timestamptz NOT NULL, "updated_at" timestamptz NOT NULL, "created_by" uuid NOT NULL, "updated_by" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "server_plus_orders_server_id_fkey" FOREIGN KEY ("server_id") REFERENCES "public"."servers" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT "server_plus_orders_user_id_fkey" FOREIGN KEY ("user_id") REFERENCES "public"."users" ("id") ON UPDATE NO ACTION ON DELETE RESTRICT);
-- Create index "idx_server_plus_orders_pk_01" to table: "server_plus_orders"
CREATE INDEX "idx_server_plus_orders_pk_01" ON "public"."server_plus_orders" ("server_id", "plus_expires_at" DESC);
-- Create index "idx_server_plus_orders_pk_02" to table: "server_plus_orders"
CREATE INDEX "idx_server_plus_orders_pk_02" ON "public"."server_plus_orders" ("user_id", "created_at" DESC);
-- Create index "idx_server_plus_orders_uk_01" to table: "server_plus_orders"
CREATE UNIQUE INDEX "idx_server_plus_orders_uk_01" ON "public"."server_plus_orders" ("reference_id");
-- Create index "idx_server_plus_orders_uk_02" to table: "server_plus_orders"
CREATE UNIQUE INDEX "idx_server_plus_orders_uk_02" ON "public"."server_plus_orders" ("xendit_payment_id") WHERE (xendit_payment_id IS NOT NULL);
