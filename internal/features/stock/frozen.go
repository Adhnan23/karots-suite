package stock

import "context"

// Frozen reports whether a product's stock is frozen (its sales don't touch stock).
func (r *Repository) Frozen(ctx context.Context, productID int64) (bool, error) {
	var on bool
	err := r.q.GetContext(ctx, &on, `SELECT stock_frozen FROM products WHERE id = $1`, productID)
	return on, err
}
