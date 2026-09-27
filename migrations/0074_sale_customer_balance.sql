-- +goose Up
-- customer_balance_after snapshots the customer's account right after a credit
-- sale, so the receipt (and every reprint) tells the truth: a sale covered by a
-- prior advance prints "Paid from advance" + what's left, not a phantom TOTAL DUE.
-- NULL on older sales: the receipt falls back to the live balance.
ALTER TABLE sales ADD COLUMN IF NOT EXISTS customer_balance_after numeric;

-- +goose Down
ALTER TABLE sales DROP COLUMN IF EXISTS customer_balance_after;
