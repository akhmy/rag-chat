CREATE TABLE documents (
    id          BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id     BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    title       TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    ingested_at TIMESTAMPTZ
);

CREATE INDEX documents_user_id_idx ON documents (user_id);
