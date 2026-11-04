-- +goose Up
-- A unit cost from a big box needs more than cents: a box of 120 at 1,000 is 8.3333
-- a unit, which adds back up to 1,000 (8.33 only makes 999.60).
ALTER TABLE products ALTER COLUMN cost_price TYPE numeric(14,4);
ALTER TABLE purchase_items ALTER COLUMN cost_price TYPE numeric(14,4);
ALTER TABLE purchase_return_items ALTER COLUMN cost_price TYPE numeric(14,4);
ALTER TABLE sale_items ALTER COLUMN cost_price TYPE numeric(14,4);
ALTER TABLE stock_batches ALTER COLUMN cost_price TYPE numeric(14,4);

-- +goose Down
ALTER TABLE products ALTER COLUMN cost_price TYPE numeric(12,2);
ALTER TABLE purchase_items ALTER COLUMN cost_price TYPE numeric(12,2);
ALTER TABLE purchase_return_items ALTER COLUMN cost_price TYPE numeric(12,2);
ALTER TABLE sale_items ALTER COLUMN cost_price TYPE numeric(12,2);
ALTER TABLE stock_batches ALTER COLUMN cost_price TYPE numeric(12,2);
