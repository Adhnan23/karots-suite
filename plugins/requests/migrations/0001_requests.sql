-- +goose Up
CREATE TABLE requests (
    id             BIGSERIAL PRIMARY KEY,
    note           TEXT NOT NULL,
    customer_name  TEXT,
    customer_phone TEXT,
    status         TEXT NOT NULL DEFAULT 'open',  -- open | converted
    product_id     BIGINT,                        -- set when converted to a product
    created_by     BIGINT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_requests_status ON requests (status, created_at DESC);

-- +goose Down
DROP TABLE requests;
