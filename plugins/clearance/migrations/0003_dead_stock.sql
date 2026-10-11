-- +goose Up
-- Dead stock = active, out-of-stock products with no movement/sale for this many
-- months. Surfaced on the clearance page for one-click disable (never automatic).
ALTER TABLE clearance_settings ADD COLUMN dead_months INTEGER NOT NULL DEFAULT 6;

-- +goose Down
ALTER TABLE clearance_settings DROP COLUMN dead_months;
