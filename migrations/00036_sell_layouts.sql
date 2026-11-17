-- +goose Up
-- Sell page 2.0: the Sell page's layout per screen size (arrangement, widths, card and
-- text size), keyed "<w>x<h>" in CSS px; set by the system user in Appearance. '{}' = today's.
ALTER TABLE settings ADD COLUMN sell_layouts JSONB NOT NULL DEFAULT '{}';
-- The screen sizes the shop's tills have reported from the Sell page, with who used them,
-- so Appearance can list them (presence itself is only kept in memory).
CREATE TABLE screen_sizes (
    w         integer     NOT NULL,
    h         integer     NOT NULL,
    users     text[]      NOT NULL DEFAULT '{}',
    last_seen timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (w, h)
);

-- +goose Down
DROP TABLE screen_sizes;
ALTER TABLE settings DROP COLUMN sell_layouts;
