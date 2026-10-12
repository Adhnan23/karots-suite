-- +goose Up

-- The suite splits what a job uses in two, with no overlap:
--   doc_consumable = PAPER (or film): whole sheets, counted per sheet, so a
--                    double-sided job uses half; per size, or NULL = any size.
--   product_recipes = everything used per COPY (toner, ink), fractional.
--
-- karots-pos 00004 copied every NULL-size consumable into recipes with
-- whole_units = true and left it in doc_consumable too. A fractional one (a toner
-- rated 5000 copies = 0.0002) is not paper: rounded up to whole units it took a
-- whole cartridge per job. Those become recipe-only, fractional.
INSERT INTO product_recipes (product_id, component_product_id, qty_per_unit, whole_units)
SELECT s.product_id, dc.product_id, dc.qty_per_unit, false
FROM doc_consumable dc
JOIN doc_service s ON s.id = dc.service_id
WHERE dc.size IS NULL AND dc.qty_per_unit <> trunc(dc.qty_per_unit)
ON CONFLICT (product_id, component_product_id) DO UPDATE SET whole_units = false;

DELETE FROM doc_consumable
WHERE size IS NULL AND qty_per_unit <> trunc(qty_per_unit);

-- A whole-sheet NULL-size row is paper and stays here; the recipe copy 00004 made
-- of it would count the same sheets again (per copy, not per sheet).
DELETE FROM product_recipes pr
USING doc_consumable dc, doc_service s
WHERE s.id = dc.service_id AND dc.size IS NULL
  AND pr.product_id = s.product_id AND pr.component_product_id = dc.product_id;

-- The job ledger is written after the sale and linked to its line, so the report
-- can take returned copies off; the options make "A4 colour 2-side" countable.
-- Jobs from karots-pos have none of these (shown as earlier jobs).
ALTER TABLE doc_job ADD COLUMN sale_item_id BIGINT REFERENCES sale_items (id) ON DELETE SET NULL;
ALTER TABLE doc_job ADD COLUMN size VARCHAR(20);
ALTER TABLE doc_job ADD COLUMN color BOOLEAN;
ALTER TABLE doc_job ADD COLUMN double_side BOOLEAN;
ALTER TABLE doc_job ADD COLUMN sheets DECIMAL(14,3);
CREATE UNIQUE INDEX idx_doc_job_sale_item ON doc_job (sale_item_id);

-- +goose Down
-- The consumable split is not undone (karots-pos reads both lists anyway).
DROP INDEX IF EXISTS idx_doc_job_sale_item;
ALTER TABLE doc_job DROP COLUMN sheets;
ALTER TABLE doc_job DROP COLUMN double_side;
ALTER TABLE doc_job DROP COLUMN color;
ALTER TABLE doc_job DROP COLUMN size;
ALTER TABLE doc_job DROP COLUMN sale_item_id;
