-- +goose Up
-- A recovery remembers enough to be undone exactly (REC-8): the lot a
-- replacement went into and that lot's cost before a top-up blended it, and
-- where a credit went — each bill (loss_recovery_bills) and the old debt.
-- NULL old_debt = a credit recorded before this, which can't be undone.
-- One recovery per loss is now the database's rule too (REC-1).
ALTER TABLE loss_recoveries
    ADD COLUMN lot_id bigint REFERENCES stock_batches(id) ON DELETE SET NULL,
    ADD COLUMN lot_cost_before numeric(14,4),
    ADD COLUMN old_debt numeric(14,2);

CREATE TABLE loss_recovery_bills (
    recovery_id bigint NOT NULL REFERENCES loss_recoveries(id) ON DELETE CASCADE,
    purchase_id bigint NOT NULL REFERENCES purchases(id),
    amount numeric(14,2) NOT NULL CHECK (amount > 0),
    PRIMARY KEY (recovery_id, purchase_id)
);

DROP INDEX idx_loss_recoveries_source;
CREATE UNIQUE INDEX loss_recoveries_one_per_loss ON loss_recoveries (source_type, source_id);

-- +goose Down
DROP INDEX loss_recoveries_one_per_loss;
CREATE INDEX idx_loss_recoveries_source ON loss_recoveries USING btree (source_type, source_id);
DROP TABLE loss_recovery_bills;
ALTER TABLE loss_recoveries DROP COLUMN lot_id, DROP COLUMN lot_cost_before, DROP COLUMN old_debt;
