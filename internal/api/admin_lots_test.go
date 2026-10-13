package api

import (
	"net/http"
	"strconv"
	"testing"
)

// Batches sheet: + Add batch and Merge batches (M7.6b).
func TestAdminAddAndMergeBatches(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	const phone = "0770009561"
	clean := func() {
		te.db.Exec(`DELETE FROM stock_movements WHERE user_id IN (SELECT id FROM users WHERE phone=$1)`, phone) //nolint:errcheck
	}
	clean()
	ck := te.login(t, phone, "manager")
	t.Cleanup(clean)
	base := "/api/v1/admin/stock/lots/" + strconv.FormatInt(f.lotID, 10)
	before := decode[ProductLots](t, te.do(http.MethodGet, base, nil, ck))

	rec := te.do(http.MethodPost, base+"/add", AddBatchRequest{Qty: "3", Cost: "40", SellingPrice: "90", Expiry: "2027-03-12"}, ck)
	if rec.Code != http.StatusOK {
		t.Fatalf("add: %d %s", rec.Code, rec.Body)
	}
	added := decode[ProductLots](t, rec)
	if len(added.Lots) != len(before.Lots)+1 {
		t.Fatalf("add: %d lots, want %d", len(added.Lots), len(before.Lots)+1)
	}
	var nl LotRow
	for _, l := range added.Lots {
		if l.ExpiryDate == "2027-03-12" {
			nl = l
		}
	}
	if nl.QtyRemaining != "3" || nl.CostPrice != "40.00" || nl.SellingPrice != "90.00" {
		t.Fatalf("new batch: %+v", nl)
	}

	ids := make([]int64, 0, len(added.Lots))
	for _, l := range added.Lots {
		ids = append(ids, l.ID)
	}
	rec = te.do(http.MethodPost, base+"/merge", MergeBatchesRequest{BatchIDs: ids, Cost: "55", Expiry: "2027-03-12", SellingPrice: ""}, ck)
	if rec.Code != http.StatusOK {
		t.Fatalf("merge: %d %s", rec.Code, rec.Body)
	}
	merged := decode[ProductLots](t, rec)
	if len(merged.Lots) != 1 || merged.Lots[0].QtyRemaining != added.StockQty || merged.Lots[0].CostPrice != "55.00" || merged.StockQty != added.StockQty {
		t.Fatalf("merged: %+v (stock was %s)", merged, added.StockQty)
	}
	if rec := te.do(http.MethodPost, base+"/merge", MergeBatchesRequest{BatchIDs: ids[:1]}, ck); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("one batch: %d", rec.Code)
	}

	cash := te.login(t, "0770009562", "cashier")
	if rec := te.do(http.MethodPost, base+"/add", AddBatchRequest{Qty: "1"}, cash); rec.Code != http.StatusForbidden {
		t.Fatalf("cashier add: %d", rec.Code)
	}
}

// Intake tops up the batch it is told to, opens a new one with an expiry, and
// an undo after part of that batch sold still goes through.
func TestAdminIntakeChoosesTheBatch(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	const phone = "0770009563"
	clean := func() {
		te.db.Exec(`DELETE FROM stock_movements WHERE user_id IN (SELECT id FROM users WHERE phone=$1)`, phone) //nolint:errcheck
	}
	clean()
	ck := te.login(t, phone, "manager")
	t.Cleanup(clean)
	lots := decode[ProductLots](t, te.do(http.MethodGet, "/api/v1/admin/stock/lots/"+strconv.FormatInt(f.lotID, 10), nil, ck))
	lot := lots.Lots[0]
	n := len(lots.Lots)

	r := decode[IntakeItem](t, te.do(http.MethodPost, "/api/v1/admin/inventory/intake/restock", IntakeRestockRequest{ProductID: f.lotID, Quantity: "4", BatchID: lot.ID}, ck))
	var rem, recv float64
	te.db.QueryRow(`SELECT qty_remaining, qty_received FROM stock_batches WHERE id=$1`, lot.ID).Scan(&rem, &recv) //nolint:errcheck
	if r.BatchID != lot.ID || rem != mustF(lot.QtyRemaining)+4 || recv != mustF(lot.QtyReceived)+4 {
		t.Fatalf("top-up: item %+v lot %v of %v", r, rem, recv)
	}
	var count int
	te.db.Get(&count, `SELECT COUNT(*) FROM stock_batches WHERE product_id=$1 AND qty_remaining > 0`, f.lotID) //nolint:errcheck
	if count != n {
		t.Fatalf("a top-up opened a batch: %d lots, want %d", count, n)
	}

	nw := decode[IntakeItem](t, te.do(http.MethodPost, "/api/v1/admin/inventory/intake/restock", IntakeRestockRequest{ProductID: f.lotID, Quantity: "2", Expiry: "2027-01-31"}, ck))
	var exp string
	te.db.Get(&exp, `SELECT to_char(expiry_date,'YYYY-MM-DD') FROM stock_batches WHERE id=$1`, nw.BatchID) //nolint:errcheck
	if nw.BatchID == 0 || nw.BatchID == lot.ID || exp != "2027-01-31" {
		t.Fatalf("new batch: %+v exp %q", nw, exp)
	}

	// One of the two new units sold: undo of 2 can't come off that batch alone.
	te.db.MustExec(`UPDATE stock_batches SET qty_remaining = 1 WHERE id=$1`, nw.BatchID)
	te.db.MustExec(`UPDATE stock SET quantity = quantity - 1 WHERE product_id=$1`, f.lotID)
	if rec := te.do(http.MethodPost, "/api/v1/admin/inventory/intake/undo", IntakeUndoRequest{ProductID: f.lotID, Qty: "2", Kind: "restock", BatchID: nw.BatchID}, ck); rec.Code != http.StatusNoContent {
		t.Fatalf("undo after a sale: %d %s", rec.Code, rec.Body)
	}
	// Undo of the top-up comes off that batch.
	te.db.QueryRow(`SELECT qty_remaining FROM stock_batches WHERE id=$1`, lot.ID).Scan(&rem) //nolint:errcheck
	before := rem
	if rec := te.do(http.MethodPost, "/api/v1/admin/inventory/intake/undo", IntakeUndoRequest{ProductID: f.lotID, Qty: "1", Kind: "restock", BatchID: lot.ID}, ck); rec.Code != http.StatusNoContent {
		t.Fatalf("undo: %d %s", rec.Code, rec.Body)
	}
	te.db.QueryRow(`SELECT qty_remaining FROM stock_batches WHERE id=$1`, lot.ID).Scan(&rem) //nolint:errcheck
	if rem != before-1 {
		t.Fatalf("undo took %v off the topped-up batch, want 1", before-rem)
	}
}

func mustF(s string) float64 { v, _ := strconv.ParseFloat(s, 64); return v }
