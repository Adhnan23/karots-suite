package gen

import (
	"fmt"
	"slices"

	"github.com/shopspring/decimal"

	"karots-suite/internal/features/cashregister"
	"karots-suite/internal/features/sales"
	"karots-suite/internal/features/settings"
)

// openTills opens a till with the usual float for each shown login that has none,
// so the owner lands straight on Sell.
func (s *Sim) openTills() error {
	g := s.g
	for _, l := range g.s.p.Logins {
		var id int64
		if err := g.d.DB.GetContext(g.ctx, &id, `SELECT id FROM users WHERE phone = $1`, l.Phone); err != nil {
			return err
		}
		if cur, err := g.d.Svc.CashRegister.Current(g.ctx, id); err == nil && cur != nil {
			continue
		}
		if _, err := g.d.Svc.CashRegister.Open(g.ctx, id, cashregister.OpenInput{OpeningCash: "5000"}); err != nil {
			return fmt.Errorf("open a till for %s: %w", l.Name, err)
		}
	}
	return nil
}

// lowStock writes a few well-stocked products down to just under their reorder
// level, so Low stock and reorder have something to show.
func (s *Sim) lowStock() error {
	g := s.g
	var rows []struct {
		ID    int64           `db:"id"`
		Level int             `db:"reorder_level"`
		Have  decimal.Decimal `db:"have"`
	}
	if err := g.d.DB.SelectContext(g.ctx, &rows, `
		SELECT p.id, p.reorder_level, COALESCE(sum(b.qty_remaining), 0) AS have
		FROM products p JOIN stock_batches b ON b.product_id = p.id
		WHERE p.is_active AND p.reorder_level > 0
		GROUP BY p.id HAVING COALESCE(sum(b.qty_remaining), 0) > p.reorder_level + 1
		ORDER BY random() LIMIT 5`); err != nil {
		return err
	}
	for _, r := range rows {
		out := r.Have.Sub(decimal.NewFromInt(int64(max(r.Level-1, 0)))).Floor()
		if !out.IsPositive() {
			continue
		}
		if err := g.admin.Do("POST", "/api/v1/admin/stock/damage", map[string]any{"productId": r.ID, "quantity": out.String(),
			"reason": "damage", "note": "Water leak in the store room"}, nil); err != nil {
			return err
		}
	}
	return g.loadStock()
}

// creditDue puts a regular customer over their credit limit with one more bill.
func (s *Sim) creditDue() error {
	g := s.g
	var c struct {
		ID    int64           `db:"id"`
		Limit decimal.Decimal `db:"credit_limit"`
		Owed  decimal.Decimal `db:"outstanding_balance"`
	}
	if err := g.d.DB.GetContext(g.ctx, &c, `SELECT id, credit_limit, outstanding_balance FROM customers
		WHERE credit_limit > 0 ORDER BY credit_limit LIMIT 1`); err != nil {
		return err
	}
	var items []sales.ItemInput
	total := decimal.Zero
	need := c.Limit.Sub(c.Owed).Add(decimal.NewFromInt(500))
	for _, it := range g.s.items {
		if total.GreaterThan(need) {
			break
		}
		if g.stock[it.id] >= 1 {
			items = append(items, sales.ItemInput{ProductID: it.id, Quantity: "1"})
			total = total.Add(g.priceOf(it.id))
			g.stock[it.id]--
		}
	}
	if len(items) == 0 {
		return fmt.Errorf("nothing on the shelf to sell")
	}
	_, err := g.d.Svc.Sales.Create(g.ctx, sales.CreateInput{SaleType: "retail", CustomerID: &c.ID, AllowOverLimit: true, Items: items,
		Payments: []sales.PaymentInput{{Method: sales.MethodOnAccount, Amount: total.String()}}}, g.cashier().id)
	return err
}

// cycleLook moves the shop to the next palette or receipt style.
func (s *Sim) cycleLook(palette bool) error {
	g := s.g
	st, err := settings.NewRepository(g.d.DB).Get(g.ctx)
	if err != nil {
		return err
	}
	skin, style := st.Skin, st.ReceiptStyle
	next := func(list []string, cur string) string { return list[(slices.Index(list, cur)+1)%len(list)] }
	if palette {
		skin = next(palettes, skin)
	} else {
		style = next(receiptStyles, style)
	}
	return settings.NewRepository(g.d.DB).SetAppearance(g.ctx, skin, style, "", "", st.LabelStyles)
}
