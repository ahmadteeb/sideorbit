-- +goose Up
-- The sign-in session that turned alerts on, so signing it out also stops them.
ALTER TABLE push_subscriptions ADD COLUMN session_id uuid;
CREATE INDEX ON push_subscriptions (session_id);
