-- name: CreateTurn :one
WITH new_turn AS (
INSERT INTO turns (session_id)
VALUES (sqlc.arg(session_id))
    RETURNING id
    )
INSERT INTO messages (turn_id, role, content)
SELECT
    nt.id,
    sqlc.arg(role),
    sqlc.arg(content)
FROM new_turn nt
    RETURNING turn_id;

-- name: CreateMessage :exec
INSERT INTO messages (turn_id, role, content)
VALUES ($1, $2, $3);

-- name: ListRecentMessages :many
WITH recent_turns AS (
    SELECT id
    FROM turns
    WHERE session_id = sqlc.arg(session_id)
    ORDER BY created_at DESC, id DESC
    LIMIT sqlc.arg(turn_limit)
    )
SELECT
    m.id,
    m.role,
    m.content,
    m.created_at
FROM messages m
         JOIN recent_turns rt ON rt.id = m.turn_id
ORDER BY m.created_at ASC, m.id ASC;