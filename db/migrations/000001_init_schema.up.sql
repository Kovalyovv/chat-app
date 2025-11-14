-- db/migrations/000001_init_schema.up.sql

CREATE TABLE "users" (
                         "id" SERIAL PRIMARY KEY,
                         "username" VARCHAR(255) UNIQUE NOT NULL,
                         "password_hash" VARCHAR(255) NOT NULL,
                         "created_at" TIMESTAMPTZ NOT NULL DEFAULT (now())
);

CREATE TABLE "rooms" (
                         "id" SERIAL PRIMARY KEY,
                         "name" VARCHAR(255) NOT NULL,
                         "invite_code" VARCHAR(16) UNIQUE NOT NULL,
                         "owner_id" INT NOT NULL,
                         "created_at" TIMESTAMPTZ NOT NULL DEFAULT (now())
);

CREATE TABLE "room_members" (
                                "user_id" INT NOT NULL,
                                "room_id" INT NOT NULL,
                                PRIMARY KEY ("user_id", "room_id")
);

CREATE TABLE "messages" (
                            "id" BIGSERIAL PRIMARY KEY,
                            "content" TEXT NOT NULL,
                            "room_id" INT NOT NULL,
                            "user_id" INT NOT NULL,
                            "created_at" TIMESTAMPTZ NOT NULL DEFAULT (now())
);

ALTER TABLE "rooms" ADD FOREIGN KEY ("owner_id") REFERENCES "users" ("id");
ALTER TABLE "room_members" ADD FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON DELETE CASCADE;
ALTER TABLE "room_members" ADD FOREIGN KEY ("room_id") REFERENCES "rooms" ("id") ON DELETE CASCADE;
ALTER TABLE "messages" ADD FOREIGN KEY ("room_id") REFERENCES "rooms" ("id") ON DELETE CASCADE;
ALTER TABLE "messages" ADD FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON DELETE SET NULL;

CREATE INDEX ON "messages" ("room_id", "created_at" DESC);
