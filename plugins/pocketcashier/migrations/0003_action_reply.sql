-- +goose Up
-- The reply of a call that made more than one thing (a new patient is also a new
-- customer), so later actions can find each one's real id.
ALTER TABLE pocket_actions ADD COLUMN result_reply jsonb;

-- +goose Down
ALTER TABLE pocket_actions DROP COLUMN result_reply;
