-- +goose Up
-- Karots Shelf (the shop staff's product app): a cashier or sales person may use it
-- only when the admin ticks this; admins and managers always may.
ALTER TABLE users ADD COLUMN can_use_shelf boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE users DROP COLUMN can_use_shelf;
