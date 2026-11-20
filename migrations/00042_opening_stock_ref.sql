-- +goose Up
-- Stock put on the books is not a count that found more, so the P&L must never
-- book it as profit (RPT-9; users' answers Q5, Q6, 2026-11-20): an opening
-- quantity (a product sheet, the demo's seed), stock added with no bill
-- (+ Add batch, intake), and a product's very first stock however it came in.
-- New ones are marked by the stock package; these are the ones written before.
UPDATE stock_movements m SET reference_type = 'opening'
WHERE m.type = 'adjust' AND m.quantity > 0 AND m.reference_type IS NULL
  AND (m.note IN ('CSV import opening stock', 'opening stock', 'batch added', 'stock intake', 'stock intake restock')
       OR NOT EXISTS (SELECT 1 FROM stock_movements e WHERE e.product_id = m.product_id
                      AND (e.created_at, e.id) < (m.created_at, m.id)));

-- +goose Down
UPDATE stock_movements SET reference_type = NULL WHERE type = 'adjust' AND reference_type = 'opening';
