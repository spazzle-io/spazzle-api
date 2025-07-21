-- SQL dump generated using DBML (dbml-lang.org)
-- Database: PostgreSQL
-- Generated at: 2025-07-21T01:45:21.809Z

CREATE TABLE "users" (
  "id" UUID PRIMARY KEY DEFAULT (gen_random_uuid()),
  "wallet_address" varchar UNIQUE NOT NULL,
  "gamer_tag" varchar UNIQUE,
  "ens_name" varchar UNIQUE,
  "ens_avatar_uri" varchar,
  "ens_image_url" varchar,
  "ens_last_resolved_at" timestamptz,
  "created_at" timestamptz NOT NULL DEFAULT (now())
);
