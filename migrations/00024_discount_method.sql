-- +goose NO TRANSACTION
-- +goose Up
-- A discount a supplier gave at payment (ALTER TYPE ADD VALUE cannot run inside a transaction).
ALTER TYPE payment_method ADD VALUE IF NOT EXISTS 'discount';

-- +goose Down
-- Postgres cannot drop an enum value; intentional no-op.
SELECT 1;
