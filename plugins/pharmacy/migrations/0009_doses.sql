-- +goose Up
-- The shop's dose list: how a dose is written (1-0-1), what it means on a dose
-- label (Morning · Night) and doses a day (0 = when needed: no days). Starts with
-- the usual ones; a dose used on a prescription or as a usual dose joins it by
-- itself (unnamed until the shop names it). Editable, deletable.
CREATE TABLE pharmacy_doses (
    id         BIGSERIAL PRIMARY KEY,
    dose       TEXT NOT NULL,
    words      TEXT NOT NULL DEFAULT '',
    per_day    NUMERIC(6,2) NOT NULL DEFAULT 0 CHECK (per_day >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX pharmacy_doses_dose ON pharmacy_doses (lower(trim(dose)));
-- The usual doses are put in by the plugin whenever the list is empty (a full
-- restore empties it too) — see seedDoses.

-- How to take a medicine (After meals, Empty stomach — 30 min before food …): the
-- product's usual one and each prescription line's; printed on the dose label.
ALTER TABLE pharmacy_medicines ADD COLUMN note TEXT NOT NULL DEFAULT '';
ALTER TABLE pharmacy_prescription_items ADD COLUMN note TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE pharmacy_prescription_items DROP COLUMN note;
ALTER TABLE pharmacy_medicines DROP COLUMN note;
DROP TABLE IF EXISTS pharmacy_doses;
