-- +goose Up
-- A supplier replacing expired or damaged stock may ask a little extra (the
-- item costs more now). extra_charge is that total: it is added to what we owe
-- the supplier and to the replacement batch's cost. Old recoveries have none.
ALTER TABLE loss_recoveries ADD COLUMN extra_charge NUMERIC(12,2) NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE loss_recoveries DROP COLUMN extra_charge;
