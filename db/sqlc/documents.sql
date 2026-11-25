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