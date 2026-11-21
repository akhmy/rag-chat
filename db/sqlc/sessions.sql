-- name: CreateSession :one
WITH new_session AS (
    INSERT INTO sessions (id, user_id, title)
    VALUES (sqlc.arg(id), sqlc.arg(user_id), sqlc.narg(title))
    RETURNING id, user_id
)
UPDATE users AS u
SET active_session_id = ns.id
FROM new_session AS ns
WHERE u.id = ns.user_id
RETURNING u.active_session_id;

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
WHERE id = $1 AND user_id = $2;

-- name: SetSessionTitle :exec
UPDATE sessions
SET title = $1
WHERE id = $2 AND user_id = $3;

-- name: DeleteSession :execrows
DELETE FROM sessions
WHERE id = $1 AND user_id = $2;

-- name: RenameSession :execrows
UPDATE sessions
SET title = $3
WHERE id = $1 AND user_id = $2;