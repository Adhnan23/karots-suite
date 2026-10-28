-- +goose Up

-- A job's type (material) beside its size: Gloss / Matte / 125 mic pouches, photo
-- or sticker paper. '' = no type. Each size + type has its own prices and paper
-- (its own pouch); the job ledger keeps it so the report can tell them apart.
ALTER TABLE doc_price ADD COLUMN material VARCHAR(30) NOT NULL DEFAULT '';
ALTER TABLE doc_consumable ADD COLUMN material VARCHAR(30) NOT NULL DEFAULT '';
ALTER TABLE doc_job ADD COLUMN material VARCHAR(30) NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE doc_job DROP COLUMN material;
ALTER TABLE doc_consumable DROP COLUMN material;
ALTER TABLE doc_price DROP COLUMN material;
