package supplierpay

import (
	"testing"

	"karots-suite/internal/features/purchases"

	"github.com/shopspring/decimal"
)

func dinv(id int64, total string) purchases.Purchase {
	return purchases.Purchase{ID: id, Total: decimal.RequireFromString(total)}
}

func allocMap(in PayInput) map[int64]string {
	m := map[int64]string{}
	for _, a := range in.Allocations {
		m[a.PurchaseID] = a.Amount.StringFixed(2)
	}
	return m
}

// Two invoices (2000 oldest, 1000) + 400 old debt. Test the cascade in both
// modes, including the leftover-becomes-advance case.
func TestDistribute(t *testing.T) {
	invoices := []purchases.Purchase{dinv(1, "2000"), dinv(2, "1000")}
	opening := decimal.RequireFromString("400")

	// mode "pay": invoices oldest-first, then old debt, then advance.
	// Pay 2500 -> invoice1 2000, invoice2 500, nothing to old, no advance.
	in := Distribute(invoices, opening, decimal.RequireFromString("2500"), "pay")
	am := allocMap(in)
	if am[1] != "2000.00" || am[2] != "500.00" {
		t.Fatalf("pay 2500 allocations = %v, want {1:2000,2:500}", am)
	}
	if !in.Opening.IsZero() || !in.Unallocated.IsZero() {
		t.Fatalf("pay 2500: opening=%s unalloc=%s, want 0/0", in.Opening, in.Unallocated)
	}

	// Pay 5000 (total debt 3400): invoices 3000, old 400, advance 1600.
	in = Distribute(invoices, opening, decimal.RequireFromString("5000"), "pay")
	am = allocMap(in)
	if am[1] != "2000.00" || am[2] != "1000.00" {
		t.Fatalf("pay 5000 allocations = %v", am)
	}
	if in.Opening.StringFixed(2) != "400.00" {
		t.Fatalf("pay 5000 opening = %s, want 400.00", in.Opening)
	}
	if in.Unallocated.StringFixed(2) != "1600.00" {
		t.Fatalf("pay 5000 advance = %s, want 1600.00", in.Unallocated)
	}

	// mode "old": old debt first, then invoices, then advance.
	// Pay 1000 -> old 400, invoice1 600.
	in = Distribute(invoices, opening, decimal.RequireFromString("1000"), "old")
	am = allocMap(in)
	if in.Opening.StringFixed(2) != "400.00" {
		t.Fatalf("old 1000 opening = %s, want 400.00", in.Opening)
	}
	if am[1] != "600.00" {
		t.Fatalf("old 1000 allocations = %v, want {1:600}", am)
	}
	if _, ok := am[2]; ok {
		t.Fatalf("old 1000 should not touch invoice 2: %v", am)
	}
}

// A bill already paid off (or credited to zero) gets nothing and is skipped, and
// a negative amount pays nothing at all — never a negative allocation or advance.
func TestDistributeSkipsPaidBillsAndNegatives(t *testing.T) {
	paid := dinv(1, "500")
	paid.PaidAmount = decimal.RequireFromString("300")
	paid.CreditedAmount = decimal.RequireFromString("200")
	invoices := []purchases.Purchase{paid, dinv(2, "1000")}
	in := Distribute(invoices, decimal.Zero, decimal.RequireFromString("700"), "pay")
	if am := allocMap(in); len(am) != 1 || am[2] != "700.00" {
		t.Errorf("700 over a paid bill and a 1000 one = %v, want {2:700}", am)
	}
	in = Distribute(invoices, decimal.RequireFromString("400"), decimal.RequireFromString("-50"), "old")
	if len(in.Allocations) != 0 || !in.Opening.IsZero() || !in.Unallocated.IsZero() {
		t.Errorf("-50 paid %+v, want nothing", in)
	}
}
