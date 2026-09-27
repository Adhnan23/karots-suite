-- Fixed accounts the Playwright specs log in as. The dev `-seed` command makes
-- different phones/PINs for its own demo data, so e2e gets its own two users
-- here instead. PIN for both is 1234 (bcrypt below); idempotent delete+insert.
DELETE FROM users WHERE phone IN ('0770000001', '0770000002');
INSERT INTO users (name, phone, role, pin_hash, must_change_pin)
VALUES
  ('E2E Admin', '0770000001', 'admin', '$2a$10$lukfChD6JcyG9MrSm8u1iOfnzhD53NEqpDFcdAT4uXh.iaCIifuCu', false),
  ('E2E Cashier', '0770000002', 'cashier', '$2a$10$lukfChD6JcyG9MrSm8u1iOfnzhD53NEqpDFcdAT4uXh.iaCIifuCu', false);
