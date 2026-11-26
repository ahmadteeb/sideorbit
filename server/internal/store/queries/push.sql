-- name: ListPushSubscriptions :many
SELECT * FROM push_subscriptions WHERE user_id = $1 ORDER BY created_at, id;

-- name: SavePushSubscription :one
-- The endpoint identifies the browser, so signing in as someone else moves it.
INSERT INTO push_subscriptions (id, user_id, endpoint, p256dh, auth, user_agent, session_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (endpoint) DO UPDATE SET user_id = EXCLUDED.user_id, p256dh = EXCLUDED.p256dh,
  auth = EXCLUDED.auth, user_agent = EXCLUDED.user_agent, session_id = EXCLUDED.session_id, failures = 0
RETURNING *;

-- name: TrimPushSubscriptions :exec
-- Keep each account's newest few.
DELETE FROM push_subscriptions WHERE id IN (
  SELECT p.id FROM push_subscriptions p WHERE p.user_id = $1 ORDER BY p.created_at DESC, p.id DESC OFFSET sqlc.arg(keep)::int
);

-- name: DeletePushSubscription :execrows
DELETE FROM push_subscriptions WHERE id = $1 AND user_id = $2;

-- name: DeletePushSubscriptionByID :exec
DELETE FROM push_subscriptions WHERE id = $1;

-- name: ResetPushFailures :exec
UPDATE push_subscriptions SET failures = 0 WHERE id = $1;

-- name: CountPushFailure :one
UPDATE push_subscriptions SET failures = failures + 1 WHERE id = $1 RETURNING failures;

-- name: DeleteSessionPushSubscriptions :exec
DELETE FROM push_subscriptions WHERE session_id = sqlc.arg(session_id)::uuid;
