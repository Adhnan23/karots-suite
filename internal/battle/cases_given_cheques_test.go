package battle

import (
	"fmt"
	"testing"
	"time"
)

// Area 5 cases, one per rule (docs/rules/purchasing.md): cheques given to
// suppliers. Received cheques are pinned in cases_cheque_test.go (RET-17, TEND-8).

// payByCheque pays a supplier with one cheque drawn on bank, dated date.
func (s *shop) payByCheque(t *testing.T, sup, bank int64, amount, number, date string) (cheque int64) {
	t.Helper()
	s.must(t, s.admin.Do("POST", supPath(sup, "payment"), map[string]any{"amount": amount, "method": "cheque",
		"cheques": []map[string]any{{"lockerId": bank, "number": number, "amount": amount, "date": date}}}, nil))
	s.must(t, s.db.Get(&cheque, `SELECT id FROM cheques WHERE number = $1`, number))
	return cheque
}

func (s *shop) chequeTo(id int64, what string) string { return fmt.Sprintf("/api/v1/admin/cheques/%d/%s", id, what) }

// CHQ-1, CHQ-2: a given cheque pays the bill when written but moves no money
// until it clears — once, not before its date nor in the future.
func TestCHQ1And2GivenChequeClears(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	bank := s.locker(t, "Bank", "bank", "1000")
	sup := s.newSup(t, "Cheque Ltd", "")
	b := s.mustDeliver(t, sup, []sl{{"productId": s.prod["plain"], "quantity": "10", "costPrice": "150"}}, nil) // 1500
	today := time.Now().Format("2006-01-02")
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	now := s.payByCheque(t, sup, bank, "1200", "700001", today)
	later := s.payByCheque(t, sup, bank, "300", "700002", tomorrow)
	if bb := s.bill(t, b); bb.Status != "paid" {
		t.Errorf("bill %s once cheques are written, want paid", bb.Status)
	}
	eq(t, "bank before any clears", s.lockerBal(t, bank), "1000")
	s.mustRefuse(t, "clear before its date", s.admin.Do("POST", s.chequeTo(later, "clear"), map[string]string{}, nil))
	s.mustRefuse(t, "clear in the future", s.admin.Do("POST", s.chequeTo(later, "clear"), map[string]string{"date": tomorrow}, nil))
	s.must(t, s.admin.Do("POST", s.chequeTo(now, "clear"), map[string]string{}, nil))
	eq(t, "bank after one clears (below zero is allowed)", s.lockerBal(t, bank), "-200")
	s.mustRefuse(t, "clear twice", s.admin.Do("POST", s.chequeTo(now, "clear"), map[string]string{}, nil))
	eq(t, "bank after a second clear", s.lockerBal(t, bank), "-200")
	s.audit(t)
}

// CHQ-3: a bounced given cheque: the supplier is owed it again, the bills it
// paid reopen (newest allocation first); bounced or cancelled once only.
func TestCHQ3GivenChequeBounces(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	bank := s.locker(t, "Bank", "bank", "10000")
	sup := s.newSup(t, "Bounce Ltd", "")
	b1 := s.mustDeliver(t, sup, []sl{{"productId": s.prod["plain"], "quantity": "1", "costPrice": "400"}}, nil)
	b2 := s.mustDeliver(t, sup, []sl{{"productId": s.prod["plain"], "quantity": "1", "costPrice": "600"}}, nil)
	today := time.Now().Format("2006-01-02")
	c := s.payByCheque(t, sup, bank, "700", "710001", today) // b1 400, b2 300
	rows := s.rows(t)
	s.must(t, s.admin.Do("POST", s.chequeTo(c, "return"), map[string]string{"status": "bounced"}, nil))
	eq(t, "b1 paid", s.bill(t, b1).Paid, "0")
	eq(t, "b2 paid", s.bill(t, b2).Paid, "0")
	if got := s.supBal(t, sup); got != "1000.00" {
		t.Errorf("owed %s after the bounce, want 1000.00", got)
	}
	if s.rows(t) != rows {
		t.Errorf("a bounce moved money")
	}
	s.mustRefuse(t, "bounced twice", s.admin.Do("POST", s.chequeTo(c, "return"), map[string]string{"status": "cancelled"}, nil))
	if got := s.supBal(t, sup); got != "1000.00" {
		t.Errorf("owed %s after a second bounce, want 1000.00", got)
	}
	// A bill credits still cover stays paid when the cheque on its rest bounces.
	b3 := s.mustDeliver(t, sup, []sl{{"productId": s.prod["odd"], "quantity": "10", "costPrice": "10"}}, nil) // 100
	s.must(t, s.admin.Do("POST", supPath(sup, "payment"), map[string]any{"method": "cheque", "allocations": []map[string]any{{"purchaseId": b3, "amount": "60"}},
		"cheques": []map[string]any{{"lockerId": bank, "number": "710002", "amount": "60", "date": today}}}, nil))
	s.must(t, s.admin.Do("POST", supPath(sup, "payment"), map[string]any{"method": "discount", "allocations": []map[string]any{{"purchaseId": b3, "amount": "40"}}}, nil))
	var c2 int64
	s.must(t, s.db.Get(&c2, `SELECT id FROM cheques WHERE number = '710002'`))
	s.must(t, s.admin.Do("POST", s.chequeTo(c2, "return"), map[string]string{"status": "cancelled"}, nil))
	if bb := s.bill(t, b3); bb.Status != "received" && bb.Status != "partial" {
		t.Errorf("b3 %s with 60 back on it, want open", bb.Status)
	}
	s.audit(t)
}

