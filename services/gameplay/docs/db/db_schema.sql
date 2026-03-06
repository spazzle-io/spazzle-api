-- SQL dump generated using DBML (dbml-lang.org)
-- Database: PostgreSQL
-- Generated at: 2026-01-05T06:12:55.501Z

CREATE TABLE "servers" (
  "id" UUID PRIMARY KEY DEFAULT (gen_random_uuid()),
  "name" text NOT NULL,
  "owner_id" UUID NOT NULL,
  "num_admins" int NOT NULL DEFAULT 0,
  "num_custom_words" int NOT NULL DEFAULT 0,
  "is_publicly_visible" boolean NOT NULL DEFAULT true,
  "server_address" text NOT NULL,
  "stake_per_game" numeric NOT NULL,
  "num_rounds_per_game" int NOT NULL,
  "round_duration_secs" int NOT NULL,
  "num_drawing_options" int NOT NULL,
  "is_archived" boolean NOT NULL DEFAULT false,
  "archived_at" timestamptz,
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE TABLE "server_admins" (
  "server_id" UUID NOT NULL,
  "user_id" UUID NOT NULL,
  "added_at" timestamptz NOT NULL DEFAULT (now()),
  PRIMARY KEY ("server_id", "user_id")
);

CREATE TABLE "words" (
  "id" UUID PRIMARY KEY DEFAULT (gen_random_uuid()),
  "word_idx" int NOT NULL,
  "server_id" UUID NOT NULL,
  "word" text NOT NULL,
  "added_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE INDEX ON "servers" ("name");

CREATE INDEX ON "servers" ("owner_id");

CREATE INDEX ON "server_admins" ("user_id", "server_id");

CREATE INDEX ON "words" ("server_id");

CREATE UNIQUE INDEX ON "words" ("server_id", "word_idx");

CREATE UNIQUE INDEX ON "words" ("server_id", "word");

COMMENT ON COLUMN "servers"."name" IS 'a partial unique index on name and is_archived is added in a subsequent db migration';

COMMENT ON COLUMN "servers"."num_drawing_options" IS 'number of word options presented to a player before they select one to draw';

COMMENT ON COLUMN "servers"."created_at" IS 'an index on created_at DESC and id DESC is added in a subsequent db migration';

ALTER TABLE "server_admins" ADD FOREIGN KEY ("server_id") REFERENCES "servers" ("id") ON DELETE CASCADE ON UPDATE NO ACTION;

ALTER TABLE "words" ADD FOREIGN KEY ("server_id") REFERENCES "servers" ("id") ON DELETE CASCADE ON UPDATE NO ACTION;
