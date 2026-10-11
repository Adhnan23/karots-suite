package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestTillSaleLineGroups(t *testing.T) {
	te := newTestEnv(t)
	te.db.Exec(`UPDATE settings SET allow_untracked_cash = true`) //nolint:errcheck
	f := te.seedTill(t)
	ck := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "cashier")
	te.do(http.MethodPost, "/api/v1/till/drawer/open", OpenDrawerRequest{Breakdown: map[string]float64{"500": 2}}, ck)
	defer te.do(http.MethodPost, "/api/v1/till/drawer/close", CloseDrawerRequest{}, ck)

	g := &SaleLineGroup{No: 1, Label: "Pair (deal)", Sets: "2"}
	req := sale(f.plainID, "2", PaymentRequest{Method: "cash", Amount: "500"})
	req.Items[0].Discount, req.Items[0].Group = "10", g
	req.Items = append(req.Items, SaleItemRequest{ProductID: f.plainID, Quantity: "2", Discount: "10", DiscountType: "fixed", Group: g})
	res := decode[SaleResult](t, te.do(http.MethodPost, "/api/v1/till/sales", req, ck))
	v := decode[SaleView](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/till/sales/%d", res.ID), nil, ck))
	if len(v.Lines) != 2 {
		t.Fatalf("lines: %+v", v.Lines)
	}
	for _, l := range v.Lines {
		if l.Group != 1 || l.GroupLabel != "Pair (deal)" || l.GroupSets != "2" {
			t.Fatalf("group not kept: %+v", l)
		}
	}
	// A plain sale has no group.
	plain := decode[SaleResult](t, te.do(http.MethodPost, "/api/v1/till/sales", sale(f.plainID, "1", PaymentRequest{Method: "cash", Amount: "100"}), ck))
	if l := decode[SaleView](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/till/sales/%d", plain.ID), nil, ck)).Lines[0]; l.Group != 0 || l.GroupLabel != "" {
		t.Fatalf("plain line has a group: %+v", l)
	}
	// A group without its label or sets is refused.
	bad := sale(f.plainID, "1", PaymentRequest{Method: "cash", Amount: "100"})
	bad.Items[0].Group = &SaleLineGroup{No: 1, Label: " ", Sets: "1"}
	if rec := te.do(http.MethodPost, "/api/v1/till/sales", bad, ck); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("blank label = %d %s", rec.Code, rec.Body)
	}
}
