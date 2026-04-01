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
  "total_games" int NOT NULL DEFAULT 0,
  "total_volume" numeric NOT NULL DEFAULT 0,
  "total_players" int NOT NULL DEFAULT 0,
  "trending_score" float8 NOT NULL DEFAULT 0,
  "is_archived" boolean NOT NULL DEFAULT false,
  "archived_at" timestamptz,
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE TABLE "games" (
  "id" UUID PRIMARY KEY,
  "server_id" UUID NOT NULL,
  "num_rounds" int NOT NULL DEFAULT 0,
  "num_players" int NOT NULL DEFAULT 0,
  "total_pot" numeric NOT NULL DEFAULT 0,
  "game_stake" numeric NOT NULL DEFAULT 0,
  "started_at" timestamptz NOT NULL,
  "ended_at" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE TABLE "game_players" (
  "game_id" UUID NOT NULL,
  "user_id" UUID NOT NULL,
  "score" int NOT NULL DEFAULT 0,
  "pnl" numeric NOT NULL DEFAULT 0,
  "position" int NOT NULL,
  "rounds_played" int NOT NULL DEFAULT 0,
  "provisional_payout" numeric NOT NULL DEFAULT 0,
  "total_stake_lost" numeric NOT NULL DEFAULT 0,
  "is_evicted" boolean NOT NULL DEFAULT false,
  PRIMARY KEY ("game_id", "user_id")
);

CREATE TABLE "user_stats" (
  "user_id" UUID PRIMARY KEY,
  "total_games" int NOT NULL DEFAULT 0,
  "total_score" int NOT NULL DEFAULT 0,
  "total_pnl" numeric NOT NULL DEFAULT 0,
  "total_volume" numeric NOT NULL DEFAULT 0,
  "updated_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE TABLE "server_player_stats" (
  "server_id" UUID NOT NULL,
  "user_id" UUID NOT NULL,
  "total_games" int NOT NULL DEFAULT 0,
  "total_score" int NOT NULL DEFAULT 0,
  "total_pnl" numeric NOT NULL DEFAULT 0,
  "total_volume" numeric NOT NULL DEFAULT 0,
  "updated_at" timestamptz NOT NULL DEFAULT (now()),
  PRIMARY KEY ("server_id", "user_id")
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

CREATE INDEX ON "servers" ("trending_score" DESC);

CREATE INDEX ON "servers" ("total_games" DESC);

CREATE UNIQUE INDEX servers_name_unique_unarchived_idx ON servers (name) WHERE is_archived = false;

CREATE INDEX servers_created_at_id_desc ON servers (created_at DESC, id DESC);

CREATE INDEX ON "games" ("server_id");

CREATE INDEX ON "games" ("server_id", "ended_at" DESC);

CREATE INDEX ON "game_players" ("user_id", "game_id");

CREATE INDEX ON "game_players" ("game_id", "position" ASC);

CREATE INDEX ON "user_stats" ("total_pnl" DESC, "total_score" DESC);

CREATE INDEX ON "server_player_stats" ("server_id", "total_pnl" DESC, "total_score" DESC);

CREATE INDEX ON "server_player_stats" ("user_id");

CREATE INDEX ON "server_admins" ("user_id", "server_id");

CREATE INDEX server_admins_server_added_at_user_desc ON server_admins (server_id, added_at DESC, user_id DESC);

CREATE INDEX ON "words" ("server_id");

CREATE UNIQUE INDEX ON "words" ("server_id", "word_idx");

CREATE UNIQUE INDEX ON "words" ("server_id", "word");

CREATE INDEX words_server_added_at_id_desc ON words (server_id, added_at DESC, id DESC);

COMMENT ON COLUMN "servers"."num_drawing_options" IS 'number of word options presented to a player before they select one to draw';

ALTER TABLE "games" ADD FOREIGN KEY ("server_id") REFERENCES "servers" ("id") ON DELETE CASCADE ON UPDATE NO ACTION;

ALTER TABLE "game_players" ADD FOREIGN KEY ("game_id") REFERENCES "games" ("id") ON DELETE CASCADE ON UPDATE NO ACTION;

ALTER TABLE "server_player_stats" ADD FOREIGN KEY ("server_id") REFERENCES "servers" ("id") ON DELETE CASCADE ON UPDATE NO ACTION;

ALTER TABLE "server_admins" ADD FOREIGN KEY ("server_id") REFERENCES "servers" ("id") ON DELETE CASCADE ON UPDATE NO ACTION;

ALTER TABLE "words" ADD FOREIGN KEY ("server_id") REFERENCES "servers" ("id") ON DELETE CASCADE ON UPDATE NO ACTION;
