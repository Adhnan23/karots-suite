-- +goose Up
-- The suite rings every reload, package, bill, wallet deposit and cash out as a
-- cart line on the normal Pay, and books each device's float (and the carrier's
-- commission) as it happens. A device's balance is its last old karots-pos close
-- plus every float row since, so restored karots-pos data reads as it did there.

-- Commission per carrier and kind: a percent and/or a fixed amount per move,
-- and the default service charge the till offers for that kind.
CREATE TABLE recharge_rates (
    carrier_id BIGINT NOT NULL REFERENCES recharge_carriers(id),
    kind       TEXT NOT NULL CHECK (kind IN ('reload','billpay','deposit','cashout','wallet_in')),
    percent    NUMERIC(7,3)  NOT NULL DEFAULT 0,
    fixed      NUMERIC(12,2) NOT NULL DEFAULT 0,
    charge     NUMERIC(12,2) NOT NULL DEFAULT 0,
    PRIMARY KEY (carrier_id, kind)
);

-- Packages sold from a carrier's float: price to the customer, float it uses
-- (the difference is the commission). One added at the till keeps its cashier
-- until an admin reviews it.
CREATE TABLE recharge_packages (
    id               BIGSERIAL PRIMARY KEY,
    carrier_id       BIGINT NOT NULL REFERENCES recharge_carriers(id),
    name             TEXT NOT NULL,
    price            NUMERIC(12,2) NOT NULL,
    float_cost       NUMERIC(12,2) NOT NULL,
    is_active        BOOLEAN NOT NULL DEFAULT TRUE,
    sort             INT NOT NULL DEFAULT 0,
    added_at_till_by BIGINT,
    reviewed_at      TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX recharge_packages_name_idx ON recharge_packages (carrier_id, lower(name)) WHERE is_active;

-- A float count: what the phone showed against what the suite expected; the
-- difference is booked as a 'count' float row.
CREATE TABLE recharge_counts (
    id         BIGSERIAL PRIMARY KEY,
    device_id  BIGINT NOT NULL REFERENCES recharge_devices(id),
    counted    NUMERIC(12,2) NOT NULL,
    expected   NUMERIC(12,2) NOT NULL,
    session_id BIGINT,
    created_by BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX recharge_counts_device_idx ON recharge_counts (device_id, created_at);

-- Float rows no longer belong to a till session; they link to the sale line,
-- the move a commission belongs to, the package and the count.
ALTER TABLE recharge_transactions ALTER COLUMN session_id DROP NOT NULL;
ALTER TABLE recharge_transactions ADD COLUMN sale_item_id    BIGINT;
ALTER TABLE recharge_transactions ADD COLUMN parent_id       BIGINT REFERENCES recharge_transactions(id);
ALTER TABLE recharge_transactions ADD COLUMN package_id      BIGINT;
ALTER TABLE recharge_transactions ADD COLUMN count_id        BIGINT;
ALTER TABLE recharge_transactions ADD COLUMN customer_number TEXT;
ALTER TABLE recharge_transactions ADD COLUMN bill_type       TEXT;
CREATE INDEX idx_recharge_tx_device ON recharge_transactions (device_id, created_at);
CREATE INDEX idx_recharge_tx_sale_item ON recharge_transactions (sale_item_id) WHERE sale_item_id IS NOT NULL;
ALTER TABLE recharge_transactions DROP CONSTRAINT recharge_transactions_type_check;
ALTER TABLE recharge_transactions ADD CONSTRAINT recharge_transactions_type_check
    CHECK (type IN ('deposit','withdrawal','billpay','topup','wallet_in','reload','refill','opening',
                    'reversal','reversal_lost','commission','count','cashout'));

-- A bill paid from (or cash out into) a bank locker: the money is core's locker
-- ledger; this row keeps the bill detail and its sale line.
ALTER TABLE recharge_bill_tx ADD COLUMN sale_id      BIGINT;
ALTER TABLE recharge_bill_tx ADD COLUMN sale_item_id BIGINT;
ALTER TABLE recharge_bill_tx ADD COLUMN returned     NUMERIC(12,2) NOT NULL DEFAULT 0;

-- The plugin's own products (Bill payment, Wallet deposit, Cash out, Service
-- charge), made once and kept by id.
CREATE TABLE recharge_config (
    key        TEXT PRIMARY KEY,
    product_id BIGINT NOT NULL
);

-- +goose Down
DROP TABLE recharge_config;
ALTER TABLE recharge_bill_tx DROP COLUMN returned;
ALTER TABLE recharge_bill_tx DROP COLUMN sale_item_id;
ALTER TABLE recharge_bill_tx DROP COLUMN sale_id;
DELETE FROM recharge_transactions WHERE type IN ('commission','count','cashout');
ALTER TABLE recharge_transactions DROP CONSTRAINT recharge_transactions_type_check;
ALTER TABLE recharge_transactions ADD CONSTRAINT recharge_transactions_type_check
    CHECK (type IN ('deposit','withdrawal','billpay','topup','wallet_in','reload','refill','opening','reversal','reversal_lost'));
DROP INDEX idx_recharge_tx_sale_item;
DROP INDEX idx_recharge_tx_device;
ALTER TABLE recharge_transactions DROP COLUMN bill_type;
ALTER TABLE recharge_transactions DROP COLUMN customer_number;
ALTER TABLE recharge_transactions DROP COLUMN count_id;
ALTER TABLE recharge_transactions DROP COLUMN package_id;
ALTER TABLE recharge_transactions DROP COLUMN parent_id;
ALTER TABLE recharge_transactions DROP COLUMN sale_item_id;
DROP TABLE recharge_counts;
DROP TABLE recharge_packages;
DROP TABLE recharge_rates;
