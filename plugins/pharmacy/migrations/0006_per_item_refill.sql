-- +goose Up
-- Phase 6: per-drug refills. A dispense line can now name WHICH prescription row
-- it fulfils and how many days THAT drug's hand-over covers. So each medicine on
-- a script runs out — and reminds — on its own schedule: the cheap tablet bought
-- for a month and the expensive one bought for a week no longer share one
-- reminder. Still brand-agnostic: refill timing is supplied-qty ÷ doses-a-day,
-- never which product went out. Old rows keep prescription_item_id NULL / days 0
-- (no per-drug reminder) — the dispense header's days_supply still covered them.
ALTER TABLE pharmacy_dispense_items
    ADD COLUMN prescription_item_id BIGINT REFERENCES pharmacy_prescription_items(id) ON DELETE SET NULL,
    ADD COLUMN days_supply INTEGER NOT NULL DEFAULT 0;
CREATE INDEX pharmacy_dispense_items_rxitem ON pharmacy_dispense_items(prescription_item_id);

-- +goose Down
DROP INDEX IF EXISTS pharmacy_dispense_items_rxitem;
ALTER TABLE pharmacy_dispense_items DROP COLUMN days_supply;
ALTER TABLE pharmacy_dispense_items DROP COLUMN prescription_item_id;
