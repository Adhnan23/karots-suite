-- +goose Up
-- A "card" money location is a clearing account: card-machine takings land here
-- until the acquirer settles the batch into a real bank account (net of the
-- service fee). Widen the locker-kind CHECK to allow it. The inline column CHECK
-- from 0037 is named lockers_kind_check by Postgres.
ALTER TABLE lockers DROP CONSTRAINT lockers_kind_check;
ALTER TABLE lockers ADD CONSTRAINT lockers_kind_check
	CHECK (kind IN ('safe','bank','pocket','other','card'));

-- +goose Down
-- Reverting requires no 'card' lockers to remain.
ALTER TABLE lockers DROP CONSTRAINT lockers_kind_check;
ALTER TABLE lockers ADD CONSTRAINT lockers_kind_check
	CHECK (kind IN ('safe','bank','pocket','other'));
