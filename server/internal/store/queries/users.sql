-- name: GetActiveUser :one
SELECT * FROM users WHERE id = $1 AND deleted_at IS NULL;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = sqlc.arg(email)::citext AND deleted_at IS NULL;

-- name: GetUserByIdentity :one
SELECT u.* FROM users u
JOIN identities i ON i.user_id = u.id
WHERE i.provider = $1 AND i.subject = $2 AND u.deleted_at IS NULL;

-- name: CreateUser :one
INSERT INTO users (id, email, display_name) VALUES ($1, $2, $3) RETURNING *;

-- name: CreateIdentity :exec
INSERT INTO identities (id, user_id, provider, subject) VALUES ($1, $2, $3, $4)
ON CONFLICT (provider, subject) DO NOTHING;

-- name: ListProviders :many
SELECT DISTINCT provider FROM identities WHERE user_id = $1 ORDER BY provider;

-- name: DeleteIdentities :exec
DELETE FROM identities WHERE user_id = $1;

-- name: DeleteUserDevices :exec
DELETE FROM devices WHERE user_id = $1;

-- name: DeleteUserSources :exec
DELETE FROM sources WHERE user_id = $1;

-- name: DeleteUserPushSubscriptions :exec
DELETE FROM push_subscriptions WHERE user_id = $1;

-- name: AnonymizeUser :exec
UPDATE users SET email = NULL, display_name = NULL, deleted_at = now() WHERE id = $1;
