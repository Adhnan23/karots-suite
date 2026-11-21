-- +goose Up
-- The sale lines that collected a job (its tagged lines on the pickup sale), so
-- the report counts a job's own lines: two jobs on one sale, or a part also sold
-- loose beside them, never count each other's (REP-8).
CREATE TABLE repair_pickup_lines (
    job_id       BIGINT NOT NULL REFERENCES repair_jobs(id) ON DELETE CASCADE,
    sale_item_id BIGINT NOT NULL UNIQUE
);
CREATE INDEX repair_pickup_lines_job_idx ON repair_pickup_lines (job_id);

-- Jobs collected before: their pickup sale's labour lines and lines of their parts,
-- as the report counted them.
INSERT INTO repair_pickup_lines (job_id, sale_item_id)
SELECT DISTINCT ON (si.id) j.id, si.id
FROM repair_jobs j JOIN sale_items si ON si.sale_id = j.sale_id
WHERE j.status = 'collected'
  AND (si.product_id = (SELECT labour_product_id FROM repair_config WHERE id = 1)
       OR si.product_id IN (SELECT product_id FROM repair_parts WHERE job_id = j.id))
ORDER BY si.id, j.id;

-- +goose Down
DROP TABLE repair_pickup_lines;
