-- +goose Up
-- Float dealers are ordinary suppliers: buying float is a core supplier bill
-- (owed, or paid on the spot), linked from its refill row. A carrier remembers
-- its usual dealer so the Float job knows which devices to offer first.
ALTER TABLE recharge_transactions ADD COLUMN purchase_id BIGINT REFERENCES purchases(id) ON DELETE SET NULL;
ALTER TABLE recharge_carriers ADD COLUMN supplier_id BIGINT REFERENCES suppliers(id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE recharge_carriers DROP COLUMN supplier_id;
ALTER TABLE recharge_transactions DROP COLUMN purchase_id;
