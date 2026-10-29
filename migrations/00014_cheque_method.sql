-- +goose NO TRANSACTION
-- +goose Up
-- A 'cheque' payment method (ALTER TYPE ADD VALUE cannot run inside a transaction).
ALTER TYPE payment_method ADD VALUE IF NOT EXISTS 'cheque';

-- +goose Down
-- Postgres cannot drop an enum value; intentional no-op.
SELECT 1;
