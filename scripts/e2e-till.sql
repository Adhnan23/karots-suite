-- Till fixtures for Playwright. Runs after `-reset` + e2e-users.sql (make e2e-seed).
-- Barcodes are fixed; the specs scan them.
UPDATE settings SET ask_to_print = true, allow_untracked_cash = true, allow_continue_last_count = true, open_cash_drawer = false;

INSERT INTO categories (name) VALUES ('E2E Goods');

INSERT INTO products (name, barcode, category_id, unit_id, cost_price, selling_price)
SELECT v.name, v.barcode, c.id, 1, v.cost, v.price
FROM (VALUES
  ('E2E Milk',     '4790000000011', 80, 100),
  ('E2E Lot Soap', '4790000000028', 50, 100),
  ('E2E Yoghurt',  '4790000000035', 40,  80),
  ('E2E Scarce',   '4790000000042', 50, 250)
) AS v(name, barcode, cost, price), categories c
WHERE c.name = 'E2E Goods';

INSERT INTO stock (product_id, quantity)
SELECT id, CASE name WHEN 'E2E Milk' THEN 500 WHEN 'E2E Lot Soap' THEN 10 WHEN 'E2E Yoghurt' THEN 6 ELSE 1 END
FROM products WHERE name LIKE 'E2E %'
ON CONFLICT (product_id) DO UPDATE SET quantity = EXCLUDED.quantity;

-- selling_price is NOT NULL (0 = "no own price, inherit the product's" — see
-- stock.PriceOptions' own_price check), so 0 stands in for "no lot price" below.
INSERT INTO stock_batches (product_id, qty_received, qty_remaining, cost_price, selling_price, expiry_date)
SELECT p.id, v.qty, v.qty, 50, v.price, v.expiry
FROM (VALUES
  ('E2E Milk',     500, 0::numeric, NULL::date),
  ('E2E Lot Soap',   5, 0,          NULL),
  ('E2E Lot Soap',   5, 120,        NULL),
  ('E2E Yoghurt',    3, 0,          CURRENT_DATE - 1),
  ('E2E Yoghurt',    3, 0,          CURRENT_DATE + 180),
  ('E2E Scarce',     1, 0,          NULL)
) AS v(name, qty, price, expiry)
JOIN products p ON p.name = v.name;

INSERT INTO product_groups (name, emoji) VALUES ('E2E Dairy', '🥛');
INSERT INTO product_group_items (group_id, product_id)
SELECT g.id, p.id FROM product_groups g, products p
WHERE g.name = 'E2E Dairy' AND p.name IN ('E2E Milk', 'E2E Yoghurt');

INSERT INTO customers (name, phone, credit_limit) VALUES ('E2E Credit', '0771000001', 1000);
INSERT INTO lockers (name, kind, cashier_access) VALUES ('E2E Card machine', 'card', true);
