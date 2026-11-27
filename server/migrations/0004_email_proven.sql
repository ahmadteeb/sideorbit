-- +goose Up
-- When the account's holder last showed they control its mailbox (a magic link,
-- or a Google account that runs the mailbox). NULL: never shown.
ALTER TABLE users ADD COLUMN email_proven_at timestamptz;
UPDATE users SET email_proven_at = created_at
  WHERE id IN (SELECT user_id FROM identities WHERE provider = 'email');
