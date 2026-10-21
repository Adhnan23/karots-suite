-- +goose Up
-- Smart reorder levels. reorder_level stays the level every low-stock reader
-- uses; on an Auto product the nightly run (internal/features/reorder) writes it,
-- learning from reorder_start toward what sales and deliveries show.
-- reorder_calc is that run's last result, for both modes (the reason lines).
-- Existing products stay Fixed; every product created from now on starts Auto.
ALTER TABLE products ADD COLUMN reorder_auto BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE products ALTER COLUMN reorder_auto SET DEFAULT true;
ALTER TABLE products ADD COLUMN reorder_start INT NOT NULL DEFAULT 0;
ALTER TABLE products ADD COLUMN reorder_calc JSONB;

ALTER TABLE settings ADD COLUMN reorder_default_days INT NOT NULL DEFAULT 7;
ALTER TABLE settings ADD COLUMN reorder_safety_days INT NOT NULL DEFAULT 3;
ALTER TABLE settings ADD COLUMN reorder_ran_at TIMESTAMPTZ;

-- A cashier with this sees the till's Low stock tab (admins and managers always).
ALTER TABLE users ADD COLUMN can_see_low_stock BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE users DROP COLUMN can_see_low_stock;
ALTER TABLE settings DROP COLUMN reorder_ran_at, DROP COLUMN reorder_safety_days, DROP COLUMN reorder_default_days;
ALTER TABLE products DROP COLUMN reorder_calc, DROP COLUMN reorder_start, DROP COLUMN reorder_auto;
