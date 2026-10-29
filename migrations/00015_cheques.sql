-- +goose Up
ALTER TABLE lockers DROP CONSTRAINT lockers_kind_check;
ALTER TABLE lockers ADD CONSTRAINT lockers_kind_check CHECK (kind IN ('safe','bank','pocket','other','card','cheque'));
CREATE UNIQUE INDEX lockers_one_cheque ON lockers ((kind)) WHERE kind = 'cheque';
INSERT INTO lockers (name, kind, allow_negative, is_active, cashier_access) VALUES ('Cheques in hand', 'cheque', false, true, false);
CREATE TABLE cheques (
  id bigserial PRIMARY KEY,
  direction text NOT NULL CHECK (direction IN ('in','out')),
  bank text NOT NULL DEFAULT '',                 -- in: the customer's bank
  locker_id bigint REFERENCES lockers(id),       -- out: drawn on (required); in: where it cleared
  number text NOT NULL CHECK (number <> ''),
  amount numeric(14,2) NOT NULL CHECK (amount > 0),
  cheque_date date NOT NULL,
  note text NOT NULL DEFAULT '',
  status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','cleared','bounced','cancelled')),
  status_date date, status_by bigint REFERENCES users(id),
  customer_id bigint REFERENCES customers(id),
  supplier_id bigint REFERENCES suppliers(id),
  sale_id bigint REFERENCES sales(id), payment_id bigint REFERENCES payments(id),
  customer_payment_id bigint REFERENCES customer_payments(id),
  supplier_payment_id bigint REFERENCES supplier_payments(id),
  expense_id bigint REFERENCES expenses(id),
  created_by bigint REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  CHECK ((direction = 'in') = (customer_id IS NOT NULL)),
  CHECK (direction = 'in' OR locker_id IS NOT NULL)
);
CREATE INDEX cheques_status_date ON cheques (direction, status, cheque_date);
ALTER TABLE expenses ADD COLUMN unpaid numeric(12,2) NOT NULL DEFAULT 0;
ALTER TABLE supplier_payment_allocations ADD COLUMN reopened numeric(14,2) NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE supplier_payment_allocations DROP COLUMN reopened;
ALTER TABLE expenses DROP COLUMN unpaid;
DROP TABLE cheques;
DELETE FROM locker_ledger WHERE locker_id IN (SELECT id FROM lockers WHERE kind = 'cheque');
DELETE FROM lockers WHERE kind = 'cheque';
DROP INDEX lockers_one_cheque;
ALTER TABLE lockers DROP CONSTRAINT lockers_kind_check;
ALTER TABLE lockers ADD CONSTRAINT lockers_kind_check CHECK (kind IN ('safe','bank','pocket','other','card'));
