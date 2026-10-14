package products

import (
	"context"

	"github.com/jmoiron/sqlx"
)

// mergeHistory are the rows that only point at a product: they move as they are.
var mergeHistory = []struct{ table, column string }{
	{"sale_items", "product_id"},
	{"sale_return_items", "product_id"},
	{"purchase_items", "product_id"},
	{"purchase_return_items", "product_id"},
	{"stock_movements", "product_id"},
	{"stock_batches", "product_id"},
	{"loss_recoveries", "product_id"},
	{"warranty_units", "product_id"},
	{"conversion_runs", "from_product_id"},
	{"conversion_runs", "to_product_id"},
	{"expenses", "service_product_id"},
}

// mergeSetup collapses doubles in the set-up tables first (the kept product's row
// wins), then moves the rest. Order matters: each DELETE removes what the UPDATE
// after it would turn into a duplicate or a self-reference.
var mergeSetup = []string{
	// Menu cards: one entry per group.
	`DELETE FROM product_group_items d USING product_group_items k
	  WHERE d.product_id = $1 AND k.product_id = $2 AND k.group_id = d.group_id`,
	`UPDATE product_group_items SET product_id = $2 WHERE product_id = $1`,
	// Recipes, own side: a line naming the other product would become itself.
	`DELETE FROM product_recipes WHERE (product_id = $1 AND component_product_id = $2) OR (product_id = $2 AND component_product_id = $1)`,
	`DELETE FROM product_recipes d USING product_recipes k
	  WHERE d.product_id = $1 AND k.product_id = $2 AND k.component_product_id = d.component_product_id`,
	`UPDATE product_recipes SET product_id = $2 WHERE product_id = $1`,
	// Recipes, component side: a recipe using both keeps its line for the kept one.
	`DELETE FROM product_recipes d USING product_recipes k
	  WHERE d.component_product_id = $1 AND k.component_product_id = $2 AND k.product_id = d.product_id`,
	`UPDATE product_recipes SET component_product_id = $2 WHERE component_product_id = $1`,
	`DELETE FROM product_recipe_costs d USING product_recipe_costs k
	  WHERE d.product_id = $1 AND k.product_id = $2 AND k.label = d.label`,
	`UPDATE product_recipe_costs SET product_id = $2 WHERE product_id = $1`,
	// Conversions: one between the two is meaningless; a clash keeps the kept one's.
	`DELETE FROM product_conversions WHERE (from_product_id = $1 AND to_product_id = $2) OR (from_product_id = $2 AND to_product_id = $1)`,
	`DELETE FROM product_conversions d USING product_conversions k
	  WHERE d.from_product_id = $1 AND k.from_product_id = $2 AND k.to_product_id = d.to_product_id`,
	`DELETE FROM product_conversions d USING product_conversions k
	  WHERE d.to_product_id = $1 AND k.to_product_id = $2 AND k.from_product_id = d.from_product_id`,
	`UPDATE product_conversions SET from_product_id = $2 WHERE from_product_id = $1`,
	`UPDATE product_conversions SET to_product_id = $2 WHERE to_product_id = $1`,
}

// mergeTables moves everything core owns from one product to the other, then
// retires the duplicate. Runs inside Merge's transaction.
func mergeTables(ctx context.Context, tx *sqlx.Tx, from, into int64) error {
	// Barcodes: the main one is cleared first so it can become an extra (one code, one product).
	var old struct {
		Barcode   *string `db:"barcode"`
		ScannedAt *string `db:"barcode_scanned_at"`
	}
	if err := tx.GetContext(ctx, &old, `SELECT barcode, barcode_scanned_at::text FROM products WHERE id = $1`, from); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE products SET barcode = NULL WHERE id = $1`, from); err != nil {
		return err
	}
	if old.Barcode != nil {
		if _, err := tx.ExecContext(ctx, `INSERT INTO product_barcodes (product_id, barcode, source, last_scanned_at)
			VALUES ($1, $2, 'merge', $3::timestamptz)`, into, *old.Barcode, old.ScannedAt); err != nil {
			return err
		}
	}
	// Packs: one the kept product already has (same name) takes the duplicate's
	// codes; the rest move across as they are.
	for _, q := range []string{
		`UPDATE product_barcodes b SET pack_id = k.id FROM product_packs f JOIN product_packs k
		   ON k.product_id = $2 AND lower(k.name) = lower(f.name)
		 WHERE b.pack_id = f.id AND f.product_id = $1`,
		`DELETE FROM product_packs f USING product_packs k
		 WHERE f.product_id = $1 AND k.product_id = $2 AND lower(k.name) = lower(f.name)`,
		`UPDATE product_packs SET product_id = $2 WHERE product_id = $1`,
	} {
		if _, err := tx.ExecContext(ctx, q, from, into); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE product_barcodes SET product_id = $2, source = 'merge' WHERE product_id = $1`, from, into); err != nil {
		return err
	}
	// Stock: the kept product's count takes the duplicate's (lots move with history below).
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO stock (product_id, quantity)
		SELECT $2, COALESCE((SELECT quantity FROM stock WHERE product_id = $1), 0)
		ON CONFLICT (product_id) DO UPDATE SET quantity = stock.quantity + EXCLUDED.quantity, last_updated = now()`, from, into); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM stock WHERE product_id = $1`, from); err != nil {
		return err
	}
	for _, h := range mergeHistory {
		if _, err := tx.ExecContext(ctx, `UPDATE `+h.table+` SET `+h.column+` = $2 WHERE `+h.column+` = $1`, from, into); err != nil {
			return err
		}
	}
	for _, q := range mergeSetup {
		if _, err := tx.ExecContext(ctx, q, from, into); err != nil {
			return err
		}
	}
	_, err := tx.ExecContext(ctx, `UPDATE products SET is_active = false, needs_review = false, merged_into_id = $2 WHERE id = $1`, from, into)
	return err
}
