-- name: InsertEvent :exec
-- A report resent after a lost response must not duplicate history.
INSERT INTO refresh_events (id, device_id, kind, trigger, result, error_code, duration_ms, occurred_at)
SELECT $1, $2, $3, $4, $5, $6, $7, $8
WHERE NOT EXISTS (
  SELECT 1 FROM refresh_events e
  WHERE e.device_id = $2 AND e.occurred_at = $8 AND e.kind = $3 AND e.result = $5
);

-- name: ListEvents :many
SELECT e.*, d.name AS device_name FROM refresh_events e
JOIN devices d ON d.id = e.device_id
WHERE d.user_id = sqlc.arg(user_id)
  AND (sqlc.narg(device_id)::uuid IS NULL OR e.device_id = sqlc.narg(device_id)::uuid)
  AND (sqlc.narg(result)::text IS NULL OR e.result = sqlc.narg(result)::text)
  AND (sqlc.narg(before_time)::timestamptz IS NULL
       OR (e.occurred_at, e.id) < (sqlc.narg(before_time)::timestamptz, sqlc.narg(before_id)::uuid))
ORDER BY e.occurred_at DESC, e.id DESC
LIMIT sqlc.arg(page_size);
