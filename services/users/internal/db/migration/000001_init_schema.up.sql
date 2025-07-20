-- SQL dump generated using DBML (dbml-lang.org)
-- Database: PostgreSQL
-- Generated at: 2025-07-20T00:01:17.347Z

CREATE TABLE "users" (
  "id" UUID PRIMARY KEY DEFAULT (gen_random_uuid()),
  "wallet_address" varchar UNIQUE NOT NULL,
  "gamer_tag" varchar UNIQUE,
  "created_at" timestamptz NOT NULL DEFAULT (now())
);
