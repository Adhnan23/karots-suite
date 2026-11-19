package battle

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"karots-suite/internal/ledgercheck"
)

// A clean day of every area-3 action breaks no rule; each corruption below breaks
// the rule it names (the auditor sees what it is meant to see).
func TestCustomerRulesSeeCorruption(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	ctx := context.Background()
	safe := s.locker(t, "Safe", "safe", "10000")
	var old struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/customers", map[string]string{"name": "Old Debt", "phone": "0712000001",
		"creditLimit": "5000", "openingBalance": "1000"}, &old))
	s.open(t, s.till1)
	cu := s.customer

	// On account, change kept to the account, part of the credit sale back.
	var credit saleResult
	s.must(t, s.till1.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
		"customerId": cu, "items": []line{{"productId": s.prod["plain"], "quantity": "3"}}, "payments": []pay{{"method": "credit", "amount": "300"}}}, &credit))
	s.must(t, s.till1.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
		"customerId": cu, "items": []line{{"productId": s.prod["plain"], "quantity": "2"}}, "payments": []pay{cash("250")},
		"changeGiven": "30", "onAccountRounding": true}, nil))
	s.giveBack(t, s.till1, credit.ID, s.sale(t, s.till1, credit.ID).Lines[0].ID, "1")
	// Payments: cash at the counter, card in the back office, a cheque, old debt first, more than owed.
	s.must(t, s.till1.Do("POST", fmt.Sprintf("/api/v1/till/customers/%d/payment", cu), map[string]any{"amount": "100", "method": "cash"}, nil))
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/customers/%d/payment", cu), map[string]any{"amount": "50", "method": "card",
		"source": fmt.Sprintf("locker:%d", s.cardAcct)}, nil))
	s.must(t, s.till1.Do("POST", fmt.Sprintf("/api/v1/till/customers/%d/payment", cu), map[string]any{"method": "cheque",
		"cheques": []map[string]string{{"bank": "BOC", "number": "700100", "amount": "40", "date": "2026-12-01"}}}, nil))
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/customers/%d/payment", old.ID), map[string]any{"amount": "300", "method": "cash",
		"source": fmt.Sprintf("locker:%d", safe), "oldDebtFirst": true}, nil))
	s.must(t, s.till1.Do("POST", fmt.Sprintf("/api/v1/till/customers/%d/payment", cu), map[string]any{"amount": "500", "method": "cash"}, nil))
	s.must(t, s.till1.Do("POST", fmt.Sprintf("/api/v1/till/customers/%d/refund", cu), map[string]any{"amount": "10"}, nil))
	var cheque int64
	s.must(t, s.db.GetContext(ctx, &cheque, `SELECT id FROM cheques WHERE number = '700100'`))
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/cheques/%d/return", cheque), map[string]string{"status": "bounced"}, nil))
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/customers/%d/opening", old.ID), map[string]string{"opening": "900"}, nil))
	s.audit(t)

	cases := []struct{ rule, breakIt, mend string }{
		{"CUST-1", `UPDATE customers SET outstanding_balance = outstanding_balance + 0.01 WHERE id = CU`,
			`UPDATE customers SET outstanding_balance = outstanding_balance - 0.01 WHERE id = CU`},
		{"CUST-1", `UPDATE sales SET rounding_to_account = rounding_to_account + 1 WHERE rounding_to_account > 0`,
			`UPDATE sales SET rounding_to_account = rounding_to_account - 1 WHERE rounding_to_account > 0`},
		{"CUST-3", `INSERT INTO customers (name, phone) VALUES ('Twin', '071-111 1111')`, `DELETE FROM customers WHERE name = 'Twin'`},
		{"CUST-5", `UPDATE money_receipts SET amount = amount - 1 WHERE ref_kind = 'customer_payment' AND ref_id = (SELECT id FROM customer_payments WHERE method = 'card')`,
			`UPDATE money_receipts SET amount = amount + 1 WHERE ref_kind = 'customer_payment' AND ref_id = (SELECT id FROM customer_payments WHERE method = 'card')`},
		{"CUST-5", `UPDATE money_receipts SET ref_id = -ref_id WHERE ref_kind = 'customer_payment' AND kind = 'refund'`,
			`UPDATE money_receipts SET ref_id = -ref_id WHERE ref_kind = 'customer_payment' AND kind = 'refund'`},
		{"CUST-5", `UPDATE cheques SET customer_payment_id = NULL WHERE number = '700100'`,
			`UPDATE cheques SET customer_payment_id = (SELECT id FROM customer_payments WHERE method = 'cheque') WHERE number = '700100'`},
		{"CUST-6", `UPDATE customer_payments SET balance_after = balance_after + 1 WHERE method = 'card'`,
			`UPDATE customer_payments SET balance_after = balance_after - 1 WHERE method = 'card'`},
		{"CUST-9", `UPDATE customers SET opening_unlinked = opening_unlinked + 1000 WHERE id = OLD`,
			`UPDATE customers SET opening_unlinked = opening_unlinked - 1000 WHERE id = OLD`},
		{"CUST-15", `UPDATE customers SET opening_unlinked = opening_unlinked + 1 WHERE id = OLD`,
			`UPDATE customers SET opening_unlinked = opening_unlinked - 1 WHERE id = OLD`},
	}
	ids := strings.NewReplacer("CU", fmt.Sprint(cu), "OLD", fmt.Sprint(old.ID))
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
		var got []string
		for _, v := range vs {
			got = append(got, v.Rule)
		}
		if !slices.Contains(got, c.rule) {
			t.Errorf("%s: corruption not seen (got %v)", c.rule, got)
		}
	}
	s.audit(t)
}
