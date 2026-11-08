-- +goose Up
-- The shop picks the unit new products start with (was always "pcs").
ALTER TABLE units ADD COLUMN is_default boolean NOT NULL DEFAULT false;
UPDATE units SET is_default = true
 WHERE id = (SELECT id FROM units ORDER BY (abbreviation = 'pcs') DESC, id LIMIT 1);
CREATE UNIQUE INDEX units_one_default ON units (is_default) WHERE is_default;

-- +goose Down
DROP INDEX units_one_default;
ALTER TABLE units DROP COLUMN is_default;
