CREATE TABLE "users" (
  "id" UUID PRIMARY KEY DEFAULT (gen_random_uuid()),
  "wallet_address" text UNIQUE NOT NULL,
  "gamer_tag" text UNIQUE,
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE INDEX users_created_at_id_desc ON users (created_at DESC, id DESC);
