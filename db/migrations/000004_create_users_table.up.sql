CREATE TABLE users (
    id          BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    telegram_id BIGINT      UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);