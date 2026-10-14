-- +goose Up
-- Let a prescription be stopped so it stops nagging in Refills due — e.g. a
-- repeat script whose patient never came back and can't be reached. Inactive
-- scripts are excluded from the refill reminders but kept for the record.
ALTER TABLE pharmacy_prescriptions ADD COLUMN active BOOLEAN NOT NULL DEFAULT true;

-- +goose Down
ALTER TABLE pharmacy_prescriptions DROP COLUMN active;
