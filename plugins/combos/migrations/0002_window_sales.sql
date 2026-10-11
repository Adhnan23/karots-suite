-- +goose Up
-- Suite M7.3: optional dates a combo runs between, and one row per combo per
-- sale for the sold counts and the Combos report. name is a snapshot so a
-- deleted combo's history stays readable.
ALTER TABLE combos ADD COLUMN starts_on DATE, ADD COLUMN ends_on DATE;

CREATE TABLE combo_sales (
    id         BIGSERIAL PRIMARY KEY,
    combo_id   BIGINT REFERENCES combos(id) ON DELETE SET NULL,
    name       TEXT NOT NULL,
    sale_id    BIGINT NOT NULL,
    sets       NUMERIC(14,4) NOT NULL,
    net        NUMERIC(14,2) NOT NULL,
    discount   NUMERIC(14,2) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (sale_id, combo_id)
);
CREATE INDEX idx_combo_sales_combo ON combo_sales(combo_id);

-- +goose Down
DROP TABLE combo_sales;
ALTER TABLE combos DROP COLUMN starts_on, DROP COLUMN ends_on;
