-- +goose Up
-- The shop's copy of the published catalogue (CTL-2): replaced or patched whole by a sync, never edited here.
CREATE TABLE catalogue_items (
    id     BIGINT PRIMARY KEY,               -- the catalogue's permanent id
    name   TEXT  NOT NULL,
    unit   TEXT  NOT NULL DEFAULT '',        -- the catalogue's unit short name (pcs, tab, cap, pkt)
    kinds  TEXT  NOT NULL DEFAULT '',
    fields JSONB NOT NULL DEFAULT '{}',      -- field name → value
    search TEXT  NOT NULL DEFAULT ''         -- lower(name + field values), for search
);
CREATE INDEX idx_catalogue_items_search ON catalogue_items USING gin (search gin_trgm_ops);

-- One row: the version the copy holds and the last check.
CREATE TABLE catalogue_state (
    id         INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    version    INT   NOT NULL DEFAULT 0,
    count      INT   NOT NULL DEFAULT 0,  -- the version's product count, from its index (CTL-2)
    fields     JSONB NOT NULL DEFAULT '[]', -- the catalogue's field list at that version
    checked_at TIMESTAMPTZ,
    synced_at  TIMESTAMPTZ,
    error      TEXT  NOT NULL DEFAULT ''
);
INSERT INTO catalogue_state DEFAULT VALUES;

-- A shop product made from a pick (CTL-7): what it took, and whether "From the catalogue" was reviewed.
CREATE TABLE catalogue_links (
    product_id BIGINT PRIMARY KEY,           -- core products(id); no FK (plugin schema versions alone)
    item_id    BIGINT NOT NULL,
    took       JSONB  NOT NULL DEFAULT '{}', -- {"name","unit","fields":{…}} as the catalogue had it
    reviewed   BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_catalogue_links_item ON catalogue_links (item_id);

-- A later catalogue change to a linked product, waiting on Items to review (CTL-8).
CREATE TABLE catalogue_reviews (
    product_id BIGINT PRIMARY KEY,
    item_id    BIGINT NOT NULL,
    changes    JSONB  NOT NULL,              -- [{"what","then","now"}]
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE catalogue_reviews;
DROP TABLE catalogue_links;
DROP TABLE catalogue_state;
DROP TABLE catalogue_items;
