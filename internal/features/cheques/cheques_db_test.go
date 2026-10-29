package cheques

import (
	"context"
	"os"
	"testing"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/db"
	"karots-suite/internal/features/cashflow"
	"karots-suite/internal/features/lockers"
	"karots-suite/internal/features/sales"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// fx is one test's world: a rolled-back tx with a customer, a supplier and a bank place.
type fx struct {
	t        *testing.T
	ctx      context.Context
	tx       *sqlx.Tx
	svc      *Service
	inHand   int64
	bank     int64
	customer int64
	supplier int64
}

func setup(t *testing.T) *fx {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	conn, err := db.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	ctx := context.Background()
	tx, err := conn.BeginTxx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tx.Rollback() }) //nolint:errcheck
	f := &fx{t: t, ctx: ctx, tx: tx, svc: NewService(conn, cashflow.NewService(conn, sales.NewService(conn)))}
	if f.inHand, err = lockers.ChequesInHandTx(ctx, tx); err != nil {
		t.Fatal(err)
	}
	f.bank = f.id(`INSERT INTO lockers (name, kind, allow_negative) VALUES ('TEST chq bank', 'bank', false) RETURNING id`)
	f.customer = f.id(`INSERT INTO customers (name, credit_limit) VALUES ('TEST chq customer', 0) RETURNING id`)
	f.supplier = f.id(`INSERT INTO suppliers (name) VALUES ('TEST chq supplier') RETURNING id`)
	return f
}

func (f *fx) id(q string, args ...any) int64 {
	f.t.Helper()
	var id int64
	if err := f.tx.GetContext(f.ctx, &id, q, args...); err != nil {
		f.t.Fatalf("%s: %v", q, err)
	}
	return id
}

func (f *fx) num(q string, args ...any) decimal.Decimal {
	f.t.Helper()
	var d decimal.Decimal
	if err := f.tx.GetContext(f.ctx, &d, q, args...); err != nil {
		f.t.Fatalf("%s: %v", q, err)
	}
	return d
}

func (f *fx) balance(locker int64) decimal.Decimal {
	return f.num(`SELECT COALESCE(SUM(balance_delta), 0) FROM locker_ledger WHERE locker_id = $1`, locker)
}

func (f *fx) owed() decimal.Decimal {
	return f.num(`SELECT outstanding_balance FROM customers WHERE id = $1`, f.customer)
}

func (f *fx) exec(q string, args ...any) {
	f.t.Helper()
	if _, err := f.tx.ExecContext(f.ctx, q, args...); err != nil {
		f.t.Fatalf("%s: %v", q, err)
	}
}

func eq(t *testing.T, what string, got decimal.Decimal, want int64) {
	t.Helper()
	if !got.Equal(decimal.NewFromInt(want)) {
		t.Fatalf("%s = %s, want %d", what, got, want)
	}
}

func isCode(err error, code string) bool {
	ae, ok := apperr.As(err)
	return ok && ae.Code == code
}

func today() time.Time { return time.Now() }

func line(number string, amount int64) Line {
	return Line{Bank: "Bank of Ceylon", Number: number, Amount: decimal.NewFromInt(amount), Date: today()}
}

// receive takes a 5,000 cheque from the test customer.
func (f *fx) receive(number string) *Cheque {
	f.t.Helper()
	c, err := ReceiveTx(f.ctx, f.tx, f.svc.cf, line(number, 5000), Link{CustomerID: f.customer, Party: "TEST chq customer"}, 0)
	if err != nil {
		f.t.Fatal(err)
	}
	return c
}

