package repairs

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func d(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func TestOnAccountPrefill(t *testing.T) {
	for _, c := range []struct{ dep, out, bill, want string }{
		{"500", "-500", "2000", "500"},  // the deposit is still an advance
		{"500", "-200", "2000", "200"},  // part of it went on another buy
		{"500", "300", "2000", "0"},     // they owe now: never add debt
		{"500", "-900", "300", "300"},   // bill smaller than the deposit
		{"0", "-900", "300", "0"},       // no deposit on this job
	} {
		if got := onAccountPrefill(d(c.dep), d(c.out), d(c.bill)); !got.Equal(d(c.want)) {
			t.Errorf("dep %s out %s bill %s: got %s want %s", c.dep, c.out, c.bill, got, c.want)
		}
	}
}

func TestEstimateAndOldDiscounts(t *testing.T) {
	det := &Detail{
		Parts: []Part{
			{Qty: d("2"), Price: d("100")},
			{Qty: d("1"), Price: d("400"), DiscountType: "percent", DiscountValue: d("10")},
			{Qty: d("2"), Price: d("50"), DiscountType: "fixed", DiscountValue: d("80")}, // clamped to gross
		},
		Charges: []Charge{{Amount: d("750")}},
	}
	if got := estimate(det); !got.Equal(d("1310")) { // 200 + 360 + 0 + 750
		t.Fatalf("estimate %s", got)
	}
}

func TestLegacyHeldAndPaid(t *testing.T) {
	det := &Detail{
		Job:      Job{RepairerPaid: d("150")},
		Payments: []Payment{{Kind: "deposit", Amount: d("500")}, {Kind: "applied", Amount: d("300")}, {Kind: "refund", Amount: d("50")}},
		Payouts:  []Payout{{Amount: d("100")}},
	}
	if got := legacyHeld(det); !got.Equal(d("150")) {
		t.Fatalf("held %s", got)
	}
	if got := repairerPaid(det); !got.Equal(d("250")) {
		t.Fatalf("paid %s", got)
	}
}

func TestWarranty(t *testing.T) {
	from := time.Date(2026, 10, 14, 15, 30, 0, 0, time.UTC)
	det := &Detail{Job: Job{WarrantyDays: 30}, Parts: []Part{{WarrantyDays: 90}, {WarrantyDays: 7}}}
	if u := jobWarrantyUntil("days", det, from); u == nil || u.Format("2006-01-02") != "2026-11-13" {
		t.Fatalf("days mode %v", u)
	}
	if u := jobWarrantyUntil("parts", det, from); u == nil || u.Format("2006-01-02") != "2027-01-12" {
		t.Fatalf("parts mode %v", u)
	}
	if warrantyUntil(0, from) != nil {
		t.Fatal("0 days = no warranty")
	}
}

func TestDueLabel(t *testing.T) {
	now := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	day := func(n int) *time.Time { x := now.AddDate(0, 0, n); return &x }
	for _, c := range []struct {
		j      Job
		label  string
		urgent bool
	}{
		{Job{Status: "received", PromisedDate: day(-2)}, "overdue 2d", true},
		{Job{Status: "ready", PromisedDate: day(0)}, "due today", true},
		{Job{Status: "received", PromisedDate: day(1)}, "due tomorrow", true},
		{Job{Status: "received", PromisedDate: day(5)}, "in 5d", false},
		{Job{Status: "received", PromisedDate: day(5), Urgent: true}, "in 5d", true},
		{Job{Status: "received", Urgent: true}, "", true},
		{Job{Status: "collected", PromisedDate: day(-9), Urgent: true}, "", false},
	} {
		l, u := dueLabel(c.j, now)
		if l != c.label || u != c.urgent {
			t.Errorf("%+v: got %q %v", c.j.PromisedDate, l, u)
		}
	}
}
