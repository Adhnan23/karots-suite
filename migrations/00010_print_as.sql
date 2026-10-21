-- +goose Up
-- How receipts print: 'classic' (ESC/POS text, the old way) or 'image' (a 1-bit
-- picture of the on-screen slip — what you see is what prints). Every shop, and
-- every old backup, stays Classic until someone switches it.
ALTER TABLE settings ADD COLUMN print_as TEXT NOT NULL DEFAULT 'classic'
    CHECK (print_as IN ('classic', 'image'));

-- +goose Down
ALTER TABLE settings DROP COLUMN print_as;
