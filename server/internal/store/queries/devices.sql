-- name: ListDevices :many
SELECT * FROM devices WHERE user_id = $1 ORDER BY created_at, id;

-- name: GetDevice :one
SELECT * FROM devices WHERE id = $1 AND user_id = $2;

-- name: LockDevice :one
SELECT * FROM devices WHERE id = $1 AND user_id = $2 FOR UPDATE;

-- name: GetDeviceByClientID :one
SELECT * FROM devices WHERE anisette_client_id = sqlc.arg(client_id)::text;

-- name: CountDevices :one
SELECT count(*) FROM devices WHERE user_id = $1;

-- name: CreateDevice :one
INSERT INTO devices (id, user_id, name, model, ios_version, anisette_client_id, last_seen_at)
VALUES ($1, $2, $3, $4, $5, $6, now()) RETURNING *;

-- name: TouchDevice :one
UPDATE devices SET model = COALESCE(sqlc.narg(model), model),
  ios_version = COALESCE(sqlc.narg(ios_version), ios_version), last_seen_at = now()
WHERE id = sqlc.arg(id) RETURNING *;

-- name: RenameDevice :one
UPDATE devices SET name = $3 WHERE id = $1 AND user_id = $2 RETURNING *;

-- name: DeleteDevice :execrows
DELETE FROM devices WHERE id = $1 AND user_id = $2;

-- name: MarkDeviceSeen :exec
UPDATE devices SET last_seen_at = now() WHERE id = $1;

-- name: ApplyDeviceStatus :exec
UPDATE devices SET
  last_report_at = sqlc.arg(reported_at),
  vpn_ok = COALESCE(sqlc.narg(vpn_ok), vpn_ok),
  pairing_ok = COALESCE(sqlc.narg(pairing_ok), pairing_ok),
  apple_team_id = COALESCE(sqlc.narg(apple_team_id), apple_team_id),
  apple_label = COALESCE(sqlc.narg(apple_label), apple_label),
  app_ids_used_7d = COALESCE(sqlc.narg(app_ids_used_7d), app_ids_used_7d)
WHERE id = sqlc.arg(id);

-- name: UpsertDeviceApp :exec
INSERT INTO device_apps (device_id, bundle_id, name, version, expires_at, source_url, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, now())
ON CONFLICT (device_id, bundle_id) DO UPDATE SET
  name = EXCLUDED.name, version = EXCLUDED.version, expires_at = EXCLUDED.expires_at,
  source_url = EXCLUDED.source_url, updated_at = now();

-- name: DeleteDeviceAppsExcept :exec
DELETE FROM device_apps WHERE device_id = $1 AND NOT (bundle_id = ANY(sqlc.arg(keep)::text[]));

-- name: ListDeviceApps :many
SELECT * FROM device_apps WHERE device_id = $1 ORDER BY expires_at NULLS LAST, name, bundle_id;
