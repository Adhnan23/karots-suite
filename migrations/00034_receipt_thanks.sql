-- +goose Up
-- The shop's thank-you line at the foot of every receipt ("Get well soon!" for a pharmacy).
-- '' = each receipt style's own wording.
ALTER TABLE settings ADD COLUMN receipt_thanks text NOT NULL DEFAULT '';
-- "compact" is a receipt style again (the eleventh, 2026-11-12). Before 2026-10-21 it was an old value
-- that read as classic: a shop still holding that keeps classic instead of switching by itself.
UPDATE settings SET receipt_style = 'classic' WHERE receipt_style = 'compact';

-- +goose Down
ALTER TABLE settings DROP COLUMN receipt_thanks;
