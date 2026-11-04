-- +goose Up
-- A cashier PC's print helper: it reports that PC's printers and takes their jobs.
CREATE TABLE print_machines (
    id              bigserial PRIMARY KEY,
    token_hash      text NOT NULL UNIQUE,
    name            text NOT NULL DEFAULT '',
    ip              text NOT NULL DEFAULT '',
    printers        jsonb NOT NULL DEFAULT '[]',
    approved        boolean NOT NULL DEFAULT false,
    receipt_printer text NOT NULL DEFAULT '',
    label_printer   text NOT NULL DEFAULT '',
    last_seen       timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE print_machines;
