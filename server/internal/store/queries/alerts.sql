-- name: ExpiringApps :many
-- Apps expiring within a day, for accounts that can be notified.
SELECT d.id AS device_id, d.name AS device_name, d.user_id, a.bundle_id, a.expires_at::timestamptz AS expires_at
FROM device_apps a
JOIN devices d ON d.id = a.device_id
JOIN users u ON u.id = d.user_id
WHERE u.deleted_at IS NULL
  AND a.expires_at > sqlc.arg(now)::timestamptz
  AND a.expires_at <= sqlc.arg(now)::timestamptz + interval '24 hours'
  AND EXISTS (SELECT 1 FROM push_subscriptions p WHERE p.user_id = d.user_id)
ORDER BY d.id, a.expires_at, a.bundle_id;

-- name: SilentDevices :many
-- Devices with apps that haven't checked in for two days.
SELECT d.id AS device_id, d.name AS device_name, d.user_id, d.last_seen_at::timestamptz AS last_seen_at
FROM devices d
JOIN users u ON u.id = d.user_id
WHERE u.deleted_at IS NULL
  AND d.last_seen_at <= sqlc.arg(now)::timestamptz - interval '48 hours'
  AND EXISTS (SELECT 1 FROM device_apps a WHERE a.device_id = d.id)
  AND EXISTS (SELECT 1 FROM push_subscriptions p WHERE p.user_id = d.user_id)
ORDER BY d.id;

-- name: ClaimAlert :execrows
INSERT INTO alerts_sent (device_id, bundle_id, kind, window_start) VALUES ($1, $2, $3, $4)
ON CONFLICT DO NOTHING;

-- name: ReleaseAlert :exec
DELETE FROM alerts_sent WHERE device_id = $1 AND bundle_id = $2 AND kind = $3 AND window_start = $4;

-- name: PruneLoginCodes :execrows
DELETE FROM login_codes WHERE expires_at < sqlc.arg(now)::timestamptz - interval '1 day';

-- name: PruneRefreshTokens :execrows
DELETE FROM refresh_tokens WHERE expires_at < sqlc.arg(now)::timestamptz - interval '1 day';

-- name: PruneEvents :execrows
DELETE FROM refresh_events WHERE occurred_at < sqlc.arg(before)::timestamptz;

-- name: PruneAlerts :execrows
DELETE FROM alerts_sent WHERE sent_at < sqlc.arg(before)::timestamptz;
