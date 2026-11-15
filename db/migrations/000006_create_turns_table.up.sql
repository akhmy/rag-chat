CREATE TABLE turns (
    id           BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    session_id   BIGINT      NOT NULL REFERENCES sessions (id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);