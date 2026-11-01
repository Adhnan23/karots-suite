-- +goose Up
-- What a supplier credited for a loss and no bill has taken yet: it pays their
-- next delivery, as an advance or a return credit does.
ALTER TABLE loss_recoveries ADD COLUMN credit_unapplied numeric(14,2) NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE loss_recoveries DROP COLUMN credit_unapplied;
