-- +goose Up
-- What an action made (a payment, a patient, a job), so later actions that
-- refer to it by its temporary reference can find its real id.
ALTER TABLE pocket_actions ADD COLUMN result_id bigint;

-- +goose Down
ALTER TABLE pocket_actions DROP COLUMN result_id;
