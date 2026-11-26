-- name: CreateRefreshToken :exec
INSERT INTO refresh_tokens (id, user_id, family_id, token_hash, client, expires_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetRefreshTokenOwner :one
SELECT user_id FROM refresh_tokens WHERE token_hash = $1;

-- name: LockRefreshToken :one
SELECT *, (expires_at <= now())::boolean AS expired
FROM refresh_tokens WHERE token_hash = $1 FOR UPDATE;

-- name: MarkRefreshTokenUsed :exec
UPDATE refresh_tokens SET used_at = now() WHERE id = $1;

-- name: RevokeFamily :exec
UPDATE refresh_tokens SET revoked_at = now() WHERE family_id = $1 AND revoked_at IS NULL;

-- name: DeleteUserRefreshTokens :exec
DELETE FROM refresh_tokens WHERE user_id = $1;

-- name: CreateLoginCode :exec
INSERT INTO login_codes (code_hash, user_id, purpose, code_challenge, email, client, redirect, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, now() + make_interval(secs => sqlc.arg(ttl_seconds)::int));

-- name: PeekLoginCode :one
SELECT * FROM login_codes
WHERE code_hash = $1 AND purpose = $2 AND used_at IS NULL AND expires_at > now();

-- name: ConsumeLoginCode :one
UPDATE login_codes SET used_at = now()
WHERE code_hash = $1 AND purpose = $2 AND used_at IS NULL AND expires_at > now()
RETURNING *;

-- name: DeleteUserLoginCodes :exec
DELETE FROM login_codes WHERE user_id = $1 OR email = sqlc.narg(email)::citext;

-- name: ListSessions :many
-- One row per signed-in session that can still be refreshed.
SELECT family_id, client,
  min(created_at)::timestamptz AS created_at,
  max(created_at)::timestamptz AS last_used_at
FROM refresh_tokens
WHERE user_id = $1
GROUP BY family_id, client
HAVING bool_or(used_at IS NULL AND revoked_at IS NULL AND expires_at > now())
ORDER BY last_used_at DESC, family_id;

-- name: RevokeUserFamily :execrows
UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND family_id = $2 AND revoked_at IS NULL;

-- name: GetRefreshTokenFamily :one
SELECT family_id FROM refresh_tokens WHERE token_hash = $1;
