-- +goose Up
-- Sales person: prepares carts and sends them to the cashier, never handles money.
ALTER TABLE users ADD COLUMN sends_to_cashier boolean NOT NULL DEFAULT false;
-- A held sale sent to the cashier (a ticket): any cashier may open it.
ALTER TABLE held_sales
    ADD COLUMN to_cashier boolean NOT NULL DEFAULT false,
    ADD COLUMN ticket_no integer,                                  -- random, unique among the tickets waiting
    ADD COLUMN for_cashier bigint REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN assigned_at timestamptz;                            -- it moves on if not opened in time
-- Who prepared a sale that came from a ticket.
ALTER TABLE sales ADD COLUMN prepared_by bigint REFERENCES users(id);

-- +goose Down
ALTER TABLE sales DROP COLUMN prepared_by;
ALTER TABLE held_sales DROP COLUMN to_cashier, DROP COLUMN ticket_no, DROP COLUMN for_cashier, DROP COLUMN assigned_at;
ALTER TABLE users DROP COLUMN sends_to_cashier;
