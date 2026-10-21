-- +goose Up
-- A product whose reorder level was never set (0) learns one too: Auto from
-- nothing typed. A level someone typed stays Fixed (00008).
UPDATE products SET reorder_auto = true, reorder_start = 0
WHERE NOT reorder_auto AND reorder_level = 0 AND NOT is_service;

-- +goose Down
SELECT 1; -- which products were switched isn't recorded; they stay Auto
