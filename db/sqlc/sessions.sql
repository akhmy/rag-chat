-- name: CreateUser :one
INSERT INTO users (telegram_id)
VALUES (sqlc.narg(telegram_id))
    RETURNING id, telegram_id, created_at;

-- name: GetUserByTelegramID :one
SELECT id, active_session_id, created_at
FROM users
WHERE telegram_id = $1;

-- name: CreateSession :one
WITH new_session AS (
    INSERT INTO sessions (user_id, title)
    VALUES (sqlc.arg(user_id), sqlc.narg(title))
    RETURNING id, user_id
)
UPDATE users AS u
SET active_session_id = ns.id
FROM new_session AS ns
WHERE u.id = ns.user_id
RETURNING u.active_session_id;

-- name: SetSessionTitle :exec
UPDATE sessions
SET title = $2
WHERE id = $1;

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

-- name: ListSessions :many
SELECT id, title, created_at
FROM sessions
WHERE user_id = $1
ORDER BY created_at DESC, id DESC
OFFSET $2
LIMIT $3;

-- name: GetSession :one
SELECT title, user_id, created_at
FROM sessions
WHERE id = $1;

-- name: SetActiveSession :exec
UPDATE users
SET active_session_id = $2
WHERE id = $1;

-- name: GetActiveSession :one
SELECT s.id, s.title
FROM sessions s
JOIN users u ON u.active_session_id = s.id
WHERE u.id = $1;
