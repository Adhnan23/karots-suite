-- +goose Up
-- Packs: a product counted in its smallest unit (a tablet, a stick, an egg) can
-- also be sold, bought and scanned by the pack (Card ×12, Packet ×20, Tray ×30).
-- Stock, batches and sale lines stay in units; a pack only sets a price for a
-- full pack and how quantities are entered. selling_price 0 = size × the unit
-- price; wholesale_price 0 = the pack's selling price. set_at_till marks a size
-- a cashier typed in at the till (Items to review until someone checks it).
CREATE TABLE product_packs (
    id              BIGSERIAL PRIMARY KEY,
    product_id      BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    name            VARCHAR(30) NOT NULL,
    size            NUMERIC(12,3) NOT NULL CHECK (size > 1),
    selling_price   NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (selling_price >= 0),
    wholesale_price NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (wholesale_price >= 0),
    set_at_till     BOOLEAN NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX product_packs_name ON product_packs (product_id, lower(name));

-- A pack's own code (the manufacturer's barcode on the card). NULL = an extra
-- code of the product itself.
ALTER TABLE product_barcodes ADD COLUMN pack_id BIGINT REFERENCES product_packs(id) ON DELETE CASCADE;

-- The split a line was sold as ("1 Card + 2"); NULL when no pack fitted.
ALTER TABLE sale_items ADD COLUMN pack_label TEXT;

-- A product's main barcode may take over one of its own EXTRA codes, never one
-- of its packs' codes (that would silently turn a card's code into a tablet's).
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION barcode_one_product() RETURNS trigger AS $$
BEGIN
    IF NEW.barcode IS NULL THEN
        RETURN NEW;
    END IF;
    PERFORM pg_advisory_xact_lock(hashtext('barcode:' || NEW.barcode));
    IF TG_TABLE_NAME = 'products' THEN
        DELETE FROM product_barcodes WHERE barcode = NEW.barcode AND product_id = NEW.id AND pack_id IS NULL;
        IF EXISTS (SELECT 1 FROM product_barcodes WHERE barcode = NEW.barcode) THEN
            RAISE EXCEPTION 'barcode % is already used', NEW.barcode USING ERRCODE = 'unique_violation';
        END IF;
    ELSIF EXISTS (SELECT 1 FROM products WHERE barcode = NEW.barcode) THEN
        RAISE EXCEPTION 'barcode % is already used', NEW.barcode USING ERRCODE = 'unique_violation';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION barcode_one_product() RETURNS trigger AS $$
BEGIN
    IF NEW.barcode IS NULL THEN
        RETURN NEW;
    END IF;
    PERFORM pg_advisory_xact_lock(hashtext('barcode:' || NEW.barcode));
    IF TG_TABLE_NAME = 'products' THEN
        DELETE FROM product_barcodes WHERE barcode = NEW.barcode AND product_id = NEW.id;
        IF EXISTS (SELECT 1 FROM product_barcodes WHERE barcode = NEW.barcode) THEN
            RAISE EXCEPTION 'barcode % is already used', NEW.barcode USING ERRCODE = 'unique_violation';
        END IF;
    ELSIF EXISTS (SELECT 1 FROM products WHERE barcode = NEW.barcode) THEN
        RAISE EXCEPTION 'barcode % is already used', NEW.barcode USING ERRCODE = 'unique_violation';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
ALTER TABLE sale_items DROP COLUMN pack_label;
ALTER TABLE product_barcodes DROP COLUMN pack_id;
DROP TABLE product_packs;
