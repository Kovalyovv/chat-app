CREATE TABLE rooms (
                       id SERIAL PRIMARY KEY,
                       name VARCHAR(255) NOT NULL,
                       invite_code VARCHAR(16) UNIQUE NOT NULL,
                       owner_id INT NOT NULL,
                       created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE room_members (
                              user_id INT NOT NULL,
                              room_id INT NOT NULL,
                              PRIMARY KEY (user_id, room_id)
);

CREATE TABLE messages (
                          id SERIAL PRIMARY KEY,
                          room_id INT NOT NULL,
                          user_id INT NOT NULL,
                          text TEXT NOT NULL,
                          created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE room_read_states (
                                  room_id INT NOT NULL,
                                  user_id INT NOT NULL,
                                  last_read_message_id BIGINT NOT NULL DEFAULT 0,
                                  last_delivered_message_id BIGINT NOT NULL DEFAULT 0,
                                  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
                                  PRIMARY KEY (room_id, user_id),
                                  FOREIGN KEY (room_id) REFERENCES rooms(id) ON DELETE CASCADE
);

ALTER TABLE room_members ADD FOREIGN KEY (room_id) REFERENCES rooms (id) ON DELETE CASCADE;
ALTER TABLE messages ADD FOREIGN KEY (room_id) REFERENCES rooms (id) ON DELETE CASCADE;

CREATE INDEX ON messages (room_id, created_at DESC);
