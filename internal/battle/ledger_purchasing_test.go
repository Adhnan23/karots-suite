package battle

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"karots-suite/internal/ledgercheck"
)

// A clean day of every area-5 action breaks no rule; each corruption below breaks
// the rule it names (the auditor sees what it is meant to see).
func TestPurchasingRulesSeeCorruption(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	ctx := context.Background()
	safe := s.locker(t, "Safe", "safe", "100000")
	bank := s.locker(t, "Bank", "bank", "100000")
	today := time.Now().Format("2006-01-02")
	var sup, old struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/suppliers", map[string]string{"name": "Battle Wholesale"}, &sup))
	s.must(t, s.admin.Do("POST", "/api/v1/admin/suppliers", map[string]string{"name": "Old Debts Ltd", "openingBalance": "1000"}, &old))
	path := func(id int64, what string) string { return fmt.Sprintf("/api/v1/admin/suppliers/%d/%s", id, what) }

	// An order received: free units, a line discount, a bill discount, a pack line.
	var order struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/purchases", map[string]any{"supplierId": sup.ID, "lines": []map[string]any{
		{"productId": s.prod["plain"], "quantity": "10", "costPrice": "50"},
		{"productId": s.prod["pack"], "quantity": "2", "costPrice": "120", "packSize": "12"}}}, &order))
	var got struct{ PurchaseID int64 }
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/purchases/%d/receive", order.ID), map[string]any{"supplierId": sup.ID,
		"discount": "10", "discountType": "fixed", "lines": []map[string]any{
			{"productId": s.prod["plain"], "ordered": "10", "quantity": "10", "freeQty": "2", "costPrice": "50", "discount": "5", "discountType": "percent"},
			{"productId": s.prod["pack"], "ordered": "24", "quantity": "2", "costPrice": "120", "packSize": "12"}}}, &got))
	// A delivery with no order, paid on delivery in cash from the safe.
	var walk struct{ PurchaseID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/purchases/receive", map[string]any{"supplierId": sup.ID, "lines": []map[string]any{
		{"productId": s.prod["plain"], "quantity": "4", "costPrice": "55"}},
		"pay": map[string]string{"amount": "100", "method": "cash", "source": fmt.Sprintf("locker:%d", safe)}}, &walk))
	// Paid oldest first by card from the bank; the old debt paid; then an advance.
	s.must(t, s.admin.Do("POST", path(sup.ID, "payment"), map[string]string{"amount": "300", "method": "card", "source": fmt.Sprintf("locker:%d", bank)}, nil))
	s.must(t, s.admin.Do("POST", path(old.ID, "payment"), map[string]string{"amount": "400", "method": "cash", "mode": "old", "source": fmt.Sprintf("locker:%d", safe)}, nil))
	s.must(t, s.admin.Do("POST", path(old.ID, "payment"), map[string]string{"amount": "900", "method": "cash", "source": fmt.Sprintf("locker:%d", safe)}, nil))
	// A delivery settled out of that advance, then a discount on what's left.
	var settled struct{ PurchaseID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/purchases/receive", map[string]any{"supplierId": old.ID, "lines": []map[string]any{
		{"productId": s.prod["plain"], "quantity": "5", "costPrice": "60"}}}, &settled))
	s.must(t, s.admin.Do("POST", path(sup.ID, "payment"), map[string]string{"amount": "20", "method": "discount"}, nil))
	// Goods sent back against the first delivery, and stock that came on none.
	var lot, opening int64
	s.must(t, s.db.GetContext(ctx, &lot, `SELECT b.id FROM stock_batches b JOIN purchase_items pi ON pi.id = b.purchase_item_id
		WHERE pi.purchase_id = $1 AND b.product_id = $2`, got.PurchaseID, s.prod["plain"]))
	s.must(t, s.db.GetContext(ctx, &opening, `SELECT id FROM stock_batches WHERE product_id = $1 AND purchase_item_id IS NULL ORDER BY id LIMIT 1`, s.prod["odd"]))
	s.must(t, s.admin.Do("POST", "/api/v1/admin/purchase-returns", map[string]any{"supplierId": sup.ID, "purchaseId": got.PurchaseID,
		"lines": []map[string]any{{"productId": s.prod["plain"], "quantity": "3", "batchId": lot}}}, nil))
	s.must(t, s.admin.Do("POST", "/api/v1/admin/purchase-returns", map[string]any{"supplierId": sup.ID,
		"lines": []map[string]any{{"productId": s.prod["odd"], "quantity": "500", "batchId": opening}}}, nil))
	// Sent back beyond what's owed: a credit, part of it refunded in cash.
	s.must(t, s.admin.Do("POST", path(sup.ID, "refund"), map[string]string{"amount": "5", "method": "cash", "source": fmt.Sprintf("locker:%d", safe)}, nil))
	// Two cheque payments to the old-debt supplier's next bill: one clears, one bounces.
	s.must(t, s.admin.Do("POST", "/api/v1/admin/purchases/receive", map[string]any{"supplierId": old.ID, "lines": []map[string]any{
		{"productId": s.prod["plain"], "quantity": "20", "costPrice": "60"}}}, nil))
	for _, no := range []string{"500100", "500101"} {
		s.must(t, s.admin.Do("POST", path(old.ID, "payment"), map[string]any{"amount": "200", "method": "cheque",
			"cheques": []map[string]any{{"lockerId": bank, "number": no, "amount": "200", "date": today}}}, nil))
	}
	var clears, bounces int64
	s.must(t, s.db.GetContext(ctx, &clears, `SELECT id FROM cheques WHERE number = '500100'`))
	s.must(t, s.db.GetContext(ctx, &bounces, `SELECT id FROM cheques WHERE number = '500101'`))
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/cheques/%d/clear", clears), map[string]string{}, nil))
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/cheques/%d/return", bounces), map[string]string{"status": "bounced"}, nil))
	s.must(t, s.admin.Do("POST", path(old.ID, "opening"), map[string]string{"opening": "0"}, nil))
	s.audit(t)

	cases := []struct{ rule, breakIt, mend string }{
		{"SUP-1", `UPDATE suppliers SET outstanding_balance = outstanding_balance + 0.01 WHERE id = SUP`,
			`UPDATE suppliers SET outstanding_balance = outstanding_balance - 0.01 WHERE id = SUP`},
		{"SUP-1", `UPDATE supplier_payments SET amount = amount + 1 WHERE method = 'card'`,
			`UPDATE supplier_payments SET amount = amount - 1 WHERE method = 'card'`},
		{"SUP-2", `UPDATE suppliers SET opening_unlinked = opening_balance + 1 WHERE id = OLD`,
			`UPDATE suppliers SET opening_unlinked = opening_balance - 1 WHERE id = OLD`},
		{"SUP-2", `UPDATE suppliers SET opening_balance = opening_balance - 5000, outstanding_balance = outstanding_balance - 5000 WHERE id = SUP`,
			`UPDATE suppliers SET opening_balance = opening_balance + 5000, outstanding_balance = outstanding_balance + 5000 WHERE id = SUP`},
		{"PUR-1", `UPDATE purchases SET total = total + 1 WHERE id = GOT`, `UPDATE purchases SET total = total - 1 WHERE id = GOT`},
		{"PUR-1", `UPDATE purchase_items SET subtotal = subtotal + 1 WHERE id = (SELECT MIN(id) FROM purchase_items WHERE purchase_id = GOT)`,
			`UPDATE purchase_items SET subtotal = subtotal - 1 WHERE id = (SELECT MIN(id) FROM purchase_items WHERE purchase_id = GOT)`},
		{"PUR-10", `UPDATE purchase_return_items SET subtotal = subtotal + 1 WHERE id = (SELECT MIN(id) FROM purchase_return_items)`,
			`UPDATE purchase_return_items SET subtotal = subtotal - 1 WHERE id = (SELECT MIN(id) FROM purchase_return_items)`},
		{"PUR-10", `UPDATE purchase_return_allocations SET amount = amount + 100000 WHERE id = (SELECT MIN(id) FROM purchase_return_allocations)`,
			`UPDATE purchase_return_allocations SET amount = amount - 100000 WHERE id = (SELECT MIN(id) FROM purchase_return_allocations)`},
		{"PUR-11", `UPDATE purchases SET status = 'received' WHERE id = GOT`, `UPDATE purchases SET status = 'STATUS' WHERE id = GOT`},
		{"PUR-11", `UPDATE purchases SET credited_amount = credited_amount + total WHERE id = WALK`,
			`UPDATE purchases SET credited_amount = credited_amount - total WHERE id = WALK`},
		{"PUR-12", `UPDATE supplier_payment_allocations SET reopened = reopened + 1 WHERE purchase_id = WALK`,
			`UPDATE supplier_payment_allocations SET reopened = reopened - 1 WHERE purchase_id = WALK`},
		{"PUR-13", `UPDATE stock_batches SET purchase_item_id = NULL WHERE id = LOT`,
			`UPDATE stock_batches SET purchase_item_id = (SELECT MIN(id) FROM purchase_items WHERE purchase_id = GOT) WHERE id = LOT`},
		{"PUR-13", `UPDATE stock_movements SET quantity = quantity - 1 WHERE type = 'purchase' AND reference_id = WALK`,
			`UPDATE stock_movements SET quantity = quantity + 1 WHERE type = 'purchase' AND reference_id = WALK`},
		{"SUP-3", `UPDATE money_receipts SET amount = amount + 1 WHERE ref_kind = 'supplier_payment' AND ref_id = (SELECT id FROM supplier_payments WHERE method = 'card')`,
			`UPDATE money_receipts SET amount = amount - 1 WHERE ref_kind = 'supplier_payment' AND ref_id = (SELECT id FROM supplier_payments WHERE method = 'card')`},
		{"SUP-3", `UPDATE cheques SET supplier_payment_id = NULL WHERE number = '500100'`,
			`UPDATE cheques SET supplier_payment_id = (SELECT MIN(id) FROM supplier_payments WHERE method = 'cheque') WHERE number = '500100'`},
		{"SUP-4", `UPDATE supplier_payment_allocations SET amount = amount + 100000 WHERE purchase_id = WALK`,
			`UPDATE supplier_payment_allocations SET amount = amount - 100000 WHERE purchase_id = WALK`},
	}
	var status string
	s.must(t, s.db.GetContext(ctx, &status, `SELECT status FROM purchases WHERE id = $1`, got.PurchaseID))
	ids := strings.NewReplacer("SUP", fmt.Sprint(sup.ID), "OLD", fmt.Sprint(old.ID), "GOT", fmt.Sprint(got.PurchaseID),
		"WALK", fmt.Sprint(walk.PurchaseID), "LOT", fmt.Sprint(lot), "STATUS", status)
	for _, c := range cases {
		if _, err := s.db.ExecContext(ctx, ids.Replace(c.breakIt)); err != nil {
			t.Fatalf("%s: %v", c.rule, err)
		}
		vs, err := ledgercheck.Run(ctx, s.db)
		if _, merr := s.db.ExecContext(ctx, ids.Replace(c.mend)); merr != nil {
			t.Fatalf("%s mend: %v", c.rule, merr)
		}
		if err != nil {
			t.Fatal(err)
		}
		var rules []string
		for _, v := range vs {
			rules = append(rules, v.Rule)
		}
		if !slices.Contains(rules, c.rule) {
			t.Errorf("%s: corruption not seen (got %v)", c.rule, rules)
		}
	}
	s.audit(t)
}
