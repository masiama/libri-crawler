-- Modify "books" table
ALTER TABLE "public"."books" ADD COLUMN "source_priority" bigint NOT NULL DEFAULT 100, ADD COLUMN "source_name" character varying NULL;
