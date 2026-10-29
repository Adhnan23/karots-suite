package cheques

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func (f *fx) give(number string, amount int64, k Link) *Cheque {
	f.t.Helper()
	l := line(number, amount)
	l.LockerID = f.bank
	c, err := GiveTx(f.ctx, f.tx, l, k, 0)
	if err != nil {
		f.t.Fatal(err)
	}
	return c
}

func TestSupplierBounceCapsReopen(t *testing.T) {
	f := setup(t)
	a := f.id(`INSERT INTO purchases (supplier_id, status, total, paid_amount) VALUES ($1, 'paid', 2000, 2000) RETURNING id`, f.supplier)
	b := f.id(`INSERT INTO purchases (supplier_id, status, total, paid_amount) VALUES ($1, 'paid', 1000, 1000) RETURNING id`, f.supplier)
	pay := f.id(`INSERT INTO supplier_payments (supplier_id, amount, method) VALUES ($1, 3000, 'cheque') RETURNING id`, f.supplier)
	f.exec(`INSERT INTO supplier_payment_allocations (payment_id, purchase_id, amount) VALUES ($1, $2, 2000), ($1, $3, 1000)`, pay, a, b)
	f.exec(`UPDATE purchases SET paid_amount = 300, status = 'partial' WHERE id = $1`, b)
	owes := f.num(`SELECT outstanding_balance FROM suppliers WHERE id = $1`, f.supplier)
	c := f.give("300001", 3000, Link{SupplierID: f.supplier, SupplierPaymentID: &pay})
	if err := f.svc.ReturnTx(f.ctx, f.tx, c.ID, "bounced", 0); err != nil {
		t.Fatal(err)
	}
	eq(t, "B paid", f.num(`SELECT paid_amount FROM purchases WHERE id = $1`, b), 0)
	eq(t, "A paid", f.num(`SELECT paid_amount FROM purchases WHERE id = $1`, a), 0)
	var st string
	if err := f.tx.GetContext(f.ctx, &st, `SELECT status FROM purchases WHERE id = $1`, a); err != nil || st != "received" {
		t.Fatalf("A status %q, %v", st, err)
	}
	eq(t, "supplier owed", f.num(`SELECT outstanding_balance FROM suppliers WHERE id = $1`, f.supplier).Sub(owes), 3000)
	var over int
	if err := f.tx.GetContext(f.ctx, &over, `SELECT count(*) FROM supplier_payment_allocations WHERE payment_id = $1 AND reopened > amount`, pay); err != nil || over != 0 {
		t.Fatalf("over-reopened %d, %v", over, err)
	}
	eq(t, "bank", f.balance(f.bank), 0)
	if err := f.svc.ReturnTx(f.ctx, f.tx, c.ID, "bounced", 0); !isCode(err, "CONFLICT") {
		t.Fatalf("second return: %v", err)
	}
}

func TestExpenseBounceUnpaid(t *testing.T) {
	f := setup(t)
	exp := f.id(`INSERT INTO expenses (category, amount) VALUES ('TEST chq', 1200) RETURNING id`)
	c := f.give("300002", 1200, Link{ExpenseID: &exp})
	if err := f.svc.ReturnTx(f.ctx, f.tx, c.ID, "cancelled", 0); err != nil {
		t.Fatal(err)
	}
	eq(t, "unpaid", f.num(`SELECT unpaid FROM expenses WHERE id = $1`, exp), 1200)
	var n int
	if err := f.tx.GetContext(f.ctx, &n, `SELECT count(*) FROM locker_ledger WHERE locker_id = $1`, f.bank); err != nil || n != 0 {
		t.Fatalf("ledger rows %d, %v", n, err)
	}
}

func TestStatusChangeIsFinal(t *testing.T) {
	f := setup(t)
	c := f.receive("300003")
	if err := f.svc.clearTx(f.ctx, f.tx, c.ID, today(), f.bank, 0); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.clearTx(f.ctx, f.tx, c.ID, today(), f.bank, 0); !isCode(err, "CONFLICT") {
		t.Fatalf("clear twice: %v", err)
	}
	if err := f.svc.ReturnTx(f.ctx, f.tx, c.ID, "bounced", 0); !isCode(err, "CONFLICT") {
		t.Fatalf("return after clear: %v", err)
	}
	if err := fixTx(f.ctx, f.tx, c.ID, FixInput{Bank: "HSBC", Number: "9", Date: "2026-01-01"}, 0); !isCode(err, "CONFLICT") {
		t.Fatalf("fix after clear: %v", err)
	}
}

