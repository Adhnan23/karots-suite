-- +goose Up
-- Lines the till added together (a plugin's group, e.g. a deal): the group's
-- number within the sale, its label and how many sets were sold. NULL = a plain
-- line. Receipts print a group as one block; old sales have none.
ALTER TABLE sale_items
    ADD COLUMN line_group INT,
    ADD COLUMN line_group_label TEXT,
    ADD COLUMN line_group_sets NUMERIC(14,4);

-- +goose Down
ALTER TABLE sale_items DROP COLUMN line_group, DROP COLUMN line_group_label, DROP COLUMN line_group_sets;
