-- +goose Up
-- A returned serial-tracked unit (a phone brought back) goes back on the shelf and must
-- sell again under the same serial: its warranty row is kept as history with status
-- 'returned', and a serial is unique only among units that haven't come back (RET-10).
ALTER TABLE warranty_units DROP CONSTRAINT warranty_units_serial_no_key;
CREATE UNIQUE INDEX warranty_units_serial_live ON warranty_units (serial_no) WHERE status <> 'returned';

-- +goose Down
DROP INDEX warranty_units_serial_live;
ALTER TABLE warranty_units ADD CONSTRAINT warranty_units_serial_no_key UNIQUE (serial_no);
