package api

import (
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"

	"karots-suite/internal/features/purchases"
	"karots-suite/internal/features/supplierpay"

	"github.com/lib/pq"
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

// A delivery from a supplier is when their expired / written-off stock gets
// settled: the receive pages list it (a batch's own delivery supplier, else the
// product's preferred one), on the counter too.
func TestSupplierClaims(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "admin")
	sup := decode[AdminSupplierRow](t, te.do(http.MethodPost, "/api/v1/admin/suppliers",
		SupplierRequest{Name: fmt.Sprintf("T Claims Supplier %d", time.Now().UnixNano())}, ck))
	te.db.MustExec(`UPDATE products SET preferred_supplier_id=$1 WHERE id=$2`, sup.ID, f.plainID)
	var lot int64
	te.db.Get(&lot, `INSERT INTO stock_batches (product_id, qty_received, qty_remaining, cost_price, expiry_date, source)
		VALUES ($1, 2, 2, 50, CURRENT_DATE - 3, 'adjust') RETURNING id`, f.plainID) //nolint:errcheck
	te.db.MustExec(`UPDATE stock SET quantity = quantity + 2 WHERE product_id=$1`, f.plainID)
	base := fmt.Sprintf("/api/v1/admin/suppliers/%d/claims", sup.ID)

	c := decode[SupplierClaims](t, te.do(http.MethodGet, base, nil, ck))
	if len(c.Expired) != 1 || c.Expired[0].LotID != lot || c.Expired[0].Qty != "2" {
		t.Fatalf("expired on the shelf: %+v", c)
	}
	loss := decode[ConsumeResult](t, te.do(http.MethodPost, fmt.Sprintf("%s/lots/%d/give-back", base, lot), GiveBackRequest{Counted: "2"}, ck)).LossID
	c = decode[SupplierClaims](t, te.do(http.MethodGet, base, nil, ck))
	if len(c.Expired) != 0 || len(c.Waiting) != 1 || c.Waiting[0].SourceID != loss || c.Waiting[0].ProductID != f.plainID || !c.Waiting[0].GivenBack {
		t.Fatalf("after the write-off: %+v", c)
	}

	cash := te.login(t, fmt.Sprintf("07%08d", (time.Now().UnixNano()+7)%100_000_000), "manager") // the counter, as someone who deals with suppliers
	tc := decode[SupplierClaims](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/till/suppliers/%d/claims", sup.ID), nil, cash))
	if len(tc.Waiting) != 1 {
		t.Fatalf("counter claims: %+v", tc)
	}
	if rec := te.do(http.MethodPost, "/api/v1/till/recovery", RecoveryRequest{SourceType: "damage", SourceID: loss, Outcome: "written_off"}, cash); rec.Code >= 300 {
		t.Fatalf("counter settles: %d %s", rec.Code, rec.Body)
	}
	if c = decode[SupplierClaims](t, te.do(http.MethodGet, base, nil, ck)); len(c.Waiting) != 0 {
		t.Fatalf("settled stays listed: %+v", c)
	}
}

func TestSupplierClaimsGiveBack(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "admin")
	sfx := time.Now().UnixNano()
	days := 60
	sup := decode[AdminSupplierRow](t, te.do(http.MethodPost, "/api/v1/admin/suppliers", SupplierRequest{Name: fmt.Sprintf("T Give A %d", sfx), ExpiryWarnDays: &days}, ck))
	other := decode[AdminSupplierRow](t, te.do(http.MethodPost, "/api/v1/admin/suppliers", SupplierRequest{Name: fmt.Sprintf("T Give B %d", sfx)}, ck))
	te.db.MustExec(`UPDATE products SET preferred_supplier_id=$1 WHERE id IN ($2, $3)`, sup.ID, f.plainID, f.lotID)
	lot := func(product int64, days int) int64 {
		var id int64
		if err := te.db.Get(&id, `INSERT INTO stock_batches (product_id, qty_received, qty_remaining, cost_price, expiry_date, source)
			VALUES ($1, 2, 2, 50, CURRENT_DATE + $2::int, 'adjust') RETURNING id`, product, days); err != nil {
			t.Fatal(err)
		}
		te.db.MustExec(`UPDATE stock SET quantity = quantity + 2 WHERE product_id=$1`, product)
		return id
	}
	inside, outside := lot(f.plainID, 45), lot(f.plainID, 90)
	base := fmt.Sprintf("/api/v1/admin/suppliers/%d/claims", sup.ID)
	claims := func() SupplierClaims { return decode[SupplierClaims](t, te.do(http.MethodGet, base, nil, ck)) }
	has := func(lots []ClaimLot, id int64) bool {
		return slices.ContainsFunc(lots, func(l ClaimLot) bool { return l.LotID == id })
	}

	c := claims()
	if !has(c.Expiring, inside) || has(c.Expired, inside) || has(c.Expiring, outside) || has(c.Expired, outside) {
		t.Fatalf("close to expiry: %+v", c)
	}

	// Told: on the claims answer and on the product's lots.
	if rec := te.do(http.MethodPost, fmt.Sprintf("%s/lots/%d/told", base, inside), nil, ck); rec.Code != http.StatusNoContent {
		t.Fatalf("told: %d %s", rec.Code, rec.Body)
	}
	today := time.Now().Format(time.DateOnly)
	c = claims()
	if i := slices.IndexFunc(c.Expiring, func(l ClaimLot) bool { return l.LotID == inside }); i < 0 || c.Expiring[i].ToldOn != today {
		t.Fatalf("told date: %+v", c.Expiring)
	}
	pl := decode[ProductLots](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/stock/lots/%d", f.plainID), nil, ck))
	if i := slices.IndexFunc(pl.Lots, func(l LotRow) bool { return l.ID == inside }); i < 0 || pl.Lots[i].ToldOn != today || pl.Lots[i].SupplierID != sup.ID {
		t.Fatalf("lot row: %+v", pl.Lots)
	}

	// Another supplier's lot isn't reachable through this supplier.
	wrong := fmt.Sprintf("/api/v1/admin/suppliers/%d/claims/lots/%d", other.ID, outside)
	if rec := te.do(http.MethodPost, wrong+"/told", nil, ck); rec.Code != http.StatusNotFound {
		t.Errorf("told, wrong supplier: %d", rec.Code)
	}
	if rec := te.do(http.MethodPost, wrong+"/give-back", GiveBackRequest{Counted: "2"}, ck); rec.Code != http.StatusNotFound {
		t.Errorf("give back, wrong supplier: %d", rec.Code)
	}

	// Nothing counted: no loss, nothing waits, the lot is empty.
	zero := decode[ConsumeResult](t, te.do(http.MethodPost, fmt.Sprintf("%s/lots/%d/give-back", base, outside), GiveBackRequest{Counted: "0"}, ck))
	if c = claims(); zero.LossID != 0 || len(c.Waiting) != 0 {
		t.Fatalf("zero count: %+v %+v", zero, c)
	}

	// A hand-over keeps waiting past the window; an ordinary write-off doesn't.
	given := decode[ConsumeResult](t, te.do(http.MethodPost, fmt.Sprintf("%s/lots/%d/give-back", base, inside), GiveBackRequest{Counted: "2"}, ck)).LossID
	old := lot(f.lotID, 45)
	plain := decode[ConsumeResult](t, te.do(http.MethodPost, "/api/v1/admin/stock/damage", ConsumeRequest{ProductID: f.lotID, Quantity: "2", Reason: "damage", BatchID: old}, ck)).LossID
	if c = claims(); len(c.Waiting) != 2 {
		t.Fatalf("waiting: %+v", c.Waiting)
	}
	te.db.MustExec(`UPDATE stock_movements SET created_at = now() - interval '200 days' WHERE id = ANY($1)`, pq.Array([]int64{given, plain}))
	if c = claims(); len(c.Waiting) != 1 || c.Waiting[0].SourceID != given || !c.Waiting[0].GivenBack {
		t.Fatalf("after 200 days: %+v", c.Waiting)
	}

	// It stays with who took it, whatever the product's preferred supplier becomes.
	te.db.MustExec(`UPDATE products SET preferred_supplier_id=$1 WHERE id=$2`, other.ID, f.plainID)
	if c = claims(); len(c.Waiting) != 1 || c.Waiting[0].SourceID != given {
		t.Fatalf("after the preferred supplier changed: %+v", c.Waiting)
	}
	te.db.MustExec(`UPDATE products SET preferred_supplier_id=NULL WHERE id=$1`, f.plainID)
	if c = claims(); len(c.Waiting) != 1 {
		t.Fatalf("after it was cleared: %+v", c.Waiting)
	}
	otherClaims := decode[SupplierClaims](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/suppliers/%d/claims", other.ID), nil, ck))
	if len(otherClaims.Waiting) != 0 {
		t.Fatalf("it moved: %+v", otherClaims.Waiting)
	}
}

func TestSupplierRecall(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "admin")
	sfx := time.Now().UnixNano()
	sup := decode[AdminSupplierRow](t, te.do(http.MethodPost, "/api/v1/admin/suppliers", SupplierRequest{Name: fmt.Sprintf("T Recall A %d", sfx)}, ck))
	other := decode[AdminSupplierRow](t, te.do(http.MethodPost, "/api/v1/admin/suppliers", SupplierRequest{Name: fmt.Sprintf("T Recall B %d", sfx)}, ck))
	te.db.MustExec(`UPDATE products SET preferred_supplier_id=$1 WHERE id=$2`, sup.ID, f.scarceID)
	var lot int64 // nowhere near expiry: no date at all
	if err := te.db.Get(&lot, `SELECT id FROM stock_batches WHERE product_id=$1 AND qty_remaining > 0 AND expiry_date IS NULL LIMIT 1`, f.scarceID); err != nil {
		t.Fatal(err)
	}
	base := fmt.Sprintf("/api/v1/admin/suppliers/%d/claims", sup.ID)
	path := fmt.Sprintf("%s/lots/%d/recall", base, lot)
	claims := func() SupplierClaims { return decode[SupplierClaims](t, te.do(http.MethodGet, base, nil, ck)) }
	optsFor := func() []Lot {
		o := decode[map[string][]Lot](t, te.do(http.MethodGet, "/api/v1/till/price-options", nil, ck))
		return o[fmt.Sprint(f.scarceID)]
	}
	attention := func() int {
		return decode[Dashboard](t, te.do(http.MethodGet, "/api/v1/admin/dashboard", nil, ck)).Attention.Recalled
	}
	before := attention()

	// Keep selling: on the panel, nothing at the counter.
	if rec := te.do(http.MethodPost, path, RecallRequest{StopSelling: false}, ck); rec.Code != http.StatusNoContent {
		t.Fatalf("recall: %d %s", rec.Code, rec.Body)
	}
	if c := claims(); len(c.Recalled) != 1 || c.Recalled[0].LotID != lot || c.Recalled[0].RecallStops || c.Recalled[0].Expiry != "" {
		t.Fatalf("recalled: %+v", c)
	}
	if opts := optsFor(); len(opts) != 0 {
		t.Fatalf("keep selling shouldn't prompt: %+v", opts)
	}
	if got := attention(); got != before {
		t.Fatalf("keep selling on the dashboard: %d", got)
	}

	// Stop selling: the till is told, the dashboard counts it, the lot says so.
	te.do(http.MethodPost, path, RecallRequest{StopSelling: true}, ck)
	if opts := optsFor(); len(opts) != 1 || !opts[0].Recalled {
		t.Fatalf("till options: %+v", opts)
	}
	if got := attention(); got != before+1 {
		t.Fatalf("dashboard %d, want %d", got, before+1)
	}
	pl := decode[ProductLots](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/stock/lots/%d", f.scarceID), nil, ck))
	if i := slices.IndexFunc(pl.Lots, func(l LotRow) bool { return l.ID == lot }); i < 0 || pl.Lots[i].RecalledOn == "" || !pl.Lots[i].RecallStops {
		t.Fatalf("lot row: %+v", pl.Lots)
	}

	// Expired stays first on Batches, then recalled, whatever the names.
	te.db.MustExec(`UPDATE products SET name = '0 ' || name WHERE id=$1`, f.scarceID)
	v := decode[BatchesView](t, te.do(http.MethodGet, "/api/v1/admin/stock/batches", nil, ck))
	firstRecalled := slices.IndexFunc(v.Groups, func(g BatchGroup) bool { return g.Recalled > 0 && g.Expired == 0 })
	lastExpired := -1
	for i, g := range v.Groups {
		if g.Expired > 0 {
			lastExpired = i
		}
	}
	if firstRecalled < 0 || lastExpired < 0 || firstRecalled < lastExpired {
		t.Fatalf("order: recalled at %d, expired up to %d", firstRecalled, lastExpired)
	}

	// It still counts as stock until it's given back.
	var left float64
	te.db.Get(&left, `SELECT quantity FROM stock WHERE product_id=$1`, f.scarceID) //nolint:errcheck
	if left != 1 {
		t.Fatalf("stock %v, want 1", left)
	}

	// Another supplier can't touch it.
	wrong := fmt.Sprintf("/api/v1/admin/suppliers/%d/claims/lots/%d/recall", other.ID, lot)
	if rec := te.do(http.MethodPost, wrong, RecallRequest{StopSelling: true}, ck); rec.Code != http.StatusNotFound {
		t.Errorf("wrong supplier: %d", rec.Code)
	}
	if rec := te.do(http.MethodDelete, wrong, nil, ck); rec.Code != http.StatusNotFound {
		t.Errorf("wrong supplier, undo: %d", rec.Code)
	}

	// Undo: back to normal everywhere.
	if rec := te.do(http.MethodDelete, path, nil, ck); rec.Code != http.StatusNoContent {
		t.Fatalf("undo: %d %s", rec.Code, rec.Body)
	}
	if c := claims(); len(c.Recalled) != 0 {
		t.Fatalf("after undo: %+v", c.Recalled)
	}
	if opts := optsFor(); len(opts) != 0 {
		t.Fatalf("after undo, till: %+v", opts)
	}

	// Given back: it leaves the recalled list and waits on them.
	te.do(http.MethodPost, path, RecallRequest{StopSelling: true}, ck)
	if rec := te.do(http.MethodPost, fmt.Sprintf("%s/lots/%d/give-back", base, lot), GiveBackRequest{Counted: "1"}, ck); rec.Code != http.StatusOK {
		t.Fatalf("give back: %d %s", rec.Code, rec.Body)
	}
	if c := claims(); len(c.Recalled) != 0 || len(c.Waiting) != 1 {
		t.Fatalf("after give back: %+v", c)
	}
	if got := attention(); got != before {
		t.Fatalf("dashboard after give back: %d", got)
	}
}

// "Credited us" lands on the bills, not just the total, so nobody pays a bill
// the credit has already covered; what's left pays the next delivery.
func TestCreditedUsSettlesBills(t *testing.T) {
	te := newTestEnv(t)
	purchases.WithSettleCredit(supplierpay.NewService(te.db).ApplySupplierCreditTx) // as the real app wires it
	t.Cleanup(func() { purchases.WithSettleCredit(nil) })
	f := te.seedTill(t)
	ck := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "admin")
	te.db.MustExec(`UPDATE stock SET quantity = quantity + 50 WHERE product_id=$1`, f.plainID)
	var n int
	newSup := func(opening string) AdminSupplierRow {
		n++
		return decode[AdminSupplierRow](t, te.do(http.MethodPost, "/api/v1/admin/suppliers", SupplierRequest{Name: fmt.Sprintf("T Credit %d %d", time.Now().UnixNano(), n), OpeningBalance: opening}, ck))
	}
	deliver := func(sup int64, cost string) {
		t.Helper()
		if rec := te.do(http.MethodPost, "/api/v1/admin/purchases/receive", PurchaseRequest{SupplierID: sup, Lines: []StockLine{{ProductID: f.plainID, Quantity: "1", CostPrice: cost, DiscountType: "fixed"}}}, ck); rec.Code >= 300 {
			t.Fatalf("deliver: %d %s", rec.Code, rec.Body)
		}
	}
	credit := func(sup int64, amount string) {
		t.Helper()
		loss := decode[ConsumeResult](t, te.do(http.MethodPost, "/api/v1/admin/stock/damage", ConsumeRequest{ProductID: f.plainID, Quantity: "1", Reason: "damage"}, ck)).LossID
		if rec := te.do(http.MethodPost, "/api/v1/admin/recovery", RecoveryRequest{SourceType: "damage", SourceID: loss, Outcome: "paid", SupplierID: sup, Amount: amount}, ck); rec.Code >= 300 {
			t.Fatalf("credit: %d %s", rec.Code, rec.Body)
		}
	}
	check := func(what string, sup int64, owed string, open ...string) {
		t.Helper()
		d := decode[AdminSupplierDetail](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/suppliers/%d", sup), nil, ck))
		got := make([]string, 0, len(d.Invoices))
		for _, i := range d.Invoices {
			got = append(got, i.Balance)
		}
		if d.Owed != owed || !slices.Equal(got, open) {
			t.Errorf("%s: owed %s, open bills %v; want %s, %v", what, d.Owed, got, owed, open)
		}
	}

	s := newSup("")
	deliver(s.ID, "1000")
	deliver(s.ID, "400")
	credit(s.ID, "1100")
	check("oldest bill first", s.ID, "300.00", "300.00")

	s = newSup("")
	credit(s.ID, "400")
	check("no bills: they owe us", s.ID, "-400.00")
	deliver(s.ID, "300")
	check("the replacement is covered", s.ID, "-100.00")
	deliver(s.ID, "200")
	check("the rest of the credit runs out", s.ID, "100.00", "100.00")

	// A credit that only eats old debt leaves nothing to pay a later bill with.
	s = newSup("100")
	credit(s.ID, "30")
	check("old debt", s.ID, "70.00")
	deliver(s.ID, "50")
	check("old debt, then a delivery", s.ID, "120.00", "50.00")

	// Handed over as cash instead: the credit is gone, so the next bill stands.
	s = newSup("")
	credit(s.ID, "400")
	if rec := te.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/suppliers/%d/refund", s.ID), AdminMoneyRequest{Amount: "400", Method: "online"}, ck); rec.Code >= 300 {
		t.Fatalf("refund: %d %s", rec.Code, rec.Body)
	}
	deliver(s.ID, "300")
	check("refunded credit", s.ID, "300.00", "300.00")
}
