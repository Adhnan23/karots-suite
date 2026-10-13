-- +goose Up
-- Extra barcodes: the same item under a second code (new packaging, another
-- supplier, or a merged duplicate's old code). products.barcode stays the main
-- one (labels print it). last_scanned_at / barcode_scanned_at tell the owner
-- which codes are still in use; nothing is ever dropped automatically.
CREATE TABLE product_barcodes (
    id              BIGSERIAL PRIMARY KEY,
    product_id      BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    barcode         VARCHAR(50) NOT NULL UNIQUE,
    source          VARCHAR(20) NOT NULL,   -- intake | receive | form | merge | import
    added_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_scanned_at TIMESTAMPTZ
);
CREATE INDEX idx_product_barcodes_product ON product_barcodes (product_id);

ALTER TABLE products
    ADD COLUMN barcode_scanned_at TIMESTAMPTZ,
    -- Set on the duplicate a merge retired: it stays as a row so anything still
    -- pointing at it resolves, but every list hides it.
    ADD COLUMN merged_into_id BIGINT REFERENCES products(id);

-- One code, one product: a main barcode may not equal another product's extra,
-- and an extra may not equal any main barcode but its own product's. The advisory
-- lock on the code makes two concurrent writers of the same code queue up, so the
-- check can't be raced past.
-- +goose StatementBegin
CREATE FUNCTION barcode_one_product() RETURNS trigger AS $$
BEGIN
    IF NEW.barcode IS NULL THEN
        RETURN NEW;
    END IF;
    PERFORM pg_advisory_xact_lock(hashtext('barcode:' || NEW.barcode));
    IF TG_TABLE_NAME = 'products' THEN
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

CREATE TRIGGER products_barcode_one_product
    BEFORE INSERT OR UPDATE OF barcode ON products
    FOR EACH ROW EXECUTE FUNCTION barcode_one_product();
CREATE TRIGGER product_barcodes_one_product
    BEFORE INSERT OR UPDATE OF barcode ON product_barcodes
    FOR EACH ROW EXECUTE FUNCTION barcode_one_product();

-- +goose Down
DROP TRIGGER product_barcodes_one_product ON product_barcodes;
DROP TRIGGER products_barcode_one_product ON products;
DROP FUNCTION barcode_one_product();
ALTER TABLE products DROP COLUMN merged_into_id, DROP COLUMN barcode_scanned_at;
DROP TABLE product_barcodes;
