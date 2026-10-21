-- Smart reorder levels demo (dev DB only — `make demo-reorder`, after `-reset -demo`).
-- A year of daily sales and supplier deliveries for a dozen products, each showing
-- one case: steady seller, last year's school rush, irregular supplier + one big
-- bill, Fixed levels that look off, a bag sold loose, a seller that ran out, one
-- not selling, a new quick-added item. Sales go straight into the tables (no
-- payments or drawer moves), so money reports won't add up — it's for Reorder.
BEGIN;

CREATE TEMP TABLE demo_p (key text PRIMARY KEY, id bigint) ON COMMIT DROP;

CREATE FUNCTION pg_temp.ts(d int) RETURNS timestamptz LANGUAGE sql AS
  $$ SELECT now() - make_interval(days => d) - interval '2 hours' $$;

CREATE FUNCTION pg_temp.uid() RETURNS bigint LANGUAGE sql AS
  $$ SELECT id FROM users WHERE role = 'admin' AND NOT is_system ORDER BY id LIMIT 1 $$;

CREATE FUNCTION pg_temp.cat(n text) RETURNS bigint LANGUAGE plpgsql AS $$
DECLARE i bigint;
BEGIN
  SELECT id INTO i FROM categories WHERE name = n;
  IF i IS NULL THEN INSERT INTO categories (name) VALUES (n) RETURNING id INTO i; END IF;
  RETURN i;
END $$;

CREATE FUNCTION pg_temp.sup(n text) RETURNS bigint LANGUAGE plpgsql AS $$
DECLARE i bigint;
BEGIN
  SELECT id INTO i FROM suppliers WHERE name = n;
  IF i IS NULL THEN INSERT INTO suppliers (name) VALUES (n) RETURNING id INTO i; END IF;
  RETURN i;
END $$;

-- A product, created `age` days ago.
CREATE FUNCTION pg_temp.prod(k text, n text, cat text, unit text, cost numeric, price numeric,
                             auto boolean, lvl int, sup text, age int) RETURNS void LANGUAGE plpgsql AS $$
DECLARE i bigint;
BEGIN
  INSERT INTO products (name, category_id, unit_id, cost_price, selling_price, reorder_auto, reorder_level, reorder_start,
                        preferred_supplier_id, created_at)
  VALUES (n, pg_temp.cat(cat), (SELECT id FROM units WHERE abbreviation = unit), cost, price, auto, lvl, lvl,
          CASE WHEN sup IS NULL THEN NULL ELSE pg_temp.sup(sup) END, pg_temp.ts(age))
  RETURNING id INTO i;
  INSERT INTO demo_p VALUES (k, i);
END $$;

-- The day's bill (one per day, all demo products on it).
CREATE FUNCTION pg_temp.sale(d int) RETURNS bigint LANGUAGE plpgsql AS $$
DECLARE i bigint;
BEGIN
  SELECT id INTO i FROM sales WHERE receipt_no = 'DEMO-' || d;
  IF i IS NULL THEN
    INSERT INTO sales (receipt_no, cashier_id, created_at) VALUES ('DEMO-' || d, pg_temp.uid(), pg_temp.ts(d)) RETURNING id INTO i;
  END IF;
  RETURN i;
END $$;

CREATE FUNCTION pg_temp.sell1(k text, d int, q numeric) RETURNS void LANGUAGE plpgsql AS $$
DECLARE p record; s bigint;
BEGIN
  IF q <= 0 THEN RETURN; END IF;
  SELECT pr.id, pr.selling_price INTO p FROM products pr JOIN demo_p dp ON dp.id = pr.id WHERE dp.key = k;
  s := pg_temp.sale(d);
  INSERT INTO sale_items (sale_id, product_id, quantity, unit_price, subtotal) VALUES (s, p.id, q, p.selling_price, q * p.selling_price);
  INSERT INTO stock_movements (product_id, type, quantity, reference_id, reference_type, user_id, created_at)
  VALUES (p.id, 'sale', -q, s, 'sale', pg_temp.uid(), pg_temp.ts(d));
END $$;

-- `rate` a day over days [from, to) ago, spread evenly (whole units for whole-unit products).
CREATE FUNCTION pg_temp.sell(k text, rate numeric, dfrom int, dto int, whole boolean DEFAULT true) RETURNS void LANGUAGE plpgsql AS $$
BEGIN
  FOR d IN dfrom .. dto - 1 LOOP
    PERFORM pg_temp.sell1(k, d, CASE WHEN whole THEN floor((d + 1) * rate) - floor(d * rate) ELSE rate END);
  END LOOP;
