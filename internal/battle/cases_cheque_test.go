package battle

import (
	"fmt"
	"testing"
)

// chequeSale sells Rs 1,000 to the customer, paid by a cheque that hasn't cleared.
func (s *shop) chequeSale(t *testing.T) (saleID, lineID, chequeID int64) {
	t.Helper()
	var r saleResult
	s.must(t, s.till1.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
		"customerId": s.customer, "items": []line{{"productId": s.prod["plain"], "quantity": "10"}},
		"payments": []pay{{"method": "cheque", "amount": "1000", "cheque": map[string]string{"bank": "BOC", "number": "100200", "date": "2026-11-30"}}}}, &r))
	s.must(t, s.db.Get(&chequeID, `SELECT id FROM cheques WHERE sale_id = $1`, r.ID))
	return r.ID, s.sale(t, s.till1, r.ID).Lines[0].ID, chequeID
}

func (s *shop) balance(t *testing.T) string {
	t.Helper()
	var b string
	s.must(t, s.db.Get(&b, `SELECT outstanding_balance FROM customers WHERE id = $1`, s.customer))
	return b
}

// RET-17: give the cheque back — the cheque is cancelled, no cash leaves the
// drawer, and the customer owes nothing.
func TestRET17_GiveTheChequeBack(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	saleID, lineID, chequeID := s.chequeSale(t)
	var res struct{ CashRefund string }
	s.must(t, s.till1.Do("POST", fmt.Sprintf("/api/v1/till/sales/%d/return", saleID), map[string]any{"chequeBack": true,
		"lines": []map[string]any{{"saleItemId": lineID, "quantity": "10"}}}, &res))
	if res.CashRefund != "0.00" {
		t.Errorf("cash refunded %s, want 0.00", res.CashRefund)
	}
	var st string
	s.must(t, s.db.Get(&st, `SELECT status FROM cheques WHERE id = $1`, chequeID))
	if st != "cancelled" {
		t.Errorf("cheque %s, want cancelled", st)
	}
	if b := s.balance(t); b != "0.00" {
		t.Errorf("customer owes %s, want 0.00", b)
	}
	s.audit(t)
}

// RET-17: give the cheque back on part of the sale — the cheque goes back, and
// what they keep goes on their account.
func TestRET17_GiveTheChequeBackOnPart(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	saleID, lineID, _ := s.chequeSale(t)
	var res struct{ CashRefund string }
	s.must(t, s.till1.Do("POST", fmt.Sprintf("/api/v1/till/sales/%d/return", saleID), map[string]any{"chequeBack": true,
		"lines": []map[string]any{{"saleItemId": lineID, "quantity": "4"}}}, &res))
	if res.CashRefund != "0.00" {
		t.Errorf("cash refunded %s, want 0.00", res.CashRefund)
	}
	if b := s.balance(t); b != "600.00" {
		t.Errorf("customer owes %s, want 600.00 for the 6 they kept", b)
	}
	s.audit(t)
}

// RET-17: refund cash and keep the cheque — if it bounces later, its amount is
// owed on the customer's account.
func TestRET17_CashNowChequeBouncesLater(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	saleID, lineID, chequeID := s.chequeSale(t)
	if cash := s.giveBack(t, s.till1, saleID, lineID, "10"); cash.StringFixed(2) != "1000.00" {
		t.Errorf("cash refunded %s, want 1000.00", cash.StringFixed(2))
	}
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/cheques/%d/return", chequeID), map[string]string{"status": "bounced"}, nil))
	if b := s.balance(t); b != "1000.00" {
		t.Errorf("customer owes %s after the bounce, want 1000.00", b)
	}
	s.audit(t)
}
