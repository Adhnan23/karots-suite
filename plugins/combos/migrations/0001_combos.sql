-- +goose Up
CREATE TABLE combos (
    id             BIGSERIAL PRIMARY KEY,
    name           TEXT NOT NULL,
    discount_type  TEXT NOT NULL DEFAULT 'percent',  -- percent | fixed
    discount_value NUMERIC NOT NULL DEFAULT 0,
    is_active      BOOLEAN NOT NULL DEFAULT true,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE combo_items (
    id         BIGSERIAL PRIMARY KEY,
    combo_id   BIGINT NOT NULL REFERENCES combos(id) ON DELETE CASCADE,
    product_id BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    qty        NUMERIC(14,4) NOT NULL DEFAULT 1
);
CREATE INDEX idx_combo_items_combo ON combo_items(combo_id);
CREATE INDEX idx_combo_items_product ON combo_items(product_id);

-- +goose Down
DROP TABLE combo_items;
DROP TABLE combos;
