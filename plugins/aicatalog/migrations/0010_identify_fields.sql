-- +goose Up
-- The Product Plus fields Identify also fills (field keys, comma-separated), shop-wide.
ALTER TABLE aicatalog_settings ADD COLUMN identify_fields TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE aicatalog_settings DROP COLUMN identify_fields;
