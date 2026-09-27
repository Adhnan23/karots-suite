-- +goose Up
-- Configurable wrapped-line counts for the barcode label's two free-text slots
-- (top and bottom around the barcode). Default 1 = the historical single line;
-- a shop can raise either to 2 or 3 to fit long names (vehicle parts, biscuits)
-- and tune it against a real printer without a code change.
ALTER TABLE settings ADD COLUMN label_name_lines   integer NOT NULL DEFAULT 1;
ALTER TABLE settings ADD COLUMN label_bottom_lines integer NOT NULL DEFAULT 1;

-- +goose Down
ALTER TABLE settings DROP COLUMN label_name_lines;
ALTER TABLE settings DROP COLUMN label_bottom_lines;
