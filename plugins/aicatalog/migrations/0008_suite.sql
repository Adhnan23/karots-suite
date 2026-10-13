-- +goose Up
-- The suite has no bulk entry (no staging table) and no markup of its own (core
-- price fields do cost ⇄ markup ⇄ selling). Paste mode is the default: a fresh
-- install, or any shop without a key, starts in it. A restore replaces the row,
-- so a restored shop keeps its own choice.
DROP TABLE aicatalog_mock_products;
ALTER TABLE aicatalog_settings DROP COLUMN default_markup;
ALTER TABLE aicatalog_settings ALTER COLUMN manual_mode SET DEFAULT true;
UPDATE aicatalog_settings SET manual_mode = true WHERE api_key = '';

-- +goose Down
ALTER TABLE aicatalog_settings ALTER COLUMN manual_mode SET DEFAULT false;
ALTER TABLE aicatalog_settings ADD COLUMN default_markup NUMERIC(6,3) NOT NULL DEFAULT 1.400;
CREATE TABLE aicatalog_mock_products (
    id                 BIGSERIAL PRIMARY KEY,
    name               TEXT NOT NULL,
    detail             TEXT NOT NULL DEFAULT '',
    qty                TEXT NOT NULL DEFAULT '',
    barcode            TEXT NOT NULL DEFAULT '',
    cost_price         TEXT NOT NULL DEFAULT '',
    selling_price      TEXT NOT NULL DEFAULT '',
    category           TEXT NOT NULL DEFAULT '',
    specs              TEXT NOT NULL DEFAULT '',
    explanation        TEXT NOT NULL DEFAULT '',
    confident          BOOLEAN NOT NULL DEFAULT false,
    status             TEXT NOT NULL DEFAULT 'draft',
    created_product_id BIGINT,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
