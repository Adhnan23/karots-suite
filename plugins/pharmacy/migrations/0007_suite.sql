-- +goose Up
-- The suite's pharmacy (M7.7b). karots-pos 0001–0006 above stay byte for byte so
-- an old backup restores with no mapping; this adds what the suite needs.

-- Prescription-only medicines (a hint at the till, never a block). The controlled
-- mark and its schedule stay in pharmacy_controlled_products.
CREATE TABLE pharmacy_medicines (
    product_id BIGINT PRIMARY KEY REFERENCES products(id) ON DELETE CASCADE,
    rx_only    BOOLEAN NOT NULL DEFAULT false
);

-- Who the medicine went to and who prescribed it, as they were at the time: the
-- controlled register is a legal record and must survive a later rename / edit.
-- (A dispense restored from a karots-pos backup has these blank; readers fall
-- back to the live names.)
ALTER TABLE pharmacy_dispenses
    ADD COLUMN patient_name    TEXT NOT NULL DEFAULT '',
    ADD COLUMN prescriber_name TEXT NOT NULL DEFAULT '';
UPDATE pharmacy_dispenses d SET patient_name = c.name
FROM pharmacy_patients p JOIN customers c ON c.id = p.customer_id
WHERE p.id = d.patient_id;
UPDATE pharmacy_dispenses d SET prescriber_name = r.prescriber_name
FROM pharmacy_prescriptions r WHERE r.id = d.prescription_id;

-- A script's drug line names a medicine group (an alternatives group — any brand
-- of that molecule and strength fills it); called_at = the shop phoned about the
-- refill (hides the row from the call list for a few days).
ALTER TABLE pharmacy_prescription_items
    ADD COLUMN group_id  BIGINT REFERENCES alt_groups(id) ON DELETE SET NULL,
    ADD COLUMN called_at TIMESTAMPTZ;
UPDATE pharmacy_prescription_items i SET group_id = t.group_id
FROM alt_members m JOIN alt_tiers t ON t.id = m.tier_id
WHERE m.product_id = i.product_id;

-- +goose Down
ALTER TABLE pharmacy_prescription_items DROP COLUMN called_at, DROP COLUMN group_id;
ALTER TABLE pharmacy_dispenses DROP COLUMN prescriber_name, DROP COLUMN patient_name;
DROP TABLE IF EXISTS pharmacy_medicines;
