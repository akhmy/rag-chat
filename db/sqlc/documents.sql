-- name: InsertDocument :one
INSERT INTO documents (title, user_id)
VALUES ($1, $2)
RETURNING id;

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

-- name: ListDocuments :many
SELECT id, title
FROM documents
WHERE user_id = $1
    AND ingested_at IS NOT NULL
ORDER BY created_at DESC, id DESC
OFFSET $2
LIMIT $3;

-- name: GetDocument :one
SELECT id, title
FROM documents
WHERE user_id = $1
    AND id = $2
    AND ingested_at IS NOT NULL;

-- name: DeleteDocument :execrows
DELETE FROM documents
WHERE id = $1 AND user_id = $2;