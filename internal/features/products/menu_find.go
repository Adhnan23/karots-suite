package products

import (
	"context"
	"strings"
)

// FindForMenu finds the product a cashier-menu sheet line names (MENU-5): by
// any of its barcodes (a pack's too), then by its exact name with case and
// spacing ignored — never by id, so a menu made in one shop fits another.
// Disabled products count (MENU-13: they stay in the menu). A non-empty reason
// means "skip this line"; err is only for the database failing.
func (r *Repository) FindForMenu(ctx context.Context, barcode, name string) (int64, string, error) {
	const none = "no product with this barcode or name"
	if b := strings.TrimSpace(barcode); b != "" {
		var ids []int64
		if err := r.db.SelectContext(ctx, &ids, `SELECT p.id FROM products p WHERE `+barcodeIs("$1")+` ORDER BY p.id LIMIT 1`, b); err != nil {
			return 0, "", err
		}
		if len(ids) == 1 {
			return ids[0], "", nil
		}
	}
	n := strings.Join(strings.Fields(name), " ")
	if n == "" {
		return 0, none, nil
	}
	var ids []int64
	if err := r.db.SelectContext(ctx, &ids, `
		SELECT id FROM products
		WHERE lower(regexp_replace(btrim(name), '\s+', ' ', 'g')) = lower($1)
		ORDER BY id LIMIT 2`, n); err != nil {
		return 0, "", err
	}
	switch len(ids) {
	case 0:
		return 0, none, nil
	case 1:
		return ids[0], "", nil
	}
	return 0, "two products have this name", nil
}
