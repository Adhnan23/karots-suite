-- +goose Up
-- Receipts always print as pictures: Classic text is only what a slip falls back to
-- if its picture can't be drawn. Labels keep label_print_as.
ALTER TABLE settings DROP COLUMN print_as;

-- +goose Down
ALTER TABLE settings ADD COLUMN print_as TEXT NOT NULL DEFAULT 'classic'
    CHECK (print_as IN ('classic', 'image'));
