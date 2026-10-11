-- +goose Up
-- Fields per category: a field applies to the categories listed here and their
-- subcategories. No rows = every product (so a karots-pos backup restores with
-- no mapping: every old field stays global). category_id carries no FK — the
-- plugin's schema versions on its own; a deleted category just drops out.
CREATE TABLE pp_field_categories (
    field_id    BIGINT NOT NULL REFERENCES pp_fields(id) ON DELETE CASCADE,
    category_id BIGINT NOT NULL,
    PRIMARY KEY (field_id, category_id)
);

-- +goose Down
DROP TABLE pp_field_categories;
