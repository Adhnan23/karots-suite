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
