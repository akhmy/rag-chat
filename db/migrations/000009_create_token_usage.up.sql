CREATE TABLE token_usage (
    id      BIGINT      GENERATED ALWAYS AS IDENTITY,
    user_id BIGINT      NOT NULL REFERENCES users (id),
    tokens  BIGINT      NOT NULL,
    used_at TIMESTAMPTZ NOT NULL DEFAULT now()
);