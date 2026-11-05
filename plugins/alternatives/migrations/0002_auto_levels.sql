-- +goose Up
-- A group or tier learns its level from its members' levels, and a low one's order
-- follows what sells, unless the shop sets either by hand. Typed levels stay typed;
-- an unset 0 becomes Auto, as core's 00009 did for products. A typed 0 means don't track.
ALTER TABLE alt_tiers ADD COLUMN level_auto BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE alt_tiers ADD COLUMN split_auto BOOLEAN NOT NULL DEFAULT true;
UPDATE alt_tiers SET level_auto = false WHERE reorder_level > 0;
ALTER TABLE alt_members ADD COLUMN split_share INT CHECK (split_share BETWEEN 0 AND 100);

-- +goose Down
ALTER TABLE alt_members DROP COLUMN split_share;
ALTER TABLE alt_tiers DROP COLUMN split_auto;
ALTER TABLE alt_tiers DROP COLUMN level_auto;
