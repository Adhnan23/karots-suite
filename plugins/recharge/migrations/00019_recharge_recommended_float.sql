-- +goose Up
-- Per-device recommended float level: the target balance the shop tops each
-- device back up to. Feeds the core "Top-up & Reorder" readiness report, which
-- shows the shortfall (target − current) as cash needed to buy float. 0 = no
-- target set (shown but not counted).
ALTER TABLE recharge_devices ADD COLUMN recommended_float NUMERIC NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE recharge_devices DROP COLUMN recommended_float;
