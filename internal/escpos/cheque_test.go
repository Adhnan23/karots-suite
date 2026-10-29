package escpos

import (
	"strings"
	"testing"
	"time"

	"karots-suite/internal/features/sales"

	"github.com/shopspring/decimal"
)

func TestChequeText(t *testing.T) {
	if got := ChequeText("123456", "Bank of Ceylon", time.Date(2026, 11, 12, 0, 0, 0, 0, time.Local)); got != "CHEQUE 123456 Bank of Ceylon 12/11/26" {
		t.Fatalf("ChequeText = %q", got)
	}
}

// A sale paid by cheque + cash prints the cheque's number, bank and date with
// its amount, on 58 mm and 80 mm, with no line wider than the paper.
func TestReceiptPrintsChequeLine(t *testing.T) {
	d := decimal.RequireFromString
	num, bank, status := "123456", "Hatton National Bank", "pending"
	date := time.Date(2026, 11, 12, 0, 0, 0, 0, time.Local)
	det := sampleDetail()
	det.Sale.PaidAmount, det.Sale.ChangeGiven = d("1400.00"), d("0")
	det.Payments = []sales.Payment{
		{Method: "cheque", Amount: d("1000.00"), ChequeNumber: &num, ChequeBank: &bank, ChequeDate: &date, ChequeStatus: &status},
		{Method: "cash", Amount: d("400.00")},
	}
	for _, width := range []string{"58", "80"} {
		s := string(Document(det, cfg(width), Options{}))
		for _, want := range []string{"CHEQUE 123456", "1,000.00", "400.00"} {
			if !strings.Contains(s, want) {
				t.Errorf("%s mm receipt missing %q\n%s", width, want, s)
			}
		}
		w := columns(width)
		for _, l := range strings.Split(s, "\n") {
			if (strings.Contains(l, "CHEQUE") || strings.Contains(l, "Bank") || strings.Contains(l, "1,000.00")) && printable(l) > w {
				t.Errorf("%s mm line wider than %d: %q", width, w, l)
			}
		}
	}
}

// A credit payment made with two cheques lists both on its slip.
func TestDebtSlipListsCheques(t *testing.T) {
	d := decimal.RequireFromString
	date := time.Date(2026, 11, 12, 0, 0, 0, 0, time.Local)
	s := string(DebtDocument(DebtSlip{ReceiptNo: "DP-000001", Date: "2026-10-29", CustomerName: "Nimal", Method: "Cheque", Amount: d("1500"),
		Cheques: []SlipCheque{{Number: "100001", Bank: "Bank of Ceylon", Date: date, Amount: d("1000")}, {Number: "200002", Bank: "Sampath Bank", Date: date, Amount: d("500")}}},
		cfg("80"), Options{}))
	for _, want := range []string{"CHEQUE 100001 Bank of Ceylon 12/11/26", "CHEQUE 200002 Sampath Bank", "500.00"} {
		if !strings.Contains(s, want) {
			t.Errorf("debt slip missing %q\n%s", want, s)
		}
	}
}

// printable counts a line's visible characters (ESC/POS commands removed).
func printable(l string) int {
	n := 0
	for i := 0; i < len(l); i++ {
		switch c := l[i]; {
		case c == 0x1b || c == 0x1d:
			i += 2 // command byte + one argument
		case c >= 0x20 && c < 0x7f:
			n++
		}
	}
	return n
}
