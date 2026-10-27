-- +goose Up
-- How labels print: 'classic' (TSPL text + the printer's own barcode, the old
-- way) or 'image' (a 1-bit picture of the label — what you see is what prints).
-- Every shop and old backup stays Classic until someone switches it.
-- label_styles: the Image-mode style per label kind ({"price":"qrtag",
-- "<plugin>.<kind>":"<style>"}); an open map, so plugin kinds need no column.
ALTER TABLE settings ADD COLUMN label_print_as TEXT NOT NULL DEFAULT 'classic'
    CHECK (label_print_as IN ('classic', 'image'));
ALTER TABLE settings ADD COLUMN label_styles JSONB NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE settings DROP COLUMN label_styles;
ALTER TABLE settings DROP COLUMN label_print_as;
