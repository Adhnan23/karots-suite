-- +goose Up
-- A client that rings sales away from the server (offline) sends each with a
-- one-time ref, so a repeated send never books it twice, and the time it was rung.
ALTER TABLE sales ADD COLUMN client_ref text UNIQUE, ADD COLUMN rung_at timestamptz;
-- Things an admin should look at on a sale that was taken but not refused.
CREATE TABLE sale_flags (
  id bigserial PRIMARY KEY,
  sale_id bigint NOT NULL REFERENCES sales(id) ON DELETE CASCADE,
  kind text NOT NULL,
  note text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sale_flags_sale ON sale_flags (sale_id);

-- +goose Down
DROP TABLE sale_flags;
ALTER TABLE sales DROP COLUMN client_ref, DROP COLUMN rung_at;
