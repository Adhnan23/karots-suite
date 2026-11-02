package api

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"karots-suite/internal/features/purchases"
	"karots-suite/internal/features/supplierpay"
)

// settleEnv is an admin, a product in stock and a fresh supplier's money routes.
type settleEnv struct {
	te      *testEnv
	ck      *http.Cookie
	product int64
}

func newSettleEnv(t *testing.T) settleEnv {
	te := newTestEnv(t)
	purchases.WithSettleCredit(supplierpay.NewService(te.db).ApplySupplierCreditTx) // as the real app wires it
	t.Cleanup(func() { purchases.WithSettleCredit(nil) })
	f := te.seedTill(t)
	te.db.MustExec(`UPDATE stock SET quantity = quantity + 50 WHERE product_id=$1`, f.plainID)
	return settleEnv{te, te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "admin"), f.plainID}
}

func (e settleEnv) post(t *testing.T, url string, body any) {
	t.Helper()
	if rec := e.te.do(http.MethodPost, url, body, e.ck); rec.Code >= 300 {
		t.Fatalf("POST %s: %d %s", url, rec.Code, rec.Body)
	}
}

func (e settleEnv) supplier(t *testing.T, opening string) int64 {
	t.Helper()
	return decode[AdminSupplierRow](t, e.te.do(http.MethodPost, "/api/v1/admin/suppliers",
		SupplierRequest{Name: fmt.Sprintf("T Settle %d", time.Now().UnixNano()), OpeningBalance: opening}, e.ck)).ID
}

func (e settleEnv) deliver(t *testing.T, sup int64, cost string) {
	t.Helper()
	e.post(t, "/api/v1/admin/purchases/receive", PurchaseRequest{SupplierID: sup, Lines: []StockLine{{ProductID: e.product, Quantity: "1", CostPrice: cost, DiscountType: "fixed"}}})
}

func (e settleEnv) writeOff(t *testing.T) int64 {
	t.Helper()
	return decode[ConsumeResult](t, e.te.do(http.MethodPost, "/api/v1/admin/stock/damage", ConsumeRequest{ProductID: e.product, Quantity: "1", Reason: "damage"}, e.ck)).LossID
}

