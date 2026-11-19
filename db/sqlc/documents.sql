-- name: InsertDocument :one
INSERT INTO documents (title, user_id)
VALUES ($1, $2)
RETURNING id;

-- name: DeleteDocument :exec
DELETE FROM documents
WHERE id = $1;

-- name: InsertChunks :exec
INSERT INTO chunks (document_id, content, embedding)
SELECT sqlc.arg(document_id),
    unnest(sqlc.arg(contents)::text[]),
    unnest(sqlc.arg(embeddings)::text[]::vector[]);

-- name: MarkDocumentAsIngested :exec
UPDATE documents
SET ingested_at = now()
WHERE id = $1;

-- name: ListNearestChunks :many
SELECT
    d.title,
    c.content,
    (c.embedding <=> sqlc.arg(query)::text::vector)::float8 AS distance
FROM chunks c
JOIN documents d ON d.id = c.document_id
WHERE d.user_id = sqlc.arg(user_id) AND
      d.ingested_at IS NOT NULL
ORDER BY distance
LIMIT sqlc.arg(chunk_limit);