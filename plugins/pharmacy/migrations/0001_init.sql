-- +goose Up
-- Pharmacy plugin, Phase 1: patients (a clinical profile on top of a core
-- customer), prescriptions with items, and dispensing records linked to a core
-- sale. All tables are plugin-owned; they reference core tables (customers,
-- products, sales, users) at the data level only — core code never references
-- these.

-- A customer BECOMES a patient when this row exists. Not every customer is one.
CREATE TABLE pharmacy_patients (
    id          BIGSERIAL PRIMARY KEY,
    customer_id BIGINT NOT NULL UNIQUE REFERENCES customers(id),
    dob         DATE,
    sex         TEXT,
    nic         TEXT,               -- national ID / patient reference (optional)
    allergies   TEXT NOT NULL DEFAULT '',
    notes       TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE pharmacy_prescriptions (
    id                   BIGSERIAL PRIMARY KEY,
    patient_id           BIGINT NOT NULL REFERENCES pharmacy_patients(id) ON DELETE CASCADE,
    prescriber_name      TEXT NOT NULL DEFAULT '',
    prescriber_reg       TEXT NOT NULL DEFAULT '',
    prescribed_on        DATE,
    notes                TEXT NOT NULL DEFAULT '',
    is_repeat            BOOLEAN NOT NULL DEFAULT false,     -- Phase 4
    repeat_interval_days INTEGER NOT NULL DEFAULT 0,         -- Phase 4
    created_by           BIGINT REFERENCES users(id),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX pharmacy_prescriptions_patient ON pharmacy_prescriptions(patient_id);

-- A line either points at a stocked core product OR carries free text (a drug
-- not in the catalogue). dosage is free text (e.g. "1 tablet twice daily").
CREATE TABLE pharmacy_prescription_items (
    id              BIGSERIAL PRIMARY KEY,
    prescription_id BIGINT NOT NULL REFERENCES pharmacy_prescriptions(id) ON DELETE CASCADE,
    product_id      BIGINT REFERENCES products(id) ON DELETE SET NULL,
    drug_text       TEXT NOT NULL DEFAULT '',
    dosage          TEXT NOT NULL DEFAULT '',
    quantity        NUMERIC NOT NULL DEFAULT 0
);
CREATE INDEX pharmacy_prescription_items_rx ON pharmacy_prescription_items(prescription_id);

CREATE TABLE pharmacy_dispenses (
    id              BIGSERIAL PRIMARY KEY,
    patient_id      BIGINT NOT NULL REFERENCES pharmacy_patients(id),
    prescription_id BIGINT REFERENCES pharmacy_prescriptions(id) ON DELETE SET NULL,
    sale_id         BIGINT REFERENCES sales(id) ON DELETE SET NULL,
    sale_receipt    TEXT NOT NULL DEFAULT '',
    dispensed_by    BIGINT REFERENCES users(id),
    dispensed_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    notes           TEXT NOT NULL DEFAULT ''
);
CREATE INDEX pharmacy_dispenses_patient ON pharmacy_dispenses(patient_id);

CREATE TABLE pharmacy_dispense_items (
    id          BIGSERIAL PRIMARY KEY,
    dispense_id BIGINT NOT NULL REFERENCES pharmacy_dispenses(id) ON DELETE CASCADE,
    product_id  BIGINT REFERENCES products(id) ON DELETE SET NULL,
    drug_text   TEXT NOT NULL DEFAULT '',
    quantity    NUMERIC NOT NULL DEFAULT 0
);
CREATE INDEX pharmacy_dispense_items_dispense ON pharmacy_dispense_items(dispense_id);

-- +goose Down
DROP TABLE IF EXISTS pharmacy_dispense_items;
DROP TABLE IF EXISTS pharmacy_dispenses;
DROP TABLE IF EXISTS pharmacy_prescription_items;
DROP TABLE IF EXISTS pharmacy_prescriptions;
DROP TABLE IF EXISTS pharmacy_patients;
