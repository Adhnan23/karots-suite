-- +goose Up
-- Hints: what the chatbot said about a product, searchable. old_names keeps every
-- name a job or clean-up replaced, so the code staff know still finds the product.
ALTER TABLE aicatalog_items ADD COLUMN old_names TEXT NOT NULL DEFAULT '';
CREATE INDEX aicatalog_items_hint_trgm ON aicatalog_items
  USING gin ((specs || ' ' || user_explanation || ' ' || old_names || ' ' || raw_query) gin_trgm_ops);

-- A job's answer per product: it leaves that job's list, and comes back only from
-- the Skipped / no answer filter or a redo.
CREATE TABLE aicatalog_job_items (
  job        TEXT   NOT NULL,
  product_id BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  state      TEXT   NOT NULL CHECK (state IN ('applied', 'skipped', 'no_answer')),
  category_id BIGINT, -- where it was when answered: moved, its category's fields are new to it
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (job, product_id)
);
CREATE INDEX aicatalog_job_items_product ON aicatalog_job_items (product_id);

-- +goose Down
DROP TABLE aicatalog_job_items;
DROP INDEX aicatalog_items_hint_trgm;
ALTER TABLE aicatalog_items DROP COLUMN old_names;
