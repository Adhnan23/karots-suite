-- +goose Up
-- A medicine's usual dose (1-0-1, tds, …): fills a prescription line and the
-- till's Days… — always editable. Controlled drugs are prescription-only.
ALTER TABLE pharmacy_medicines ADD COLUMN dose TEXT NOT NULL DEFAULT '';
INSERT INTO pharmacy_medicines (product_id, rx_only)
SELECT product_id, true FROM pharmacy_controlled_products
ON CONFLICT (product_id) DO UPDATE SET rx_only = true;

-- +goose Down
ALTER TABLE pharmacy_medicines DROP COLUMN dose;
