package battle

import (
	"context"
	"fmt"
	"testing"
)

// Area 2 cases, one per rule (docs/rules/money.md). Rules already pinned by
// older tests name them there (internal/api/*_test.go).

// rows counts the money rows, so a refused move can be shown to write nothing.
func (s *shop) rows(t testing.TB) int {
	var n int
	s.must(t, s.db.GetContext(context.Background(), &n, `SELECT
		(SELECT count(*) FROM cash_movements) + (SELECT count(*) FROM locker_ledger) +
		(SELECT count(*) FROM money_receipts) + (SELECT count(*) FROM cash_register)`))
	return n
}

// refused asserts the call fails and writes nothing.
func (s *shop) refused(t testing.TB, what string, call func() error) {
	t.Helper()
	before := s.rows(t)
	if err := call(); err == nil {
		t.Errorf("%s: went through", what)
	}
	if after := s.rows(t); after != before {
		t.Errorf("%s: refused but wrote %d rows", what, after-before)
	}
}

func (s *shop) expected(t testing.TB, c *client) string {
	var st struct{ Summary struct{ Expected string } }
	s.must(t, c.Do("GET", "/api/v1/till/drawer", nil, &st))
	return st.Summary.Expected
}

// DRAW-5: opening twice opens one session.
func TestDRAW5_OpenTwiceOpensOne(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	s.refused(t, "second open", func() error {
		return s.till1.Do("POST", "/api/v1/till/drawer/open", map[string]any{"breakdown": map[string]int{"1000": 1}}, nil)
	})
	s.audit(t)
}

// DRAW-8: Close for them books the money as the cashier's own close and records who.
func TestDRAW8_CloseForThem(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	safe := s.locker(t, "Safe", "safe", "0")
	s.open(t, s.till1)
	s.sell(t, s.till1, "0", []line{{"productId": s.prod["plain"], "quantity": "1"}}, cash("100"))
	var sess int64
	s.must(t, s.db.GetContext(context.Background(), &sess, `SELECT id FROM cash_register WHERE closed_at IS NULL`))
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/tills/%d/close", sess), map[string]any{"breakdown": map[string]int{"5100": 1}, "destLockerId": safe}, nil))
	var got struct {
		By, Owner, Locker int64
		Diff              string
	}
	s.must(t, s.db.GetContext(context.Background(), &got, `SELECT r.closed_by AS by, r.difference AS diff,
		(SELECT created_by FROM locker_ledger WHERE counter_till_session = r.id) AS owner,
		(SELECT locker_id FROM locker_ledger WHERE counter_till_session = r.id) AS locker
		FROM cash_register r WHERE r.id = $1`, sess))
	if got.By != s.userID(t, "0770000001") || got.Owner != s.userID(t, "0770000002") || got.Locker != safe || got.Diff != "0.00" {
		t.Errorf("close for them: %+v", got)
	}
	s.refused(t, "closing it again", func() error {
		return s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/tills/%d/close", sess), map[string]any{"breakdown": map[string]int{}}, nil)
	})
	s.audit(t)
}

// DRAW-12: with untracked cash off, cash leaving a till names a locker; an empty close needn't.
func TestDRAW12_UntrackedOffNeedsALocker(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.must(t, s.admin.Do("PUT", "/api/v1/admin/settings/till-cash", map[string]any{"allowUntracked": false}, nil))
	s.open(t, s.till1)
	s.refused(t, "withdraw into nowhere", func() error {
		return s.till1.Do("POST", "/api/v1/till/drawer/withdraw", map[string]any{"amount": "10", "reason": "x"}, nil)
	})
	s.refused(t, "close into nowhere", func() error {
		return s.till1.Do("POST", "/api/v1/till/drawer/close", map[string]any{"breakdown": map[string]int{"1000": 5}}, nil)
	})
	safe := s.locker(t, "Safe", "safe", "0")
	s.must(t, s.till1.Do("POST", "/api/v1/till/drawer/withdraw", map[string]any{"amount": "5000", "reason": "all", "lockerId": safe}, nil))
	s.must(t, s.till1.Do("POST", "/api/v1/till/drawer/close", map[string]any{"breakdown": map[string]int{}}, nil))
	s.audit(t)
}

