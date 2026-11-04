-- +goose Up
-- Two sends at once must not draw the same number: a cashier could hand over the wrong cart.
CREATE UNIQUE INDEX held_sales_ticket_no ON held_sales (ticket_no) WHERE to_cashier;

-- +goose Down
DROP INDEX held_sales_ticket_no;
