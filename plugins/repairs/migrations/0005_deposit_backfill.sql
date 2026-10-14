-- +goose Up
-- Re-label deposits that earlier repair collections recorded as 'wallet'. Only
-- payments on a repair's own collection sale; compared as text so this runs
-- even where 'wallet' was never added to the enum.
UPDATE payments p SET method = 'deposit'
FROM repair_jobs j
WHERE j.sale_id = p.sale_id AND p.method::text = 'wallet';

-- +goose Down
SELECT 1;
