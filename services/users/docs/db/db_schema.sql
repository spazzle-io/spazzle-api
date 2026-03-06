-- SQL dump generated using DBML (dbml-lang.org)
-- Database: PostgreSQL
-- Generated at: 2026-01-05T06:14:18.868Z

CREATE TABLE "users" (
  "id" UUID PRIMARY KEY DEFAULT (gen_random_uuid()),
  "wallet_address" text UNIQUE NOT NULL,
  "gamer_tag" text UNIQUE,
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE INDEX ON "users" ("wallet_address");
