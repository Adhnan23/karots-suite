-- +goose Up
-- Giving a lot back to its supplier: when they were told about it, and who a
-- write-off was handed to (it waits on them until settled).
ALTER TABLE stock_batches ADD COLUMN supplier_told_on date;
ALTER TABLE stock_movements ADD COLUMN given_back_to bigint REFERENCES suppliers(id);

-- +goose Down
ALTER TABLE stock_batches DROP COLUMN supplier_told_on;
ALTER TABLE stock_movements DROP COLUMN given_back_to;
