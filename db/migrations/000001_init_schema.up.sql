
CREATE TABLE rooms
(
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    invite_code VARCHAR(16)  NOT NULL UNIQUE,
    owner_id    INTEGER      NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT NOW() NOT NULL
);

CREATE TABLE room_members
(
    user_id INTEGER NOT NULL,
    room_id INTEGER NOT NULL REFERENCES rooms ON DELETE CASCADE,
    PRIMARY KEY (user_id, room_id)
);

CREATE TABLE messages
(
    id           BIGSERIAL PRIMARY KEY,
    room_id      INTEGER   NOT NULL REFERENCES rooms ON DELETE CASCADE,
    user_id      INTEGER   NOT NULL,
    message_type VARCHAR(20) NOT NULL,
    text         TEXT,
    metadata     JSONB,
    created_at   TIMESTAMPTZ DEFAULT NOW() NOT NULL
);
CREATE INDEX messages_room_id_created_at_idx ON messages (room_id ASC, created_at DESC);

CREATE TABLE room_read_states
(
    room_id                   INTEGER   NOT NULL REFERENCES rooms ON DELETE CASCADE,
    user_id                   INTEGER   NOT NULL,
    last_read_message_id      BIGINT    DEFAULT 0 NOT NULL,
    last_delivered_message_id BIGINT    DEFAULT 0 NOT NULL,
    updated_at                TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    PRIMARY KEY (room_id, user_id)
);