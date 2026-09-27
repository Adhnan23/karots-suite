-- +goose Up
-- Configurable font size for the barcode label's two free-text slots, as a TSPL
-- built-in font token: "1" 8x12 (smallest), "2" 12x20, "3" 16x24, "4" 24x32.
-- Defaults keep today's look (top = font 2, bottom = font 3). A smaller font
-- frees up room for more lines / a taller barcode.
ALTER TABLE settings ADD COLUMN label_name_font   TEXT NOT NULL DEFAULT '2';
ALTER TABLE settings ADD COLUMN label_bottom_font TEXT NOT NULL DEFAULT '3';

-- +goose Down
ALTER TABLE settings DROP COLUMN label_name_font;
ALTER TABLE settings DROP COLUMN label_bottom_font;
