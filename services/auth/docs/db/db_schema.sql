-- SQL dump generated using DBML (dbml-lang.org)
-- Database: PostgreSQL
-- Generated at: 2025-06-28T21:46:35.041Z

CREATE TYPE "role" AS ENUM (
  'admin',
  'user'
);

CREATE TABLE "credentials" (
  "id" UUID PRIMARY KEY DEFAULT (gen_random_uuid()),
  "user_id" UUID UNIQUE NOT NULL,
  "wallet_address" varchar UNIQUE NOT NULL,
  "role" role NOT NULL DEFAULT 'user',
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE TABLE "sessions" (
  "id" UUID PRIMARY KEY,
  "user_id" UUID NOT NULL,
  "wallet_address" varchar NOT NULL,
  "refresh_token" varchar UNIQUE NOT NULL,
  "user_agent" varchar NOT NULL,
  "client_ip" varchar NOT NULL,
  "is_revoked" boolean NOT NULL DEFAULT false,
  "expires_at" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE INDEX ON "credentials" ("user_id");

CREATE INDEX ON "credentials" ("wallet_address");

CREATE INDEX ON "sessions" ("user_id");

CREATE INDEX ON "sessions" ("wallet_address");

CREATE INDEX ON "sessions" ("refresh_token");

CREATE INDEX ON "sessions" ("user_id", "is_revoked");

ALTER TABLE "sessions" ADD FOREIGN KEY ("user_id") REFERENCES "credentials" ("user_id");

ALTER TABLE "sessions" ADD FOREIGN KEY ("wallet_address") REFERENCES "credentials" ("wallet_address");
