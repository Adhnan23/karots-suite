-- +goose Up
-- How many days before expiry a lot goes on the warning list: the shop's number,
-- unless the lot's supplier has its own (NULL = follow the shop).
ALTER TABLE settings ADD COLUMN expiry_warn_days integer NOT NULL DEFAULT 30;
ALTER TABLE suppliers ADD COLUMN expiry_warn_days integer;

-- +goose Down
ALTER TABLE settings DROP COLUMN expiry_warn_days;
ALTER TABLE suppliers DROP COLUMN expiry_warn_days;
