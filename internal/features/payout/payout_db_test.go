package payout

import (
	"context"
	"os"
	"testing"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/db"
	"karots-suite/internal/features/cashflow"
	"karots-suite/internal/features/cheques"
	"karots-suite/internal/features/expenses"
	"karots-suite/internal/features/purchases"
	"karots-suite/internal/features/sales"
	"karots-suite/internal/features/supplierpay"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

type fx struct {
	t        *testing.T
	ctx      context.Context
	tx       *sqlx.Tx
	svc      *Service
	chq      *cheques.Service
	bank     int64
	safe     int64
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
	cf := cashflow.NewService(conn, sales.NewService(conn))
	f := &fx{t: t, ctx: ctx, tx: tx, svc: NewService(conn, cf), chq: cheques.NewService(conn, cf)}
	f.bank = f.id(`INSERT INTO lockers (name, kind, allow_negative) VALUES ('TEST payout bank', 'bank', false) RETURNING id`)
	f.safe = f.id(`INSERT INTO lockers (name, kind, allow_negative) VALUES ('TEST payout safe', 'safe', true) RETURNING id`)
	f.supplier = f.id(`INSERT INTO suppliers (name) VALUES ('TEST payout supplier') RETURNING id`)
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

func eq(t *testing.T, what string, got decimal.Decimal, want int64) {
	t.Helper()
	if !got.Equal(decimal.NewFromInt(want)) {
		t.Fatalf("%s = %s, want %d", what, got, want)
	}
}

func (f *fx) cheque(number string, amount int64) cheques.Line {
	return cheques.Line{LockerID: f.bank, Number: number, Amount: decimal.NewFromInt(amount), Date: time.Now()}
}

func (f *fx) bill(amount int64) int64 {
	f.t.Helper()
	d, err := purchases.CreateBillTx(f.ctx, f.tx, purchases.BillInput{SupplierID: f.supplier, Amount: decimal.NewFromInt(amount)}, 0)
	if err != nil {
		f.t.Fatal(err)
	}
	return d.Purchase.ID
}

func TestSupplierByCheques(t *testing.T) {
	f := setup(t)
	bill := f.bill(1000)
	owedBefore := f.num(`SELECT outstanding_balance FROM suppliers WHERE id = $1`, f.supplier)
	rec, err := f.svc.SupplierTx(f.ctx, f.tx, f.supplier, "TEST payout supplier",
		supplierpay.PayInput{Allocations: []supplierpay.Alloc{{PurchaseID: bill, Amount: decimal.NewFromInt(1000)}}},
		Source{Method: MethodCheque, Cheques: []cheques.Line{f.cheque("500001", 700), f.cheque("500002", 300)}}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if rec != nil {
		t.Fatal("a cheque payment moved money now")
	}
	var status string
	if err := f.tx.GetContext(f.ctx, &status, `SELECT status FROM purchases WHERE id = $1`, bill); err != nil || status != "paid" {
		t.Fatalf("bill status %q (%v), want paid", status, err)
	}
	eq(t, "supplier owed change", f.num(`SELECT outstanding_balance FROM suppliers WHERE id = $1`, f.supplier).Sub(owedBefore), -1000)
	eq(t, "pending given cheques", f.num(`SELECT count(*) FROM cheques WHERE supplier_id = $1 AND direction = 'out' AND status = 'pending'
		AND supplier_payment_id IS NOT NULL`, f.supplier), 2)
	eq(t, "bank", f.balance(f.bank), 0)
	var method string
	if err := f.tx.GetContext(f.ctx, &method, `SELECT method FROM supplier_payments WHERE supplier_id = $1`, f.supplier); err != nil || method != "cheque" {
		t.Fatalf("payment method %q (%v)", method, err)
	}
}

func TestSupplierChequesMustAddUp(t *testing.T) {
	f := setup(t)
	bill := f.bill(1000)
	_, err := f.svc.SupplierTx(f.ctx, f.tx, f.supplier, "TEST payout supplier",
		supplierpay.PayInput{Allocations: []supplierpay.Alloc{{PurchaseID: bill, Amount: decimal.NewFromInt(1000)}}},
		Source{Method: MethodCheque, Cheques: []cheques.Line{f.cheque("500003", 900)}}, 0)
	if ae, ok := apperr.As(err); !ok || ae.Code != "VALIDATION_ERROR" {
		t.Fatalf("900 of cheques for 1,000: err = %v, want validation", err)
	}
}

func TestExpenseByChequeBounceAndPayAgain(t *testing.T) {
	f := setup(t)
	before := f.num(`SELECT COALESCE(SUM(amount),0) FROM expenses`)
	id, rec, err := f.svc.ExpenseTx(f.ctx, f.tx, expenses.CreateInput{Category: "Rent", Amount: "4000"},
		Source{Method: MethodCheque, Cheques: []cheques.Line{f.cheque("600001", 4000)}}, 0)
	if err != nil || rec != nil {
		t.Fatalf("expense by cheque: rec %v err %v", rec, err)
	}
	eq(t, "bank", f.balance(f.bank), 0)
	var chq int64
	if err := f.tx.GetContext(f.ctx, &chq, `SELECT id FROM cheques WHERE expense_id = $1 AND status = 'pending'`, id); err != nil {
		t.Fatal(err)
	}
	if err := f.chq.ReturnTx(f.ctx, f.tx, chq, "bounced", 0); err != nil {
		t.Fatal(err)
	}
	eq(t, "unpaid after bounce", f.num(`SELECT unpaid FROM expenses WHERE id = $1`, id), 4000)

	f.exec(`INSERT INTO locker_ledger (locker_id, balance_delta, kind, note) VALUES ($1, 5000, 'adjust', 'TEST')`, f.safe)
	if _, err := f.svc.PayAgainTx(f.ctx, f.tx, id, Source{Loc: cashflow.Locker(f.safe), Method: "cash"}, 0); err != nil {
		t.Fatal(err)
	}
	eq(t, "unpaid after pay again", f.num(`SELECT unpaid FROM expenses WHERE id = $1`, id), 0)
	eq(t, "safe", f.balance(f.safe), 1000)
	eq(t, "expenses total counted once", f.num(`SELECT COALESCE(SUM(amount),0) FROM expenses`).Sub(before), 4000)
	if _, err := f.svc.PayAgainTx(f.ctx, f.tx, id, Source{Loc: cashflow.Locker(f.safe), Method: "cash"}, 0); err == nil {
		t.Fatal("paid an already-paid expense again")
	}
}

func TestCashStillMovesNow(t *testing.T) {
	f := setup(t)
	f.exec(`INSERT INTO locker_ledger (locker_id, balance_delta, kind, note) VALUES ($1, 2000, 'adjust', 'TEST')`, f.safe)
	id, rec, err := f.svc.ExpenseTx(f.ctx, f.tx, expenses.CreateInput{Category: "Tea", Amount: "150"}, Source{Loc: cashflow.Locker(f.safe), Method: "cash"}, 0)
	if err != nil || rec == nil {
		t.Fatalf("cash expense: rec %v err %v", rec, err)
	}
	eq(t, "safe", f.balance(f.safe), 1850)
	eq(t, "ledger rows for the expense", f.num(`SELECT count(*) FROM locker_ledger WHERE ref_kind = 'expense' AND ref_id = $1`, id), 1)
}

func (f *fx) exec(q string, args ...any) {
	f.t.Helper()
	if _, err := f.tx.ExecContext(f.ctx, q, args...); err != nil {
		f.t.Fatalf("%s: %v", q, err)
	}
}
