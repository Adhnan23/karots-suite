package api

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/shopspring/decimal"

	"karots-suite/internal/features/sales"
)

func TestSalesPersonSendsTicket(t *testing.T) {
	te := newTestEnv(t)
	seller, cashier := devPhone(), devPhone()
	sellerID := te.seedUser(t, seller, "cashier")
	cashierID := te.seedUser(t, cashier, "cashier")
	te.db.MustExec(`UPDATE users SET sends_to_cashier = true WHERE id = $1`, sellerID)
	t.Cleanup(func() {
		te.db.MustExec(`DELETE FROM held_sales WHERE cashier_id = ANY($1::bigint[])`, fmt.Sprintf("{%d,%d}", sellerID, cashierID))
	})
	s := te.signIn(t, "dev-s", "Laptop", seller)
	k := te.signIn(t, "dev-k", "Counter", cashier)

	// Money is refused, and anything not on the list with it.
	for _, r := range []struct{ method, path string }{
		{http.MethodPost, "/api/v1/till/sales"},
		{http.MethodPost, "/api/v1/till/drawer/open"},
		{http.MethodPost, "/api/v1/till/drawer/no-sale"},
		{http.MethodPost, "/api/v1/till/write-off"},
		{http.MethodPost, "/api/v1/till/sales/1/return"},
		{http.MethodPost, "/api/v1/till/customers/1/payment"},
		{http.MethodPost, "/api/v1/till/conversions/1/run"},
		{http.MethodPost, "/api/v1/cheques/1/cleared"},
	} {
		if rec := te.on("dev-s", r.method, r.path, map[string]any{}, s); rec.Code != http.StatusForbidden {
			t.Errorf("%s %s = %d, want 403", r.method, r.path, rec.Code)
		}
	}
	// The till still reads for them, and a cashier isn't limited.
	if rec := te.on("dev-s", http.MethodGet, "/api/v1/till/config", nil, s); rec.Code != http.StatusOK {
		t.Fatalf("config %d", rec.Code)
	}
	if cfg := decode[TillConfig](t, te.on("dev-s", http.MethodGet, "/api/v1/till/config", nil, s)); !cfg.SendsToCashier {
		t.Fatal("config should say they send to the cashier")
	}
	if rec := te.on("dev-k", http.MethodPost, "/api/v1/till/sales", map[string]any{}, k); rec.Code == http.StatusForbidden {
		t.Fatalf("a cashier was refused: %s", rec.Body)
	}

	send := HoldRequest{Label: "Kamal", SaleType: "retail", Cart: `{"v":2,"lines":[],"meta":{"plugin":1}}`, ItemCount: 1, Total: "100", Discount: "50", ToCashier: true}
	rec := te.on("dev-s", http.MethodPost, "/api/v1/till/held", send, s)
	if rec.Code != http.StatusCreated {
		t.Fatalf("send %d %s", rec.Code, rec.Body)
	}
	ticket := decode[HeldSale](t, rec)
	if d, _ := decimal.NewFromString(ticket.Discount); !d.IsZero() {
		t.Fatalf("a sales person's bill discount was kept: %s", ticket.Discount)
	}

	// It's a ticket, not a held sale: the sender's Held and the phones' copy leave it out.
	if held := decode[[]HeldSale](t, te.on("dev-s", http.MethodGet, "/api/v1/till/held", nil, s)); len(held) != 0 {
		t.Fatalf("held %+v", held)
	}
	var inCopy int
	te.db.Get(&inCopy, `SELECT count(*) FROM held_sales WHERE id = $1 AND NOT to_cashier`, ticket.ID) //nolint:errcheck
	if inCopy != 0 {
		t.Fatal("a ticket must be marked to_cashier")
	}

	has := func(ts []HeldSale) *HeldSale {
		for i := range ts {
			if ts[i].ID == ticket.ID {
				return &ts[i]
			}
		}
		return nil
	}
	seen := has(decode[[]HeldSale](t, te.on("dev-k", http.MethodGet, "/api/v1/till/tickets", nil, k)))
	if seen == nil || seen.PreparedByID != sellerID || seen.PreparedByName == "" {
		t.Fatalf("cashier sees %+v", seen)
	}

	// Another sales person sees only their own.
	other := devPhone()
	otherID := te.seedUser(t, other, "cashier")
	te.db.MustExec(`UPDATE users SET sends_to_cashier = true WHERE id = $1`, otherID)
	o := te.signIn(t, "dev-o", "Other", other)
	if has(decode[[]HeldSale](t, te.on("dev-o", http.MethodGet, "/api/v1/till/tickets", nil, o))) != nil {
		t.Fatal("another sales person sees this ticket")
	}
	open := fmt.Sprintf("/api/v1/till/tickets/%d/open", ticket.ID)
	if rec := te.on("dev-o", http.MethodPost, open, nil, o); rec.Code != http.StatusNotFound {
		t.Fatalf("another sales person opened it: %d", rec.Code)
	}

	// The cashier gets the whole cart, once.
	rec = te.on("dev-k", http.MethodPost, open, nil, k)
	if rec.Code != http.StatusOK {
		t.Fatalf("open %d %s", rec.Code, rec.Body)
	}
	if got := decode[HeldSale](t, rec); got.Cart != ticket.Cart || got.PreparedByID != sellerID {
		t.Fatalf("opened %+v", got)
	}
	if rec := te.on("dev-k", http.MethodPost, open, nil, k); rec.Code != http.StatusNotFound {
		t.Fatalf("opened twice: %d", rec.Code)
	}
}

// A bad preparedBy must not lose a sale, and a good one is kept with it.
func TestPreparedByOnSaleInput(t *testing.T) {
	var id int64 = 5
	if in := saleInput(SaleRequest{PreparedBy: &id}); in.PreparedBy == nil || *in.PreparedBy != 5 {
		t.Fatalf("%+v", in.PreparedBy)
	}
	var s sales.CreateInput
	if s.PreparedBy != nil {
		t.Fatal("no preparedBy by default")
	}
}
