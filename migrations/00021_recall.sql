-- +goose Up
-- A supplier recalled this lot: when, and whether the shop stopped selling it
-- (the till then warns, as it does for an expired lot).
ALTER TABLE stock_batches ADD COLUMN recalled_on date, ADD COLUMN recall_stops boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE stock_batches DROP COLUMN recalled_on, DROP COLUMN recall_stops;