END $$;

-- A delivery of q from the product's supplier, d days ago.
CREATE FUNCTION pg_temp.deliver(k text, d int, q numeric) RETURNS void LANGUAGE plpgsql AS $$
DECLARE p record; pu bigint;
BEGIN
  SELECT pr.id, pr.cost_price, pr.selling_price, pr.preferred_supplier_id AS sup INTO p FROM products pr JOIN demo_p dp ON dp.id = pr.id WHERE dp.key = k;
  SELECT id INTO pu FROM purchases WHERE supplier_id = p.sup AND notes = 'DEMO-' || d;
  IF pu IS NULL THEN
    INSERT INTO purchases (supplier_id, status, notes, created_at, received_by) VALUES (p.sup, 'received', 'DEMO-' || d, pg_temp.ts(d), pg_temp.uid()) RETURNING id INTO pu;
  END IF;
  INSERT INTO purchase_items (purchase_id, product_id, quantity, cost_price, selling_price, subtotal) VALUES (pu, p.id, q, p.cost_price, p.selling_price, q * p.cost_price);
  UPDATE purchases SET subtotal = subtotal + q * p.cost_price, total = total + q * p.cost_price WHERE id = pu;
  INSERT INTO stock_movements (product_id, type, quantity, reference_id, reference_type, user_id, created_at)
  VALUES (p.id, 'purchase', q, pu, 'purchase', pg_temp.uid(), pg_temp.ts(d));
END $$;

CREATE FUNCTION pg_temp.every(k text, gap int, first int, q numeric, dto int DEFAULT 400) RETURNS void LANGUAGE plpgsql AS $$
DECLARE d int := first;
BEGIN
  WHILE d < dto LOOP PERFORM pg_temp.deliver(k, d, q); d := d + gap; END LOOP;
END $$;

-- Ends with `target` on hand: an opening count 401 days ago makes the moves add up.
CREATE FUNCTION pg_temp.finish(k text, target numeric) RETURNS void LANGUAGE plpgsql AS $$
DECLARE p record; moved numeric;
BEGIN
  SELECT pr.id, pr.cost_price INTO p FROM products pr JOIN demo_p dp ON dp.id = pr.id WHERE dp.key = k;
  SELECT COALESCE(sum(quantity), 0) INTO moved FROM stock_movements WHERE product_id = p.id;
  INSERT INTO stock_movements (product_id, type, quantity, user_id, note, created_at)
  VALUES (p.id, 'adjust', target - moved, pg_temp.uid(), 'demo opening count', pg_temp.ts(401));
  INSERT INTO stock (product_id, quantity) VALUES (p.id, target) ON CONFLICT (product_id) DO UPDATE SET quantity = EXCLUDED.quantity;
  IF target > 0 THEN
    INSERT INTO stock_batches (product_id, qty_received, qty_remaining, cost_price, source) VALUES (p.id, target, target, p.cost_price, 'opening');
  END IF;
END $$;

-- The products. Fixed ones existed before smart levels; the rest are Auto.
SELECT pg_temp.prod('pencil',  'Atlas Pencil HB',          'Stationery', 'pcs', 25, 40,    true,  10, 'Lanka Stationers', 420);
SELECT pg_temp.prod('book',    'Exercise Book 120 pages',  'Stationery', 'pcs', 140, 190,  true,  50, 'Lanka Stationers', 420);
SELECT pg_temp.prod('pen',     'Ballpoint Pen Blue',       'Stationery', 'pcs', 30, 50,    false, 10, 'Lanka Stationers', 420);
SELECT pg_temp.prod('hilite',  'Highlighter Yellow',       'Stationery', 'pcs', 90, 140,   true,  0,  NULL, 10);
SELECT pg_temp.prod('milk',    'Anchor Milk Powder 400g',  'Groceries',  'pcs', 1020, 1180, true, 10, 'Cargills Distributors', 420);
SELECT pg_temp.prod('marie',   'Maliban Marie Biscuits',   'Groceries',  'pcs', 160, 200,  true,  40, 'Cargills Distributors', 420);
SELECT pg_temp.prod('oil',     'Coconut Oil 1L',           'Groceries',  'pcs', 620, 750,  false, 5,  'Cargills Distributors', 420);
SELECT pg_temp.prod('soap',    'Sunlight Soap 110g',       'Household',  'pcs', 165, 210,  false, 20, 'Unilever Lanka', 420);
SELECT pg_temp.prod('wrap',    'Gift Wrap Roll',           'Household',  'pcs', 60, 100,   false, 15, NULL, 420);
SELECT pg_temp.prod('clips',   'Hair Clip Set',            'Household',  'pcs', 90, 150,   true,  5,  NULL, 420);
SELECT pg_temp.prod('rice',    'Samba Rice 25kg Bag',      'Rice & Grains', 'pcs', 6250, 7000, true, 4, 'Kurunegala Rice Mill', 420);
SELECT pg_temp.prod('riceL',   'Samba Rice (loose)',       'Rice & Grains', 'kg', 250, 290, true, 10, 'Kurunegala Rice Mill', 420);
INSERT INTO product_conversions (from_product_id, to_product_id, ratio)
SELECT (SELECT id FROM demo_p WHERE key = 'rice'), (SELECT id FROM demo_p WHERE key = 'riceL'), 25;

