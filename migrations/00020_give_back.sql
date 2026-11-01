-- +goose Up
-- Giving a lot back to its supplier: when they were told about it, and which
-- write-offs were handed to them (these wait on the supplier until settled).
ALTER TABLE stock_batches ADD COLUMN supplier_told_on date;
ALTER TABLE stock_movements ADD COLUMN given_back boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE stock_batches DROP COLUMN supplier_told_on;
ALTER TABLE stock_movements DROP COLUMN given_back;
