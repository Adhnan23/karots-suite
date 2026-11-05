-- +goose Up
-- Whether the cash of a refused action has been dealt with: it's booked only once the
-- phone says the cashier really handed it over, which may be on a later try.
ALTER TABLE pocket_actions ADD COLUMN cash_settled boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE pocket_actions DROP COLUMN cash_settled;
