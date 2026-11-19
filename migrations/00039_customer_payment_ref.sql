-- +goose Up
-- A credit payment's one-time reference (CUST-16): the same Pay sent twice (a
-- double tap, Pay again after a lost answer) is one payment.
ALTER TABLE customer_payments ADD COLUMN client_ref text;
CREATE UNIQUE INDEX customer_payments_client_ref ON customer_payments (customer_id, client_ref) WHERE client_ref IS NOT NULL;

-- +goose Down
DROP INDEX customer_payments_client_ref;
ALTER TABLE customer_payments DROP COLUMN client_ref;
