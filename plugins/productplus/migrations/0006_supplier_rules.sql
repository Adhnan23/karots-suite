-- +goose Up
-- Supplier by field (PPF-10…13): one text / dropdown field whose values each name
-- a supplier for products with no own supplier (SUP-12). value_key is the value
-- trimmed and lower-cased, so "Bosch " and "bosch" share one rule.
CREATE TABLE pp_supplier_field (
    id       SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    field_id BIGINT NOT NULL REFERENCES pp_fields(id) ON DELETE CASCADE
);
CREATE TABLE pp_supplier_map (
    value_key   TEXT PRIMARY KEY,
    supplier_id BIGINT NOT NULL REFERENCES suppliers(id)
);

-- +goose Down
DROP TABLE pp_supplier_map;
DROP TABLE pp_supplier_field;
