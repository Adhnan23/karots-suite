-- +goose Up
-- 00004 copied every any-size consumable into core recipes as whole units. A
-- fractional one (a toner rated 5000 copies = 0.0002 per copy) rounded up to a
-- whole cartridge per job. Only the rows 00004 could have copied are touched.
UPDATE product_recipes pr SET whole_units = false
FROM doc_consumable dc JOIN doc_service s ON s.id = dc.service_id
WHERE dc.size IS NULL AND pr.product_id = s.product_id AND pr.component_product_id = dc.product_id
  AND pr.whole_units AND pr.qty_per_unit <> trunc(pr.qty_per_unit);

-- +goose Down
SELECT 1;
