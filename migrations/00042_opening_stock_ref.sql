-- +goose Up
-- Opening stock (a product sheet's opening quantity, the demo's seed) puts stock
-- on the books; it is not a count that found more, so the P&L must not book it
-- as profit (RPT-9). New openings are marked by stock.AddStock; these are the
-- ones written before, known by their note.
UPDATE stock_movements SET reference_type = 'opening'
WHERE type = 'adjust' AND quantity > 0 AND reference_type IS NULL
  AND note IN ('CSV import opening stock', 'opening stock');

-- +goose Down
UPDATE stock_movements SET reference_type = NULL WHERE type = 'adjust' AND reference_type = 'opening';
