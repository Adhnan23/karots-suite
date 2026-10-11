-- +goose Up
-- The request row is its first ask; repeat asks of the same item live here.
ALTER TABLE requests
    ADD COLUMN reason    TEXT NOT NULL DEFAULT '',  -- why the shop won't stock it (status 'declined')
    ADD COLUMN called_at TIMESTAMPTZ;               -- the first ask's customer was told it arrived
CREATE TABLE request_asks (
    id             BIGSERIAL PRIMARY KEY,
    request_id     BIGINT NOT NULL REFERENCES requests(id) ON DELETE CASCADE,
    customer_name  TEXT NOT NULL DEFAULT '',
    customer_phone TEXT NOT NULL DEFAULT '',
    created_by     BIGINT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    called_at      TIMESTAMPTZ
);
CREATE INDEX idx_request_asks_request ON request_asks (request_id);

-- +goose Down
DROP TABLE request_asks;
ALTER TABLE requests DROP COLUMN called_at, DROP COLUMN reason;
