CREATE TABLE turns (
    id           BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    session_id   UUID        NOT NULL REFERENCES sessions (id) ON DELETE CASCADE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);