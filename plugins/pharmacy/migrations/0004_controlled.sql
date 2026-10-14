-- +goose Up
-- Pharmacy Phase 3: controlled-drug register. The "controlled" mark lives HERE,
-- in the plugin — never as a column on core products. A drug is controlled only
-- while a pharmacy_controlled_products row exists for it. The ledger is an
-- append-only in/out record; the running balance is computed at read time, so it
-- can never drift from the entries.

CREATE TABLE pharmacy_controlled_products (
    product_id BIGINT PRIMARY KEY REFERENCES products(id) ON DELETE CASCADE,
    schedule   TEXT NOT NULL DEFAULT '',   -- regulator schedule/class, free text
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE pharmacy_cd_ledger (
    id              BIGSERIAL PRIMARY KEY,
    product_id      BIGINT NOT NULL REFERENCES products(id),
    direction       TEXT NOT NULL CHECK (direction IN ('in','out')),
    quantity        NUMERIC NOT NULL,
    -- OUT rows raised automatically from a dispense point back at it; IN rows and
    -- manual corrections have no dispense.
    dispense_id     BIGINT REFERENCES pharmacy_dispenses(id) ON DELETE SET NULL,
    -- Names are SNAPSHOTS: the register is a legal record and must survive a
    -- patient rename/delete or prescription edit.
    patient_name    TEXT NOT NULL DEFAULT '',
    prescriber_name TEXT NOT NULL DEFAULT '',
    reference       TEXT NOT NULL DEFAULT '',   -- receipt no / supplier invoice
    notes           TEXT NOT NULL DEFAULT '',
    entry_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by      BIGINT REFERENCES users(id)
);
CREATE INDEX pharmacy_cd_ledger_product ON pharmacy_cd_ledger(product_id, entry_at, id);

-- +goose Down
DROP TABLE IF EXISTS pharmacy_cd_ledger;
DROP TABLE IF EXISTS pharmacy_controlled_products;
