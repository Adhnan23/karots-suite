package sales

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/shopspring/decimal"
)

// The proven model (proofs/Karots/Refund.lean: a line's refunds never pass what
// was paid and add up to it exactly) wrote these cases; the server must give
// every one to the cent (RET-4, RET-7).
func TestPartRefundMatchesTheProof(t *testing.T) {
	b, err := os.ReadFile("../../../proofs/cases/refunds.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ Paid, Qty, Before, Back, Refund int64 }
	if err := json.Unmarshal(b, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		got := PartRefund(decimal.New(c.Paid, -2), decimal.New(c.Qty, -3), decimal.New(c.Before, -3), decimal.New(c.Back, -3))
		if want := decimal.New(c.Refund, -2); !got.Equal(want) {
			t.Fatalf("paid %d¢ for %d, %d back, %d more: refund %s, the proof says %s", c.Paid, c.Qty, c.Before, c.Back, got, want)
		}
	}
	if len(cases) < 1000 {
		t.Fatalf("only %d cases", len(cases))
	}
}

// proofs/Karots/Share.lean: sharing an amount over lines adds up exactly and
// leaves no line more than a cent from its share; the server shares as it does.
func TestShareOutMatchesTheProof(t *testing.T) {
	b, err := os.ReadFile("../../../proofs/cases/shares.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Amount  int64
		Weights []int64
		Shares  []int64
	}
	if err := json.Unmarshal(b, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		ws := make([]decimal.Decimal, len(c.Weights))
		for i, w := range c.Weights {
			ws[i] = decimal.New(w, -2)
		}
		got := shareOut(decimal.New(c.Amount, -2), ws)
		for i := range got {
			if want := decimal.New(c.Shares[i], -2); !got[i].Equal(want) {
				t.Fatalf("%d¢ over %v: line %d gets %s, the proof's rule %s", c.Amount, c.Weights, i, got[i], want)
			}
		}
	}
}