// DRAW-13 / LOCK-11: a till's cash never goes to or comes from a bank, a
// card-machine account or Cheques in hand — on every till path.
func TestDRAW13_TillCashOnlyFromCashPlaces(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	var cheque int64
	s.must(t, s.db.GetContext(context.Background(), &cheque, `SELECT id FROM lockers WHERE kind = 'cheque'`))
	bank := s.locker(t, "Bank", "bank", "10000")
	for name, id := range map[string]int64{"bank": bank, "card": s.cardAcct, "cheques": cheque} {
		s.refused(t, "open from "+name, func() error {
			return s.till1.Do("POST", "/api/v1/till/drawer/open", map[string]any{"breakdown": map[string]int{"100": 1}, "sourceLockerId": id}, nil)
		})
	}
	s.open(t, s.till1)
	for name, id := range map[string]int64{"bank": bank, "card": s.cardAcct, "cheques": cheque} {
		for _, path := range []string{"withdraw", "deposit"} {
			s.refused(t, path+" "+name, func() error {
				return s.till1.Do("POST", "/api/v1/till/drawer/"+path, map[string]any{"amount": "10", "reason": "x", "lockerId": id}, nil)
			})
		}
		s.refused(t, "close into "+name, func() error {
			return s.till1.Do("POST", "/api/v1/till/drawer/close", map[string]any{"breakdown": map[string]int{"1000": 5}, "destLockerId": id}, nil)
		})
	}
	// LOCK-11: card takings go to a card account, online ones to a bank.
	s.refused(t, "card sale into a bank", func() error {
		return s.till1.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
			"items": []line{{"productId": s.prod["plain"], "quantity": "1"}}, "payments": []pay{{"method": "card", "amount": "100", "accountId": bank}}}, nil)
	})
	s.refused(t, "online sale into a card account", func() error {
		return s.till1.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
			"items": []line{{"productId": s.prod["plain"], "quantity": "1"}}, "payments": []pay{{"method": "online", "amount": "100", "accountId": s.cardAcct}}}, nil)
	})
	s.audit(t)
}

// DRAW-17: a counted drawer adds up to the amount recorded.
func TestDRAW17_CountAddsUp(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.must(t, s.till1.Do("POST", "/api/v1/till/drawer/open", map[string]any{"breakdown": map[string]int{"1000": 2, "20": 3, "0.50": 4}}, nil))
	if got := s.expected(t, s.till1); got != "2062.00" {
		t.Errorf("opened at %s, want 2062.00", got)
	}
	s.audit(t)
}

// LOCK-9: Cheques in hand never changes by hand.
func TestLOCK9_ChequesInHandNotByHand(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	var cheque int64
	s.must(t, s.db.GetContext(context.Background(), &cheque, `SELECT id FROM lockers WHERE kind = 'cheque'`))
	safe := s.locker(t, "Safe", "safe", "1000")
	s.refused(t, "transfer in", func() error {
		return s.admin.Do("POST", "/api/v1/admin/lockers/transfer", map[string]any{"fromId": safe, "toId": cheque, "amount": "10"}, nil)
	})
	for _, typ := range []string{"adjust_up", "interest", "bank_charge"} {
		s.refused(t, typ, func() error {
			return s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/lockers/%d/adjust", cheque), map[string]any{"type": typ, "amount": "10", "note": "x"}, nil)
		})
	}
	s.refused(t, "expense from it", func() error {
		return s.admin.Do("POST", "/api/v1/admin/expenses", map[string]any{"category": "Tea", "amount": "10", "source": fmt.Sprintf("locker:%d", cheque)}, nil)
	})
	s.audit(t)
}

// LOCK-10: an opening balance is negative only where allowed.
func TestLOCK10_NegativeOpeningOnlyWhereAllowed(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.refused(t, "negative opening", func() error {
		return s.admin.Do("POST", "/api/v1/admin/lockers", map[string]any{"name": "Safe", "kind": "safe", "openingBalance": "-5"}, nil)
	})
	s.must(t, s.admin.Do("POST", "/api/v1/admin/lockers", map[string]any{"name": "Loan", "kind": "bank", "openingBalance": "-5", "overdraft": true}, nil))
	s.audit(t)
}
