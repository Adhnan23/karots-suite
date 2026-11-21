-- +goose Up
-- A sold row is written with its sale (the sale-line hook) and names the sale
-- lines it was worked out from, so the auditor can hold it to them and the report
-- can take returned sets off. Rows from before have none (read as stored).
ALTER TABLE combo_sales ADD COLUMN item_ids BIGINT[] NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE combo_sales DROP COLUMN item_ids;