// CHQ-4: what a bounced cheque didn't put on bills is no advance — it never
// pays a later bill.
func TestCHQ4BouncedAdvanceNeverPays(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	bank := s.locker(t, "Bank", "bank", "10000")
	sup := s.newSup(t, "Advance Bounce Ltd", "")
	c := s.payByCheque(t, sup, bank, "500", "720001", time.Now().Format("2006-01-02")) // all an advance
	s.must(t, s.admin.Do("POST", s.chequeTo(c, "return"), map[string]string{"status": "bounced"}, nil))
	// Some real credit on hand: the shop's own stock sent back (Rs 100).
	s.must(t, s.sendBack(t, sup, 0, map[string]any{"productId": s.prod["plain"], "quantity": "100", "batchId": s.ownLot(t, s.prod["plain"])}))
	b := s.mustDeliver(t, sup, []sl{{"productId": s.prod["plain"], "quantity": "3", "costPrice": "100"}}, nil)
	bb := s.bill(t, b)
	eq(t, "paid out of the bounced cheque", bb.Paid, "0")
	eq(t, "credited from goods sent back", bb.Credited, "100")
	if got := s.supBal(t, sup); got != "200.00" {
		t.Errorf("owed %s, want 200.00", got)
	}
	s.audit(t)
}

// CHQ-5: only a pending cheque's details can be fixed.
func TestCHQ5FixOnlyPending(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	bank := s.locker(t, "Bank", "bank", "10000")
	other := s.locker(t, "Other bank", "bank", "0")
	sup := s.newSup(t, "Fix Ltd", "1000")
	today := time.Now().Format("2006-01-02")
	c := s.payByCheque(t, sup, bank, "400", "730001", today)
	s.must(t, s.admin.Do("PUT", fmt.Sprintf("/api/v1/admin/cheques/%d", c), map[string]any{"lockerId": other, "number": "730009", "date": today, "note": "wrong book"}, nil))
	var num string
	var amount, locker int64
	s.must(t, s.db.QueryRow(`SELECT number, amount::bigint, locker_id FROM cheques WHERE id = $1`, c).Scan(&num, &amount, &locker))
	if num != "730009" || amount != 400 || locker != other {
		t.Errorf("fixed cheque: %s %d on %d, want 730009 400 on %d", num, amount, locker, other)
	}
	s.must(t, s.admin.Do("POST", s.chequeTo(c, "clear"), map[string]string{}, nil))
	eq(t, "the fixed bank paid it", s.lockerBal(t, other), "-400")
	s.mustRefuse(t, "fix a cleared cheque", s.admin.Do("PUT", fmt.Sprintf("/api/v1/admin/cheques/%d", c), map[string]any{"lockerId": bank, "number": "1", "date": today}, nil))
	s.audit(t)
}