// The bill on the Purchases list (Received tab), in the Purchases report and on the
// statement, all at once.
func (e settleEnv) checkBill(t *testing.T, what string, sup int64, status, paid, credited, due string) {
	t.Helper()
	list := decode[PurchaseList](t, e.te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/purchases?supplier=%d", sup), nil, e.ck))
	if len(list.Items) != 1 {
		t.Fatalf("%s: %d bills", what, len(list.Items))
	}
	b := list.Items[0]
	if b.Status != status || b.Paid != paid || b.Credited != credited || b.Balance != due {
		t.Errorf("%s: list %s paid %s credited %s due %s; want %s %s %s %s", what, b.Status, b.Paid, b.Credited, b.Balance, status, paid, credited, due)
	}
	today := time.Now().Format(time.DateOnly)
	rep := decode[PurchasesReport](t, e.te.do(http.MethodGet, "/api/v1/admin/reports/purchases?from="+today+"&to="+today, nil, e.ck))
	found := false
	for _, r := range rep.Rows {
		if r.ID == b.ID {
			found = true
			if r.Paid != paid || r.Credited != credited || r.Due != due {
				t.Errorf("%s: report paid %s credited %s due %s; want %s %s %s", what, r.Paid, r.Credited, r.Due, paid, credited, due)
			}
		}
	}
	if !found {
		t.Errorf("%s: the bill isn't in the report", what)
	}
	st := decode[Statement](t, e.te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/suppliers/%d/statement", sup), nil, e.ck))
	d := decode[AdminSupplierDetail](t, e.te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/suppliers/%d", sup), nil, e.ck))
	if d.Owed != due || st.Balance != due {
		t.Errorf("%s: owed %s, statement %s; want %s", what, d.Owed, st.Balance, due)
	}
}

// A bill settled by a discount and a cheque: the bounce takes back only the
// cheque. The discount stays, once, and paying again closes the bill.
func TestBouncedChequeOnADiscountedBill(t *testing.T) {
	e := newSettleEnv(t)
	stamp := time.Now().UnixNano()
	var bank int64
	if err := e.te.db.Get(&bank, `INSERT INTO lockers (name, kind) VALUES ($1, 'bank') RETURNING id`, fmt.Sprintf("T settle bank %d", stamp)); err != nil {
		t.Fatal(err)
	}
	sup := e.supplier(t, "")
	pay := fmt.Sprintf("/api/v1/admin/suppliers/%d/payment", sup)
	e.deliver(t, sup, "1000")
	e.post(t, pay, AdminSupplierPayRequest{Amount: "200", Method: "discount"})
	e.checkBill(t, "after the discount", sup, "received", "0.00", "200.00", "800.00")
	e.post(t, pay, AdminSupplierPayRequest{Amount: "800", Method: "cheque", Cheques: []ChequeLine{{LockerID: bank, Number: fmt.Sprintf("9%d", stamp%100000), Amount: "800", Date: time.Now().Format(time.DateOnly)}}})
	e.checkBill(t, "after the cheque", sup, "paid", "800.00", "200.00", "0.00")

	var chq int64
	if err := e.te.db.Get(&chq, `SELECT id FROM cheques WHERE supplier_id = $1`, sup); err != nil {
		t.Fatal(err)
	}
	today := time.Now().Format(time.DateOnly)
	finance := "/api/v1/admin/finance?from=" + today + "&to=" + today
	before := decode[AdminFinance](t, e.te.do(http.MethodGet, finance, nil, e.ck)).DiscountsReceived
	if rec := e.te.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/cheques/%d/return", chq), ChequeReturnRequest{Status: "bounced"}, e.ck); rec.Code != http.StatusNoContent {
		t.Fatalf("bounce: %d %s", rec.Code, rec.Body)
	}
	e.checkBill(t, "after the bounce", sup, "received", "0.00", "200.00", "800.00")
	if after := decode[AdminFinance](t, e.te.do(http.MethodGet, finance, nil, e.ck)).DiscountsReceived; after != before {
		t.Errorf("the bounce moved the discount in the profit report: %s to %s", before, after)
	}

	e.post(t, pay, AdminSupplierPayRequest{Amount: "800", Method: "online"})
	e.checkBill(t, "paid again", sup, "paid", "800.00", "200.00", "0.00")
}

// A discount off old debt has no bill behind it, but it is not an advance: no
// money went, so it must never read as money paid on a later bill.
func TestDiscountOnOldDebtIsNotAnAdvance(t *testing.T) {
	e := newSettleEnv(t)
	sup := e.supplier(t, "100")
	e.post(t, fmt.Sprintf("/api/v1/admin/suppliers/%d/payment", sup), AdminSupplierPayRequest{Amount: "100", Method: "discount"})
	e.post(t, "/api/v1/admin/recovery", RecoveryRequest{SourceType: "damage", SourceID: e.writeOff(t), Outcome: "paid", SupplierID: sup, Amount: "300"})
	e.deliver(t, sup, "300")
	e.checkBill(t, "credit pays the delivery", sup, "paid", "0.00", "300.00", "0.00")
	var left string
	e.te.db.Get(&left, `SELECT COALESCE(SUM(credit_unapplied), 0)::text FROM loss_recoveries WHERE supplier_id = $1`, sup) //nolint:errcheck
	if left != "0.00" {
		t.Errorf("credit still waiting: %s", left)
	}
}

// Two people (or a double tap) settling one loss: it's credited once.
func TestALossIsSettledOnce(t *testing.T) {
	e := newSettleEnv(t)
	sup := e.supplier(t, "")
	for range 5 {
		loss := e.writeOff(t)
		var wg sync.WaitGroup
		for range 4 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				e.te.do(http.MethodPost, "/api/v1/admin/recovery", RecoveryRequest{SourceType: "damage", SourceID: loss, Outcome: "paid", SupplierID: sup, Amount: "100"}, e.ck)
			}()
		}
		wg.Wait()
	}
	var n int
	e.te.db.Get(&n, `SELECT COUNT(*) FROM loss_recoveries WHERE supplier_id = $1`, sup) //nolint:errcheck
	if n != 5 {
		t.Errorf("%d recoveries for 5 losses", n)
	}
}
