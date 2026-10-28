-- +goose Up

-- A line group the receipt prints item by item: the label with the group's total,
-- then each member with its own price (a priced job and its add-ons). The other
-- groups (a combo) keep "sets x a set's price" with the members unpriced.
ALTER TABLE sale_items ADD COLUMN line_group_itemised BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE sale_items DROP COLUMN line_group_itemised;
