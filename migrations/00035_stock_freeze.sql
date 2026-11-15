-- +goose NO TRANSACTION
-- +goose Up
-- Stock freeze: a frozen product sells without touching its stock (the front shelf isn't counted);
-- counts, GRNs and conversions still work. stock_frozen_at = since when (the Frozen list).
ALTER TABLE products ADD COLUMN stock_frozen boolean NOT NULL DEFAULT false;
ALTER TABLE products ADD COLUMN stock_frozen_at timestamptz;
-- The shop is frozen: new products start frozen until it's unfrozen.
ALTER TABLE settings ADD COLUMN freeze_new_products boolean NOT NULL DEFAULT false;
-- "Opened / moved to the shelf": neither an adjustment nor a loss.
ALTER TYPE stock_movement_type ADD VALUE IF NOT EXISTS 'to_shelf';
-- A conversion whose pieces went out for sale into a frozen product (not counted).
ALTER TABLE conversion_runs ADD COLUMN out_for_sale boolean NOT NULL DEFAULT false;

-- +goose Down
-- The enum value stays (Postgres can't drop one).
ALTER TABLE conversion_runs DROP COLUMN out_for_sale;
ALTER TABLE settings DROP COLUMN freeze_new_products;
ALTER TABLE products DROP COLUMN stock_frozen_at;
ALTER TABLE products DROP COLUMN stock_frozen;
