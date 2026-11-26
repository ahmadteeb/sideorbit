-- name: ListSources :many
SELECT * FROM sources WHERE user_id = $1 ORDER BY created_at, id;

-- name: CountSources :one
SELECT count(*) FROM sources WHERE user_id = $1;

-- name: GetSourceByURL :one
SELECT * FROM sources WHERE user_id = $1 AND url = $2;

-- name: CreateSource :one
INSERT INTO sources (id, user_id, url, name) VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id, url) DO UPDATE SET name = EXCLUDED.name
RETURNING *;

-- name: DeleteSource :execrows
DELETE FROM sources WHERE id = $1 AND user_id = $2;
