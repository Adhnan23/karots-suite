-- +goose Up
-- The shop's receipt text size, in percent (100 Normal, 115 Large, 130 Larger): the
-- whole slip picture is laid out narrower and scaled up to the paper.
ALTER TABLE settings ADD COLUMN receipt_text_size integer NOT NULL DEFAULT 100;

-- +goose Down
ALTER TABLE settings DROP COLUMN receipt_text_size;
