-- name: AddTokenUsage :exec
INSERT INTO token_usage (user_id, tokens)
VALUES ($1, $2);

-- name: GetTokenUsage :one
SELECT COALESCE(SUM(tokens), 0)::BIGINT
FROM token_usage
WHERE user_id = $1 AND used_at > $2;