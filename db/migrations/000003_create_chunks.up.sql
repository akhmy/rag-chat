CREATE TABLE chunks (
    id          BIGINT       GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    document_id BIGINT       NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    content     TEXT         NOT NULL,
    embedding   VECTOR(1536) NOT NULL
);