-- +goose Up
-- A barcode a cashier linked to a product at the counter: it works at once and waits
-- on Items to review, in case it belongs to something else.
ALTER TABLE product_barcodes
    ADD COLUMN set_at_till boolean NOT NULL DEFAULT false,
    ADD COLUMN added_by bigint REFERENCES users(id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE product_barcodes DROP COLUMN set_at_till, DROP COLUMN added_by;