func TestFixPending(t *testing.T) {
	f := setup(t)
	c := f.receive("300006")
	if err := fixTx(f.ctx, f.tx, c.ID, FixInput{Bank: "HSBC", Number: "300007", Date: "2026-01-02", Note: "n"}, 0); err != nil {
		t.Fatal(err)
	}
	got, _ := get(f.ctx, f.tx, c.ID)
	if got.Bank != "HSBC" || got.Number != "300007" || got.ChequeDate.Format("2006-01-02") != "2026-01-02" || !got.Amount.Equal(decimal.NewFromInt(5000)) {
		t.Fatalf("after fix %+v", got)
	}
}

func TestClearDateAndPlace(t *testing.T) {
	f := setup(t)
	c := f.receive("300004")
	if err := f.svc.clearTx(f.ctx, f.tx, c.ID, today().AddDate(0, 0, -1), f.bank, 0); !isCode(err, "VALIDATION_ERROR") {
		t.Fatalf("before cheque date: %v", err)
	}
	if err := f.svc.clearTx(f.ctx, f.tx, c.ID, today().AddDate(0, 0, 1), f.bank, 0); !isCode(err, "VALIDATION_ERROR") {
		t.Fatalf("tomorrow: %v", err)
	}
	safe := f.id(`INSERT INTO lockers (name, kind) VALUES ('TEST chq safe', 'safe') RETURNING id`)
	if err := f.svc.clearTx(f.ctx, f.tx, c.ID, today(), safe, 0); !isCode(err, "VALIDATION_ERROR") {
		t.Fatalf("into a safe: %v", err)
	}
}

func TestDuplicateAndBanks(t *testing.T) {
	f := setup(t)
	f.receive("300005")
	if dup, err := Duplicate(f.ctx, f.tx, "in", "Bank of Ceylon", 0, "300005"); err != nil || !dup {
		t.Fatalf("in duplicate %v, %v", dup, err)
	}
	if dup, _ := Duplicate(f.ctx, f.tx, "in", "HSBC", 0, "300005"); dup {
		t.Fatal("other bank counted as duplicate")
	}
	f.give("400001", 10, Link{SupplierID: f.supplier})
	if dup, err := Duplicate(f.ctx, f.tx, "out", "", f.bank, "400001"); err != nil || !dup {
		t.Fatalf("out duplicate %v, %v", dup, err)
	}
	l := line("300008", 100)
	l.Bank = "TEST Village Bank"
	l.Date = time.Now()
	if _, err := ReceiveTx(f.ctx, f.tx, f.svc.cf, l, Link{CustomerID: f.customer}, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := ReceiveTx(f.ctx, f.tx, f.svc.cf, l, Link{CustomerID: f.customer}, 0); err != nil {
		t.Fatal(err)
	}
	banks, err := Banks(f.ctx, f.tx)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for _, b := range banks {
		seen[b]++
	}
	if seen["Bank of Ceylon"] != 1 || seen["TEST Village Bank"] != 1 {
		t.Fatalf("banks %v", banks)
	}
}

func TestParseLines(t *testing.T) {
	ls, sum, err := ParseLines([]LineInput{{Bank: "BOC", Number: "1", Amount: "100.50", Date: "2026-10-01"}, {Bank: "HNB", Number: "2", Amount: "200", Date: "2026-10-02"}}, "in")
	if err != nil || len(ls) != 2 || sum.String() != "300.5" {
		t.Fatalf("%v %s %v", ls, sum, err)
	}
	for _, bad := range [][]LineInput{
		nil,
		{{Number: "1", Amount: "1", Date: "2026-10-01"}},            // in needs a bank
		{{Bank: "B", Amount: "1", Date: "2026-10-01"}},              // number
		{{Bank: "B", Number: "1", Amount: "0", Date: "2026-10-01"}}, // amount
		{{Bank: "B", Number: "1", Amount: "1", Date: "01/10/2026"}}, // date
	} {
		if _, _, err := ParseLines(bad, "in"); !isCode(err, "VALIDATION_ERROR") {
			t.Fatalf("%v accepted: %v", bad, err)
		}
	}
	if _, _, err := ParseLines([]LineInput{{Number: "1", Amount: "1", Date: "2026-10-01"}}, "out"); !isCode(err, "VALIDATION_ERROR") {
		t.Fatal("out without a bank place accepted")
	}
}
