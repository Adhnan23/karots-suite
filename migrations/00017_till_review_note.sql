-- +goose Up
-- Why an admin should look at a till session (one opened for sales that came
-- in after their own session was closed).
ALTER TABLE cash_register ADD COLUMN review_note text;

-- +goose Down
ALTER TABLE cash_register DROP COLUMN review_note;
