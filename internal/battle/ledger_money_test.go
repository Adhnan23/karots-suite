package battle

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"karots-suite/internal/ledgercheck"
)

// A clean day of every area-2 move breaks no rule; each corruption below breaks
// exactly the rule it names (the auditor sees what it is meant to see).
func TestMoneyRulesSeeCorruption(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	ctx := context.Background()
	safe, bank := s.locker(t, "Safe", "safe", "10000"), s.locker(t, "Bank", "bank", "0")
	till1 := s.userID(t, "0770000002")

	s.must(t, s.till1.Do("POST", "/api/v1/till/drawer/open", map[string]any{"breakdown": map[string]int{"1000": 5}, "sourceLockerId": safe}, nil))
	s.sell(t, s.till1, "0", []line{{"productId": s.prod["plain"], "quantity": "2"}}, cash("500"))
	cardSale := s.sell(t, s.till1, "0", []line{{"productId": s.prod["plain"], "quantity": "3"}},
		pay{"method": "card", "amount": "300", "accountId": s.cardAcct})
	s.must(t, s.till1.Do("POST", "/api/v1/till/drawer/withdraw", map[string]any{"amount": "100", "reason": "to safe", "lockerId": safe}, nil))
	s.must(t, s.till1.Do("POST", "/api/v1/till/drawer/deposit", map[string]any{"amount": "50", "reason": "coins", "lockerId": safe}, nil))
	s.must(t, s.admin.Do("POST", "/api/v1/admin/expenses", map[string]any{"category": "Tea", "amount": "40", "source": fmt.Sprintf("till:%d", till1)}, nil))
	s.must(t, s.admin.Do("POST", "/api/v1/admin/expenses", map[string]any{"category": "Rent", "amount": "1500", "source": fmt.Sprintf("locker:%d", safe)}, nil))
	var voided int64
	s.must(t, s.admin.Do("POST", "/api/v1/admin/expenses", map[string]any{"category": "Wrong", "amount": "70", "source": fmt.Sprintf("locker:%d", safe)}, nil))
	s.must(t, s.db.GetContext(ctx, &voided, `SELECT id FROM expenses WHERE category = 'Wrong'`))
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/expenses/%d/void", voided), map[string]any{"dest": fmt.Sprintf("locker:%d", safe)}, nil))
	s.must(t, s.admin.Do("POST", "/api/v1/admin/lockers/transfer", map[string]any{"fromId": safe, "toId": bank, "amount": "2000"}, nil))
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/lockers/%d/adjust", bank), map[string]any{"type": "bank_charge", "amount": "25", "note": "fee"}, nil))
	s.must(t, s.admin.Do("POST", "/api/v1/admin/lockers/settle-card", map[string]any{"cardId": s.cardAcct, "bankId": bank, "gross": "300", "fee": "6"}, nil))
	var st struct{ Summary struct{ Expected string } }
	s.must(t, s.till1.Do("GET", "/api/v1/till/drawer", nil, &st))
	s.must(t, s.till1.Do("POST", "/api/v1/till/drawer/close", map[string]any{"breakdown": map[string]int{st.Summary.Expected: 1}, "destLockerId": safe}, nil))
	s.audit(t)

	cases := []struct{ rule, breakIt, mend string }{
		{"DRAW-1", `UPDATE cash_register SET expected_cash = expected_cash + 0.01, difference = difference - 0.01`,
			`UPDATE cash_register SET expected_cash = expected_cash - 0.01, difference = difference + 0.01`},
		{"DRAW-1", `UPDATE cash_register SET difference = difference + 1`, `UPDATE cash_register SET difference = difference - 1`},
		{"DRAW-1", `UPDATE cash_movements SET amount = amount - 1 WHERE type = 'expense'`, `UPDATE cash_movements SET amount = amount + 1 WHERE type = 'expense'`},
		{"DRAW-6", `UPDATE sales SET cashier_id = cashier_id + 1 WHERE id = SALE`, `UPDATE sales SET cashier_id = cashier_id - 1 WHERE id = SALE`},
		{"DRAW-9", `UPDATE cash_movements SET amount = amount + 1 WHERE type = 'opening'`, `UPDATE cash_movements SET amount = amount - 1 WHERE type = 'opening'`},
		{"DRAW-9", `UPDATE cash_movements SET type = 'sale' WHERE type = 'closing'`, `UPDATE cash_movements SET type = 'closing' WHERE type = 'sale'`},
		{"LOCK-3", `UPDATE cash_movements SET amount = amount - 1 WHERE type = 'withdrawal'`, `UPDATE cash_movements SET amount = amount + 1 WHERE type = 'withdrawal'`},
		{"LOCK-2", `UPDATE locker_ledger SET balance_delta = balance_delta + 1 WHERE counter_locker_id IS NOT NULL AND balance_delta > 0 AND note = ''`,
			`UPDATE locker_ledger SET balance_delta = balance_delta - 1 WHERE counter_locker_id IS NOT NULL AND balance_delta > 0 AND note = ''`},
		{"LOCK-6", `UPDATE locker_ledger SET kind = 'adjust' WHERE locker_id = CARD AND kind = 'bank_charge'`,
			`UPDATE locker_ledger SET kind = 'bank_charge' WHERE locker_id = CARD AND kind = 'adjust'`},
		{"LOCK-8", `UPDATE expenses SET amount = amount + 1 WHERE category = 'Bank charges'`, `UPDATE expenses SET amount = amount - 1 WHERE category = 'Bank charges'`},
		{"LOCK-12", `UPDATE locker_ledger SET balance_delta = balance_delta - 1 WHERE ref_kind = 'sale' AND ref_id = SALE`,
			`UPDATE locker_ledger SET balance_delta = balance_delta + 1 WHERE ref_kind = 'sale' AND ref_id = SALE`},
		{"EXP-1", `UPDATE expenses SET amount = amount + 1 WHERE category = 'Rent'`, `UPDATE expenses SET amount = amount - 1 WHERE category = 'Rent'`},
		{"EXP-2", `UPDATE money_receipts SET amount = amount - 1 WHERE kind = 'expense_void'`, `UPDATE money_receipts SET amount = amount + 1 WHERE kind = 'expense_void'`},
	}
	for _, c := range cases {
		ids := strings.NewReplacer("CARD", fmt.Sprint(s.cardAcct), "SALE", fmt.Sprint(cardSale.ID))
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

// locker makes a locker with an opening balance; its id.
func (s *shop) locker(t testing.TB, name, kind, opening string) int64 {
	var l struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/lockers", map[string]any{"name": name, "kind": kind, "openingBalance": opening, "cashierAccess": true}, &l))
	return l.ID
}

func (s *shop) userID(t testing.TB, phone string) int64 {
	var id int64
	s.must(t, s.db.GetContext(context.Background(), &id, `SELECT id FROM users WHERE phone = $1`, phone))
	return id
}
