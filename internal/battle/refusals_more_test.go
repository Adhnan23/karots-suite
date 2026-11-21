package battle

import (
	"fmt"
	"testing"
)

// More refusals that change nothing (refusals_test.go): money places, people,
// buying, stock, cheques, losses and warranty.
func TestRefusalsBackOffice(t *testing.T) {
	t.Parallel()
	s := newSweepShop(t)
	a := s.admin
	safe := s.locker(t, "Safe", "safe", "1000")
	bank := s.locker(t, "Bank", "bank", "1000")
	src := fmt.Sprintf("locker:%d", safe)
	cust := s.newCustomer(t, "0712000905", "100000", "500")
	sup := s.newSup(t, "Refuse Ltd", "0")
	other := s.newSup(t, "Other Ltd", "0")
	got := s.mustDeliver(t, sup, []sl{{"productId": s.prod["plain"], "quantity": "10", "costPrice": "40"}}, nil)
	theirs := s.mustDeliver(t, other, []sl{{"productId": s.prod["plain"], "quantity": "1", "costPrice": "40"}}, nil)
	lot, _, _ := s.lotOf(t, got, s.prod["plain"])
	p, plain := s.prod["plain"], s.prod["plain"]
	order := func(line map[string]any, extra map[string]any) map[string]any {
		l := map[string]any{"productId": plain, "quantity": "1", "costPrice": "10"}
		for k, v := range line {
			l[k] = v
		}
		b := map[string]any{"supplierId": sup, "lines": []map[string]any{l}}
		for k, v := range extra {
			b[k] = v
		}
		return b
	}
	cheque := func(c map[string]string) map[string]any {
		ch := map[string]string{"bank": "BOC", "number": "1", "amount": "100", "date": "2026-12-01"}
		for k, v := range c {
			ch[k] = v
		}
		return map[string]any{"method": "cheque", "cheques": []map[string]string{ch}}
	}
	cpay := fmt.Sprintf("/api/v1/admin/customers/%d/payment", cust)
	s.refuseAll(t, []bad{
		// money places
		{"an expense of 0", a, "POST", "/api/v1/admin/expenses", map[string]any{"category": "Tea", "amount": "0", "source": src}},
		{"an expense dated 31/12", a, "POST", "/api/v1/admin/expenses", map[string]any{"category": "Tea", "amount": "5", "source": src, "date": "31/12/2026"}},
		{"void an expense that doesn't exist", a, "POST", "/api/v1/admin/expenses/999999/void", map[string]any{"dest": src}},
		{"move 0 between lockers", a, "POST", "/api/v1/admin/lockers/transfer", map[string]any{"fromId": safe, "toId": bank, "amount": "0"}},
		{"move to a locker that doesn't exist", a, "POST", "/api/v1/admin/lockers/transfer", map[string]any{"fromId": safe, "toId": 999999, "amount": "5"}},
		{"adjust a locker that doesn't exist", a, "POST", "/api/v1/admin/lockers/999999/adjust", map[string]any{"type": "interest", "amount": "5", "note": "x"}},
		{"a locker with no name", a, "POST", "/api/v1/admin/lockers", map[string]any{"name": " ", "kind": "safe", "openingBalance": "0"}},
		{"a locker opening of abc", a, "POST", "/api/v1/admin/lockers", map[string]any{"name": "X", "kind": "safe", "openingBalance": "abc"}},
		{"settle more than the card account holds", a, "POST", "/api/v1/admin/lockers/settle-card", map[string]any{"cardId": s.cardAcct, "bankId": bank, "gross": "999999", "fee": "0"}},
		// customers
		{"a customer payment of 0", a, "POST", cpay, map[string]any{"amount": "0", "method": "cash", "source": src}},
		{"a payment from a customer that doesn't exist", a, "POST", "/api/v1/admin/customers/999999/payment", map[string]any{"amount": "5", "method": "cash", "source": src}},
		{"a customer refund of 0", s.till1, "POST", fmt.Sprintf("/api/v1/till/customers/%d/refund", cust), map[string]any{"amount": "0"}},
		{"a refund to a customer that doesn't exist", s.till1, "POST", "/api/v1/till/customers/999999/refund", map[string]any{"amount": "5"}},
		{"a customer with no name", a, "POST", "/api/v1/admin/customers", map[string]any{"name": " ", "phone": "0712000906", "creditLimit": "0"}},
		{"a credit limit of -5", a, "POST", "/api/v1/admin/customers", map[string]any{"name": "Neg", "phone": "0712000907", "creditLimit": "-5"}},
		{"a customer opening of abc", a, "POST", fmt.Sprintf("/api/v1/admin/customers/%d/opening", cust), map[string]string{"opening": "abc"}},
		{"a cheque with no number", a, "POST", cpay, cheque(map[string]string{"number": ""})},
		{"a cheque of 0", a, "POST", cpay, cheque(map[string]string{"amount": "0"})},
		{"a cheque with no date", a, "POST", cpay, cheque(map[string]string{"date": ""})},
		// suppliers
		{"a supplier refund by cheque", a, "POST", fmt.Sprintf("/api/v1/admin/suppliers/%d/refund", sup), map[string]string{"amount": "5", "method": "cheque", "source": src}},
		{"a supplier refund of 0", a, "POST", fmt.Sprintf("/api/v1/admin/suppliers/%d/refund", sup), map[string]string{"amount": "0", "method": "cash", "source": src}},
		{"a refund from a supplier that doesn't exist", a, "POST", "/api/v1/admin/suppliers/999999/refund", map[string]string{"amount": "5", "method": "cash", "source": src}},
		{"pay a supplier that doesn't exist", a, "POST", "/api/v1/admin/suppliers/999999/payment", map[string]any{"amount": "5", "method": "cash", "source": src}},
		{"pay old debt of -5", a, "POST", fmt.Sprintf("/api/v1/admin/suppliers/%d/payment", sup), map[string]any{"oldDebt": "-5", "method": "cash", "source": src}},
		{"pay a bill that doesn't exist", a, "POST", fmt.Sprintf("/api/v1/admin/suppliers/%d/payment", sup), map[string]any{"method": "cash", "source": src,
			"allocations": []map[string]any{{"purchaseId": 999999, "amount": "5"}}}},
		{"pay another supplier's bill", a, "POST", fmt.Sprintf("/api/v1/admin/suppliers/%d/payment", sup), map[string]any{"method": "cash", "source": src,
			"allocations": []map[string]any{{"purchaseId": theirs, "amount": "5"}}}},
		// buying
		{"an order with nothing on it", a, "POST", "/api/v1/admin/purchases", map[string]any{"supplierId": sup, "lines": []map[string]any{}}},
		{"an order of a product that doesn't exist", a, "POST", "/api/v1/admin/purchases", order(map[string]any{"productId": 999999}, nil)},
		{"a delivery due 31/12", a, "POST", "/api/v1/admin/purchases/receive", order(nil, map[string]any{"dueDate": "31/12/2026"})},
		{"a delivery discount of -5", a, "POST", "/api/v1/admin/purchases/receive", order(nil, map[string]any{"discount": "-5"})},
		{"a delivery line of 0", a, "POST", "/api/v1/admin/purchases/receive", order(map[string]any{"quantity": "0"}, nil)},
		{"a free quantity of abc", a, "POST", "/api/v1/admin/purchases/receive", order(map[string]any{"freeQty": "abc"}, nil)},
		{"a cost of abc", a, "POST", "/api/v1/admin/purchases/receive", order(map[string]any{"costPrice": "abc"}, nil)},
		{"a selling price of abc", a, "POST", "/api/v1/admin/purchases/receive", order(map[string]any{"sellingPrice": "abc"}, nil)},
		{"an expiry of 31/12", a, "POST", "/api/v1/admin/purchases/receive", order(map[string]any{"expiryDate": "31/12/2027"}, nil)},
		{"a line discount of abc", a, "POST", "/api/v1/admin/purchases/receive", order(map[string]any{"discount": "abc"}, nil)},
		{"send back 0", a, "POST", "/api/v1/admin/purchase-returns", map[string]any{"supplierId": sup, "purchaseId": got,
			"lines": []map[string]any{{"productId": p, "quantity": "0", "batchId": lot}}}},
		{"send back more than the lot holds", a, "POST", "/api/v1/admin/purchase-returns", map[string]any{"supplierId": sup, "purchaseId": got,
			"lines": []map[string]any{{"productId": p, "quantity": "999", "batchId": lot}}}},
		{"send back from a lot that doesn't exist", a, "POST", "/api/v1/admin/purchase-returns", map[string]any{"supplierId": sup,
			"lines": []map[string]any{{"productId": p, "quantity": "1", "batchId": 999999}}}},
		// stock
		{"count to -1", a, "POST", "/api/v1/admin/stock/adjust", map[string]any{"productId": p, "newQuantity": "-1"}},
		{"count a product that doesn't exist", a, "POST", "/api/v1/admin/stock/adjust", map[string]any{"productId": 999999, "newQuantity": "1"}},
		{"count a lot that doesn't exist", a, "POST", "/api/v1/admin/stock/adjust", map[string]any{"productId": p, "newQuantity": "1", "batchId": 999999}},
		{"write off 0", a, "POST", "/api/v1/admin/stock/damage", map[string]any{"productId": p, "quantity": "0", "reason": "damage"}},
		{"write off from a lot that doesn't exist", a, "POST", "/api/v1/admin/stock/damage", map[string]any{"productId": p, "quantity": "1", "reason": "damage", "batchId": 999999}},
		{"a lot price of -1", a, "POST", fmt.Sprintf("/api/v1/admin/stock/batches/%d/price", lot), map[string]string{"sellingPrice": "-1"}},
		{"a lot cost of -1", a, "POST", fmt.Sprintf("/api/v1/admin/stock/lots/%d/add", p), map[string]string{"qty": "1", "cost": "-1"}},
		{"put 0 out for sale", a, "POST", fmt.Sprintf("/api/v1/admin/stock/%d/to-shelf", p), map[string]string{"quantity": "0"}},
		{"a conversion into itself", a, "POST", "/api/v1/admin/conversions", map[string]any{"fromProductId": p, "toProductId": p, "ratio": "10"}},
		{"a conversion ratio of 0", a, "POST", "/api/v1/admin/conversions", map[string]any{"fromProductId": p, "toProductId": s.prod["odd"], "ratio": "0"}},
		{"run a conversion that doesn't exist", a, "POST", "/api/v1/admin/conversions/999999/run", map[string]string{"quantity": "1"}},
		// cheques, losses, warranty
		{"clear a cheque that doesn't exist", a, "POST", "/api/v1/admin/cheques/999999/clear", map[string]string{}},
		{"a cheque back as cleared", a, "POST", "/api/v1/admin/cheques/999999/return", map[string]string{"status": "cleared"}},
		{"settle a loss that doesn't exist", a, "POST", "/api/v1/admin/recovery", map[string]any{"sourceType": "damage", "sourceId": 999999, "outcome": "written_off"}},
		{"undo a recovery that doesn't exist", a, "DELETE", "/api/v1/admin/recovery/999999", nil},
		{"swap a unit that doesn't exist", s.till1, "POST", "/api/v1/till/warranty/replace", map[string]any{"unitId": 999999, "newSerial": "X", "reason": "dead"}},
	})
}
