package sales

import (
	"testing"

	"github.com/shopspring/decimal"
)

func td(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func TestSplitTenderSeparatesAccountFromMoney(t *testing.T) {
	got := SplitTender(
		[]string{"cash", "credit", "card"},
		[]decimal.Decimal{td("500"), td("700"), td("100")},
	)
	if !got.Paid.Equal(td("600")) {
		t.Errorf("Paid = %s, want 600", got.Paid)
	}
	if !got.OnAccount.Equal(td("700")) {
		t.Errorf("OnAccount = %s, want 700", got.OnAccount)
	}
}

func TestCheckTenderAcceptsExactCash(t *testing.T) {
	tn := Tender{Paid: td("1200"), OnAccount: decimal.Zero}
	if err := CheckTender(tn, td("1200"), false, decimal.Zero, false); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCheckTenderAcceptsOverpaymentInCash(t *testing.T) {
	tn := Tender{Paid: td("2000"), Cash: td("2000"), OnAccount: decimal.Zero}
	if err := CheckTender(tn, td("1200"), false, decimal.Zero, false); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// The bug this whole change exists to kill: money missing and nobody named.
func TestCheckTenderRejectsAShortfallWithNoAccountLine(t *testing.T) {
	tn := Tender{Paid: td("500"), OnAccount: decimal.Zero}
	if err := CheckTender(tn, td("1200"), true, td("9999"), false); err == nil {
		t.Error("a short-paid sale was accepted")
	}
}

func TestCheckTenderAcceptsPartCashPartAccount(t *testing.T) {
	tn := Tender{Paid: td("500"), OnAccount: td("700")}
	if err := CheckTender(tn, td("1200"), true, td("5000"), false); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCheckTenderRequiresACustomerForAnAccountLine(t *testing.T) {
	tn := Tender{Paid: td("500"), OnAccount: td("700")}
	if err := CheckTender(tn, td("1200"), false, decimal.Zero, false); err == nil {
		t.Error("an account line with no customer was accepted")
	}
}

func TestCheckTenderEnforcesTheCreditLimit(t *testing.T) {
	tn := Tender{Paid: decimal.Zero, OnAccount: td("700")}
	if err := CheckTender(tn, td("700"), true, td("300"), false); err == nil {
		t.Error("borrowing past the credit limit was accepted")
	}
}

// You cannot hand back cash against money that was never paid.
func TestCheckTenderRejectsChangeOnACreditSale(t *testing.T) {
	tn := Tender{Paid: td("1000"), OnAccount: td("700")}
	if err := CheckTender(tn, td("1200"), true, td("5000"), false); err == nil {
		t.Error("an over-covered credit sale was accepted")
	}
}

// An authorised cashier may push an account line past the limit.
func TestCheckTenderAllowsOverLimitWhenApproved(t *testing.T) {
	tn := Tender{Paid: decimal.Zero, OnAccount: td("700")}
	if err := CheckTender(tn, td("700"), true, td("300"), true); err != nil {
		t.Errorf("an approved over-limit sale should pass: %v", err)
	}
}

func TestCheckTenderStillRejectsOverLimitWithoutApproval(t *testing.T) {
	tn := Tender{Paid: decimal.Zero, OnAccount: td("700")}
	if err := CheckTender(tn, td("700"), true, td("300"), false); err == nil {
		t.Error("over-limit without approval must still be refused")
	}
}

// Approval only waives the limit — a shortfall (unpaid) is still an error.
func TestCheckTenderApprovalDoesNotWaiveShortfall(t *testing.T) {
	tn := Tender{Paid: td("500"), OnAccount: decimal.Zero}
	if err := CheckTender(tn, td("1200"), true, td("9999"), true); err == nil {
		t.Error("a shortfall must be refused even with over-limit approval")
	}
}

func TestTenderPaidCash(t *testing.T) {
	cases := []struct {
		name string
		in   []PaymentInput
		want bool
	}{
		{"cash", []PaymentInput{{Method: "cash", Amount: "100"}}, true},
		{"card only", []PaymentInput{{Method: "card", Amount: "100"}}, false},
		{"credit only", []PaymentInput{{Method: "credit", Amount: "100"}}, false},
		{"mixed card+cash", []PaymentInput{{Method: "card", Amount: "60"}, {Method: "cash", Amount: "40"}}, true},
		{"cash zero amount", []PaymentInput{{Method: "cash", Amount: "0"}}, false},
		{"empty", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tenderPaidCash(tc.in); got != tc.want {
				t.Fatalf("tenderPaidCash(%s) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestSaleCreditSummary(t *testing.T) {
	d := decimal.RequireFromString
	ptr := func(s string) *decimal.Decimal { v := d(s); return &v }
	cases := []struct {
		name          string
		after         *decimal.Decimal
		used, balance string
	}{
		{"advance covers all", ptr("-200"), "300", "-200"},
		{"advance covers part", ptr("200"), "100", "200"},
		{"advance exactly used", ptr("0"), "300", "0"},
		{"prior debt", ptr("350"), "0", "350"},
		{"legacy uses live, floored at due", nil, "0", "300"},
	}
	for _, c := range cases {
		s := Sale{Status: "credit", Total: d("300"), CustomerBalanceAfter: c.after}
		cs, ok := s.Credit(d("-999"))
		if !ok || !cs.Due.Equal(d("300")) || !cs.FromAdvance.Equal(d(c.used)) || !cs.Balance.Equal(d(c.balance)) {
			t.Errorf("%s: got %+v ok=%v", c.name, cs, ok)
		}
	}
	if _, ok := (Sale{Status: "completed", Total: d("300"), PaidAmount: d("300")}).Credit(d("0")); ok {
		t.Error("paid sale should have no credit summary")
	}
}

// Change comes out of the drawer, so only cash can be over-paid: a card or
// online payment above what's due would hand the difference back as cash.
func TestCheckTenderRejectsChangeFromCardOrOnline(t *testing.T) {
	over := SplitTender([]string{"card"}, []decimal.Decimal{td("5000")})
	if err := CheckTender(over, td("200"), false, decimal.Zero, false); err == nil {
		t.Error("a card payment above the bill was accepted")
	}
	split := SplitTender([]string{"card", "cash"}, []decimal.Decimal{td("150"), td("100")})
	if err := CheckTender(split, td("200"), false, decimal.Zero, false); err != nil {
		t.Errorf("card 150 + cash 100 for 200 (50 change from the cash): %v", err)
	}
	if !split.Cash.Equal(td("100")) {
		t.Errorf("Cash = %s, want 100", split.Cash)
	}
	tooMuch := SplitTender([]string{"card", "cash"}, []decimal.Decimal{td("250"), td("100")})
	if err := CheckTender(tooMuch, td("200"), false, decimal.Zero, false); err == nil {
		t.Error("change bigger than the cash handed over was accepted")
	}
}