-- Steady seller, weekly supplier → ~10 a day × (7 + 3).
SELECT pg_temp.sell('pencil', 10, 0, 400);   SELECT pg_temp.every('pencil', 7, 3, 70);    SELECT pg_temp.finish('pencil', 6);
-- 2 a day now, but 15 a day in the coming weeks last year (school term) → plans for 15.
SELECT pg_temp.sell('book', 2, 0, 345);      SELECT pg_temp.sell('book', 15, 345, 368);  SELECT pg_temp.sell('book', 2, 368, 400);
SELECT pg_temp.every('book', 10, 5, 30);     SELECT pg_temp.finish('book', 40);
-- Fixed 10, sells 6 a day → looks off.
SELECT pg_temp.sell('pen', 6, 0, 400);       SELECT pg_temp.every('pen', 7, 3, 42);      SELECT pg_temp.finish('pen', 8);
-- Quick-added 10 days ago, no supplier → learning, shop default rhythm.
SELECT pg_temp.sell('hilite', 3, 0, 10);     SELECT pg_temp.finish('hilite', 5);
-- 1.2 a day, irregular supplier (gaps 5 5 5 14), one customer bought 30 five days ago.
SELECT pg_temp.sell('milk', 1.2, 0, 400);    SELECT pg_temp.sell1('milk', 5, 30);
SELECT pg_temp.deliver('milk', 4, 12);       SELECT pg_temp.deliver('milk', 18, 10);     SELECT pg_temp.deliver('milk', 23, 8);
SELECT pg_temp.deliver('milk', 28, 8);       SELECT pg_temp.deliver('milk', 33, 8);      SELECT pg_temp.finish('milk', 8);
-- A fast seller that ran out 12 days ago: the empty days don't slow it down.
SELECT pg_temp.sell('marie', 8, 12, 400);    SELECT pg_temp.every('marie', 7, 15, 56);   SELECT pg_temp.finish('marie', 0);
-- Fixed 5, sells 1.4 a day → looks off (not low).
SELECT pg_temp.sell('oil', 1.4, 0, 400);     SELECT pg_temp.every('oil', 7, 1, 10);      SELECT pg_temp.finish('oil', 20);
-- Fixed 20, sells 0.6 a day → looks off and low.
SELECT pg_temp.sell('soap', 0.6, 0, 400);    SELECT pg_temp.every('soap', 7, 2, 4);      SELECT pg_temp.finish('soap', 12);
-- Fixed 15, hardly sells, no supplier → looks off.
SELECT pg_temp.sell('wrap', 0.2, 0, 400);    SELECT pg_temp.finish('wrap', 10);
-- Sold for a while months ago, nothing since → Not selling.
SELECT pg_temp.sell('clips', 0.5, 200, 260); SELECT pg_temp.finish('clips', 0);
-- Bags: a few sold whole, most opened and sold loose (12 kg a day = ~½ bag) → counted on the bag.
SELECT pg_temp.sell('rice', 0.1, 0, 400);    SELECT pg_temp.sell('riceL', 12, 0, 400, false);
SELECT pg_temp.every('rice', 14, 6, 9);      SELECT pg_temp.finish('rice', 3);           SELECT pg_temp.finish('riceL', 18);

UPDATE sales s SET subtotal = t.sum, total = t.sum, paid_amount = t.sum
FROM (SELECT sale_id, sum(subtotal) AS sum FROM sale_items GROUP BY sale_id) t
WHERE t.sale_id = s.id AND s.receipt_no LIKE 'DEMO-%';
UPDATE purchases SET paid_amount = total WHERE notes LIKE 'DEMO-%';
-- Learn tonight's numbers now: the server recalculates a minute after it starts.
UPDATE settings SET reorder_ran_at = NULL;

COMMIT;
