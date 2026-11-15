CREATE TYPE message_role AS ENUM ('user', 'assistant');

CREATE TABLE messages (
    id         BIGINT       GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    turn_id    BIGINT       NOT NULL REFERENCES turns (id),
    role       message_role NOT NULL,
    content    TEXT         NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);