-- +goose Up
-- Phase 4 (refill reminders). A prescription line gains a "doses per day" number
-- so quantity can be turned into days-of-supply: qty / per_day = days a fill
-- lasts, and last-dispense-date + that = when the patient runs out. Any schedule
-- collapses to a per-day count (three times daily = 3, twice = 2, at night = 1,
-- every 6 hours = 4). 0 = not tracked (the free-text dosage still prints on the
-- label). is_repeat / repeat_interval_days already exist on the prescription and
-- mark an ongoing (lifelong) script; a repeat recurs on schedule, a one-off is
-- only chased while a balance is still owed.
ALTER TABLE pharmacy_prescription_items ADD COLUMN per_day NUMERIC NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE pharmacy_prescription_items DROP COLUMN per_day;
