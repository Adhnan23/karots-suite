package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// A supplier's credit for a damaged item (Losses & Recovery → Credited us)
// lowers what we owe them; the statement must show it, or its running balance
// ends above the real one.
func TestSupplierStatementShowsRecoveryCredit(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "admin")
	sup := decode[AdminSupplierRow](t, te.do(http.MethodPost, "/api/v1/admin/suppliers",
		SupplierRequest{Name: fmt.Sprintf("T Recovery Supplier %d", time.Now().UnixNano()), OpeningBalance: "100"}, ck))
	loss := decode[ConsumeResult](t, te.do(http.MethodPost, "/api/v1/admin/stock/damage", ConsumeRequest{ProductID: f.plainID, Quantity: "1", Reason: "damage"}, ck)).LossID
	if rec := te.do(http.MethodPost, "/api/v1/admin/recovery", RecoveryRequest{SourceType: "damage", SourceID: loss, Outcome: "paid", SupplierID: sup.ID, Amount: "30"}, ck); rec.Code >= 300 {
		t.Fatalf("recovery: %d %s", rec.Code, rec.Body)
	}
	st := decode[Statement](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/suppliers/%d/statement", sup.ID), nil, ck))
	last := st.Lines[len(st.Lines)-1]
	if st.Balance != "70.00" || last.Kind != "Credit for a loss" || last.Balance != "70.00" {
		t.Fatalf("statement: balance %s, last %+v (want 70.00 after a 30 credit)", st.Balance, last)
	}
}

// A replacement the supplier wants a little extra for (it costs 75 now, we
// paid 70): the extra goes on what we owe them and on the new batch's cost,
// and the statement shows it. The new batch can carry a new selling price.
func TestReplacementWithAnExtraCharge(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "admin")
	sup := decode[AdminSupplierRow](t, te.do(http.MethodPost, "/api/v1/admin/suppliers",
		SupplierRequest{Name: fmt.Sprintf("T Replace Supplier %d", time.Now().UnixNano())}, ck))
	// The plain product's batch cost is 50: 2 lost = 100.
	loss := decode[ConsumeResult](t, te.do(http.MethodPost, "/api/v1/admin/stock/damage", ConsumeRequest{ProductID: f.plainID, Quantity: "2", Reason: "damage"}, ck)).LossID
	if rec := te.do(http.MethodPost, "/api/v1/admin/recovery", RecoveryRequest{SourceType: "damage", SourceID: loss, Outcome: "replacement",
		SupplierID: sup.ID, ExtraPerUnit: "5", SellingPrice: "130", Expiry: "2027-09-30"}, ck); rec.Code >= 300 {
		t.Fatalf("replacement: %d %s", rec.Code, rec.Body)
	}
	var cost, price float64
	te.db.QueryRow(`SELECT cost_price, selling_price FROM stock_batches WHERE product_id=$1 ORDER BY id DESC LIMIT 1`, f.plainID).Scan(&cost, &price) //nolint:errcheck
	if cost != 55 || price != 130 {
		t.Fatalf("new batch cost %v price %v, want 55 (50 + 5) and 130", cost, price)
	}
	st := decode[Statement](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/suppliers/%d/statement", sup.ID), nil, ck))
	last := st.Lines[len(st.Lines)-1]
	if st.Balance != "10.00" || last.Kind != "Replacement charge" || last.Debit != "10.00" {
		t.Fatalf("statement: balance %s, last %+v (want a 10.00 charge)", st.Balance, last)
	}
	if rec := te.do(http.MethodPost, "/api/v1/admin/recovery", RecoveryRequest{SourceType: "damage", SourceID: decode[ConsumeResult](t, te.do(http.MethodPost, "/api/v1/admin/stock/damage",
		ConsumeRequest{ProductID: f.plainID, Quantity: "1", Reason: "damage"}, ck)).LossID, Outcome: "replacement", ExtraPerUnit: "5"}, ck); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("an extra charge with no supplier: %d", rec.Code)
	}
}
