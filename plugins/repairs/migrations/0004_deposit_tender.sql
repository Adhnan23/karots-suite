-- +goose NO TRANSACTION
-- +goose Up
-- A repair's collection sale applies the deposit already taken as its own
-- 'deposit' tender (the cash entered the drawer when it was taken). It used to
-- borrow another plugin's 'wallet' value, which fails when that plugin isn't
-- installed. ALTER TYPE ADD VALUE cannot run inside a transaction.
ALTER TYPE payment_method ADD VALUE IF NOT EXISTS 'deposit';

-- +goose Down
-- Postgres cannot drop an enum value; intentional no-op.
SELECT 1;
