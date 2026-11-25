-- name: SearchChunks :many
WITH
    -- Semantic search
    vector_candidates AS (
        SELECT
            c.id,
            c.embedding <=> sqlc.arg(query_embedding)::vector AS distance
        FROM chunks c
        JOIN documents d ON d.id = c.document_id
        WHERE d.user_id = sqlc.arg(user_id)::bigint
            AND d.ingested_at IS NOT NULL
        ORDER BY distance, c.id
        LIMIT GREATEST(30, sqlc.arg(chunk_limit)::integer)
    ),
    -- Query tokenization
    search_query AS (
        SELECT plainto_tsquery(
            'russian'::regconfig,
            sqlc.arg(query_text)::text
        ) AS terms
    ),
    -- Full-text search
    text_candidates AS (
        SELECT
            c.id,
            ts_rank_cd(c.search_vector, sq.terms) AS relevance
        FROM chunks c
        JOIN documents d ON d.id = c.document_id
        CROSS JOIN search_query sq
        WHERE d.user_id = sqlc.arg(user_id)::bigint
            AND d.ingested_at IS NOT NULL
            AND c.search_vector @@ sq.terms
        ORDER BY relevance DESC, c.id
        LIMIT GREATEST(30, sqlc.arg(chunk_limit)::integer)
    ),
    -- Candidates union
    ranked AS (
        SELECT
            'vector' AS search_type,
            id,
            row_number() OVER (ORDER BY distance, id) AS rank
        FROM vector_candidates

        UNION ALL

        SELECT
            'text' AS search_type,
            id,
            row_number() OVER (ORDER BY relevance DESC, id) AS rank
        FROM text_candidates
    ),
    -- RRF
    fused AS (
        SELECT
            id,
            MAX(rank) FILTER (WHERE search_type = 'vector') AS vector_rank,
            MAX(rank) FILTER (WHERE search_type = 'text') AS text_rank,
            SUM(1.0::float8 / (60 + rank)) AS score
        FROM ranked
        GROUP BY id
    )
SELECT
    d.title,
    c.content,
    f.score::float8 AS rrf,
    COALESCE(f.vector_rank, 0)::integer AS vector_rank,
    COALESCE(f.text_rank, 0)::integer AS text_rank
FROM fused f
JOIN chunks c on c.id = f.id
JOIN documents d ON d.id = c.document_id
ORDER BY f.score DESC, c.id
LIMIT sqlc.arg(chunk_limit)::integer;
