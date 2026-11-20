-- +goose Up
-- A return remembers the till session its cashier had open (ZR-2): the Z report
-- shows the refunds given in that session, so a closed Z never changes. NULL =
-- given with no till open (an admin's return paid from a locker). Old returns
-- take the session of theirs open at the time, when there was one.
ALTER TABLE sale_returns ADD COLUMN session_id bigint REFERENCES cash_register(id) ON DELETE SET NULL;
UPDATE sale_returns r SET session_id = (
    SELECT c.id FROM cash_register c
    WHERE c.user_id = r.created_by AND c.opened_at <= r.created_at
      AND (c.closed_at IS NULL OR c.closed_at >= r.created_at)
    ORDER BY c.id DESC LIMIT 1);
CREATE INDEX idx_sale_returns_session ON sale_returns (session_id);

-- +goose Down
ALTER TABLE sale_returns DROP COLUMN session_id;
