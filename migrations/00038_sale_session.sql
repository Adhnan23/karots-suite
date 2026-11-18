-- +goose Up
-- Each sale belongs to the till session that took its cash (DRAW-6, DRAW-7): the
-- session open when it was rung, or — when its cashier had none open (the page
-- still up after the till was closed for them) — the next one they open. The
-- drawer's expected cash and the Z report count a session's sales by this link,
-- not by time, so a sale paid while the till is being closed is never missed.
ALTER TABLE sales ADD COLUMN session_id bigint REFERENCES cash_register(id);
UPDATE sales s SET session_id = r.id FROM cash_register r
 WHERE r.user_id = s.cashier_id AND s.created_at >= r.opened_at AND (r.closed_at IS NULL OR s.created_at < r.closed_at);
CREATE INDEX idx_sales_session ON sales (session_id);

-- +goose Down
DROP INDEX idx_sales_session;
ALTER TABLE sales DROP COLUMN session_id;