func TestReceiveHoldsInHand(t *testing.T) {
	f := setup(t)
	before, owed := f.balance(f.inHand), f.owed()
	c := f.receive("100001")
	if c.Direction != "in" || c.Status != "pending" {
		t.Fatalf("cheque %+v", c)
	}
	eq(t, "in hand", f.balance(f.inHand).Sub(before), 5000)
	eq(t, "customer owes", f.owed().Sub(owed), 0)
	if _, err := ReceiveTx(f.ctx, f.tx, f.svc.cf, line("100002", 5000), Link{}, 0); !isCode(err, "VALIDATION_ERROR") {
		t.Fatalf("no customer: %v", err)
	}
}

func TestClearReceived(t *testing.T) {
	f := setup(t)
	before := f.balance(f.inHand)
	c := f.receive("100003")
	if err := f.svc.clearTx(f.ctx, f.tx, c.ID, today(), f.bank, 0); err != nil {
		t.Fatal(err)
	}
	eq(t, "in hand", f.balance(f.inHand).Sub(before), 0)
	eq(t, "bank", f.balance(f.bank), 5000)
	got, err := get(f.ctx, f.tx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "cleared" || got.LockerID == nil || *got.LockerID != f.bank || got.StatusDate == nil {
		t.Fatalf("after clear %+v", got)
	}
	var n int
	if err := f.tx.GetContext(f.ctx, &n, `SELECT count(*) FROM audit_log WHERE entity = 'cheque' AND entity_id = $1`, itoa(c.ID)); err != nil || n < 2 {
		t.Fatalf("audit rows %d, %v", n, err)
	}
}

func testReturnReceived(t *testing.T, status string) {
	f := setup(t)
	before, owed := f.balance(f.inHand), f.owed()
	c := f.receive("100004")
	if err := f.svc.ReturnTx(f.ctx, f.tx, c.ID, status, 0); err != nil {
		t.Fatal(err)
	}
	eq(t, "in hand", f.balance(f.inHand).Sub(before), 0)
	eq(t, "bank", f.balance(f.bank), 0)
	eq(t, "customer owes", f.owed().Sub(owed), 5000)
	got, _ := get(f.ctx, f.tx, c.ID)
	if got.Status != status {
		t.Fatalf("status %s, want %s", got.Status, status)
	}
}

func TestBounceReceived(t *testing.T) { testReturnReceived(t, "bounced") }
func TestCancelReceived(t *testing.T) { testReturnReceived(t, "cancelled") }

func TestClearGiven(t *testing.T) {
	f := setup(t)
	f.exec(`INSERT INTO locker_ledger (locker_id, balance_delta, kind, note) VALUES ($1, 1000, 'intake', 'TEST')`, f.bank)
	pay := f.id(`INSERT INTO supplier_payments (supplier_id, amount, method) VALUES ($1, 3000, 'cheque') RETURNING id`, f.supplier)
	l := line("200001", 3000)
	l.LockerID = f.bank
	c, err := GiveTx(f.ctx, f.tx, l, Link{SupplierID: f.supplier, SupplierPaymentID: &pay, Party: "TEST chq supplier"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, "bank after write", f.balance(f.bank), 1000)
	if err := f.svc.clearTx(f.ctx, f.tx, c.ID, today(), 0, 0); err != nil {
		t.Fatal(err)
	}
	eq(t, "bank after clear", f.balance(f.bank), -2000)
	var kind string
	if err := f.tx.GetContext(f.ctx, &kind, `SELECT kind FROM money_receipts WHERE ref_kind = 'supplier_payment' AND ref_id = $1`, pay); err != nil || kind != "supplier_payment" {
		t.Fatalf("receipt kind %q, %v", kind, err)
	}
	safe := f.id(`INSERT INTO lockers (name, kind) VALUES ('TEST chq safe', 'safe') RETURNING id`)
	l.LockerID, l.Number = safe, "200002"
	if _, err := GiveTx(f.ctx, f.tx, l, Link{SupplierID: f.supplier}, 0); !isCode(err, "VALIDATION_ERROR") {
		t.Fatalf("drawn on a safe: %v", err)
	}
}

func itoa(id int64) string { return decimal.NewFromInt(id).String() }
