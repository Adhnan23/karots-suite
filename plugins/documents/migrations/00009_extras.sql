-- +goose Up

-- A service's extras: Urgent, Hole punch, Corner rounding. Priced per job or per
-- piece, and may take stock (a lanyard, a key tag): qty of product_id per job / piece.
-- At the till each ticked extra is a line of its own beside the job.
CREATE TABLE doc_extra (
  id         BIGSERIAL     PRIMARY KEY,
  service_id BIGINT        NOT NULL REFERENCES doc_service (id) ON DELETE CASCADE,
  name       VARCHAR(40)   NOT NULL,
  price      DECIMAL(12,2) NOT NULL DEFAULT 0,
  per_piece  BOOLEAN       NOT NULL DEFAULT false,
  product_id BIGINT        REFERENCES products (id) ON DELETE RESTRICT,
  qty        DECIMAL(12,4) NOT NULL DEFAULT 1,
  sort       INT           NOT NULL DEFAULT 0
);
CREATE INDEX idx_doc_extra_service ON doc_extra (service_id);

-- +goose Down
DROP TABLE doc_extra;
