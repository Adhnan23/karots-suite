-- +goose Up
-- The suite keeps a repair's money in core: a deposit is a core customer payment
-- (the customer's advance) and paying the repairer is a core expense. These link
-- them to the job. karots-pos deposits stay in repair_payments (kind deposit /
-- refund / applied) until those jobs are collected or cancelled.
CREATE TABLE repair_deposits (
    job_id     BIGINT NOT NULL REFERENCES repair_jobs(id) ON DELETE CASCADE,
    payment_id BIGINT NOT NULL UNIQUE
);
CREATE INDEX repair_deposits_job_idx ON repair_deposits (job_id);

CREATE TABLE repair_payouts (
    job_id     BIGINT NOT NULL REFERENCES repair_jobs(id) ON DELETE CASCADE,
    expense_id BIGINT NOT NULL UNIQUE
);
CREATE INDEX repair_payouts_job_idx ON repair_payouts (job_id);

-- +goose Down
DROP TABLE repair_payouts;
DROP TABLE repair_deposits;
