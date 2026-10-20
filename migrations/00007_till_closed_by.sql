-- +goose Up
-- Who closed a till when it wasn't its own cashier (an admin or manager closing a
-- till left open). NULL = closed by its owner — every older row and every restored
-- older backup reads correctly.
ALTER TABLE cash_register ADD COLUMN closed_by BIGINT REFERENCES users(id) ON DELETE RESTRICT;

-- +goose Down
ALTER TABLE cash_register DROP COLUMN closed_by;
