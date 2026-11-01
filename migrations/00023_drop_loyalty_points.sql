-- +goose Up
-- Never used; loyalty will be a plugin with its own tables.
ALTER TABLE customers DROP COLUMN loyalty_points;

-- +goose Down
ALTER TABLE customers ADD COLUMN loyalty_points integer NOT NULL DEFAULT 0;
