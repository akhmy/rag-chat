-- name: CreateUser :one
INSERT INTO users (telegram_id)
VALUES (sqlc.narg(telegram_id))
RETURNING id, telegram_id, created_at;

-- name: GetUserByTelegramID :one
SELECT id, active_session_id, created_at
FROM users
WHERE telegram_id = $1;

-- name: SetActiveSession :execrows
UPDATE users u
SET active_session_id = s.id
FROM sessions s
WHERE u.id = s.user_id
  AND s.user_id = sqlc.arg(user_id)
  AND s.id = sqlc.arg(session_id);

-- name: GetActiveSession :one
SELECT s.id, s.title
FROM sessions s
JOIN users u ON u.active_session_id = s.id
WHERE u.id = $1
    AND u.id = s.user_id;