-- +goose Up
-- An action other than a sale is claimed ('running') before it runs: a repeat that
-- finds the claim never runs it again, even if the PC stopped between the work and
-- its record (PKT-4). A sale's own one-time ref (sales.client_ref) does that for sales.
ALTER TABLE pocket_actions DROP CONSTRAINT pocket_actions_status_check;
ALTER TABLE pocket_actions ADD CONSTRAINT pocket_actions_status_check CHECK (status IN ('applied','skipped','failed','running'));

-- +goose Down
DELETE FROM pocket_actions WHERE status = 'running';
ALTER TABLE pocket_actions DROP CONSTRAINT pocket_actions_status_check;
ALTER TABLE pocket_actions ADD CONSTRAINT pocket_actions_status_check CHECK (status IN ('applied','skipped','failed'));
