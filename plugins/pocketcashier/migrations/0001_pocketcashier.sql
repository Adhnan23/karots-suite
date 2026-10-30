-- +goose Up
-- The shop's own certificate authority (one row): phones trust it once, and the
-- server issues its HTTPS certificate from it for whatever LAN IP it has.
CREATE TABLE pocket_ca (
  id int PRIMARY KEY CHECK (id = 1),
  cert_pem text NOT NULL,
  key_pem text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
-- A paired phone: its own key (only the hash is kept), who is on it, how fresh
-- its copy is.
CREATE TABLE pocket_phones (
  id bigserial PRIMARY KEY,
  name text NOT NULL,
  device text NOT NULL UNIQUE,
  token_hash bytea NOT NULL UNIQUE,
  status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','active','blocked')),
  pair_code text,
  user_id bigint REFERENCES users(id) ON DELETE SET NULL,
  last_seen timestamptz,
  copy_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  approved_at timestamptz,
  approved_by bigint REFERENCES users(id) ON DELETE SET NULL
);
-- One batch of actions a phone sent, and how it went.
CREATE TABLE pocket_sends (
  id bigserial PRIMARY KEY,
  phone_id bigint NOT NULL REFERENCES pocket_phones(id) ON DELETE CASCADE,
  received_at timestamptz NOT NULL DEFAULT now(),
  actions int NOT NULL DEFAULT 0,
  failed int NOT NULL DEFAULT 0,
  flagged int NOT NULL DEFAULT 0
);
-- Every action by its one-time ref: a repeat gets the stored result back.
CREATE TABLE pocket_actions (
  ref text PRIMARY KEY,
  phone_id bigint NOT NULL REFERENCES pocket_phones(id) ON DELETE CASCADE,
  send_id bigint NOT NULL REFERENCES pocket_sends(id) ON DELETE CASCADE,
  kind text NOT NULL,
  user_id bigint,
  at timestamptz NOT NULL,
  body jsonb NOT NULL,
  status text NOT NULL CHECK (status IN ('applied','skipped','failed')),
  sale_id bigint REFERENCES sales(id) ON DELETE SET NULL,
  held_id bigint,
  note text NOT NULL DEFAULT '',
  applied_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX pocket_actions_send ON pocket_actions (send_id);

-- +goose Down
DROP TABLE pocket_actions;
DROP TABLE pocket_sends;
DROP TABLE pocket_phones;
DROP TABLE pocket_ca;
