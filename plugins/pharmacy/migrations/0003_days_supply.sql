-- +goose Up
-- Phase 4 refill reminders, dispense-driven. Each hand-over records how many
-- days it covers; the next-due date is simply dispensed_at + days_supply. This
-- is deliberately NOT tied to which product went out, so brand substitution,
-- mixed brands and full-card-plus-cut-loose fills all work — the pharmacist just
-- says how many days they supplied (defaulted from the script's qty ÷ doses-a-
-- day, editable). 0 = no reminder.
ALTER TABLE pharmacy_dispenses ADD COLUMN days_supply INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE pharmacy_dispenses DROP COLUMN days_supply;
