# Data model (PostgreSQL 16)

Principle: **store the minimum.** There are no Apple credentials, certificates, pairing files or IPAs. The Apple account is a masked label and a team ID only.

```sql
-- 0001_init.sql (goose)
CREATE TABLE users (
  id            uuid PRIMARY KEY,               -- UUIDv7 generated in Go
  email         citext UNIQUE,                  -- from provider or magic link (verified only)
  display_name  text,
  created_at    timestamptz NOT NULL DEFAULT now(),
  deleted_at    timestamptz
);

CREATE TABLE identities (                       -- linked logins
  id            uuid PRIMARY KEY,
  user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  provider      text NOT NULL CHECK (provider IN ('github','google','email')),
  subject       text NOT NULL,                  -- provider user id / email
  created_at    timestamptz NOT NULL DEFAULT now(),
  UNIQUE (provider, subject)
);

CREATE TABLE refresh_tokens (
  id            uuid PRIMARY KEY,
  user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  family_id     uuid NOT NULL,                  -- rotation family, revoke all on reuse
  token_hash    bytea NOT NULL UNIQUE,          -- sha256
  client        text NOT NULL,                  -- web | ios | desktop
  expires_at    timestamptz NOT NULL,
  used_at       timestamptz,
  revoked_at    timestamptz,
  created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE login_codes (                      -- one-time codes (oauth→app exchange, magic links)
  code_hash     bytea PRIMARY KEY,
  user_id       uuid REFERENCES users(id) ON DELETE CASCADE,
  purpose       text NOT NULL CHECK (purpose IN ('app_exchange','magic_link')),
  code_challenge text,                          -- PKCE S256 for app_exchange
  email         citext,                         -- for magic_link before user exists
  client        text CHECK (client IN ('web','ios','desktop')),
  redirect      text,                           -- allowlisted native redirect
  expires_at    timestamptz NOT NULL,
  used_at       timestamptz
);

CREATE TABLE devices (
  id            uuid PRIMARY KEY,
  user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name          text NOT NULL,
  model         text,
  ios_version   text,
  anisette_client_id text UNIQUE,               -- opaque v3 client identifier
  apple_team_id text,
  apple_label   text,                           -- masked, e.g. j***@icloud.com
  app_ids_used_7d smallint,
  vpn_ok        boolean,
  pairing_ok    boolean,
  last_seen_at  timestamptz,
  created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON devices (user_id);

CREATE TABLE device_apps (                      -- latest inventory (upserted on report)
  device_id     uuid NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
  bundle_id     text NOT NULL,
  name          text NOT NULL,
  version       text,
  expires_at    timestamptz,
  source_url    text,
  updated_at    timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (device_id, bundle_id)
);
CREATE INDEX ON device_apps (expires_at);

CREATE TABLE refresh_events (
  id            uuid PRIMARY KEY,
  device_id     uuid NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
  kind          text NOT NULL CHECK (kind IN ('refresh','install','remove','repair')),
  trigger       text CHECK (trigger IN ('background','shortcut','manual','launch','setup')),
  result        text NOT NULL CHECK (result IN ('success','failure')),
  error_code    text,
  duration_ms   integer,
  occurred_at   timestamptz NOT NULL
);
CREATE INDEX ON refresh_events (device_id, occurred_at DESC);

CREATE TABLE sources (
  id            uuid PRIMARY KEY,
  user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  url           text NOT NULL,
  name          text,
  created_at    timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, url)
);

CREATE TABLE push_subscriptions (
  id            uuid PRIMARY KEY,
  user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  endpoint      text NOT NULL UNIQUE,
  p256dh        text NOT NULL,
  auth          text NOT NULL,
  user_agent    text,
  failures      smallint NOT NULL DEFAULT 0,
  created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE alerts_sent (                      -- dedupe for scheduler
  device_id     uuid NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
  bundle_id     text NOT NULL DEFAULT '',
  kind          text NOT NULL,                  -- expiry_24h | expiry_6h | device_silent
  window_start  timestamptz NOT NULL,           -- expires_at - 7d, or last_seen_at
  sent_at       timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (device_id, bundle_id, kind, window_start)
);
```

Later migrations: `0002` adds `devices.last_report_at timestamptz` (the device's clock at its newest report).

Retention: `refresh_events` older than 90 days is pruned nightly. `login_codes` and `refresh_tokens` are pruned a day after they expire; `alerts_sent` follows the event retention. Pruning runs with the scheduler, every 15 minutes.
