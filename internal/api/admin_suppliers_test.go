package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestAdminSuppliers(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, "0770009602", "admin")
	sfx := time.Now().UnixNano()
	var safe int64
	if err := te.db.Get(&safe, `INSERT INTO lockers (name, kind, allow_negative) VALUES ($1, 'safe', true) RETURNING id`, fmt.Sprintf("T safe %d", sfx)); err != nil {
		t.Fatal(err)
	}
	src := fmt.Sprintf("locker:%d", safe)
	path := func(id int64, rest string) string { return fmt.Sprintf("/api/v1/admin/suppliers/%d%s", id, rest) }
	get := func(id int64) AdminSupplierDetail { return decode[AdminSupplierDetail](t, te.do(http.MethodGet, path(id, ""), nil, ck)) }

	name := fmt.Sprintf("T Admin Supplier %d", sfx)
	rec := te.do(http.MethodPost, "/api/v1/admin/suppliers", SupplierRequest{Name: name, Phone: "0711111111", CreditDays: 30, OpeningBalance: "1000"}, ck)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	sup := decode[AdminSupplierRow](t, rec)
	if rec := te.do(http.MethodPost, "/api/v1/admin/suppliers", SupplierRequest{Name: name}, ck); rec.Code != http.StatusConflict {
		t.Fatalf("duplicate name: %d", rec.Code)
	}
	list := decode[AdminSupplierList](t, te.do(http.MethodGet, "/api/v1/admin/suppliers?show=owing&q="+fmt.Sprint(sfx), nil, ck))
	if list.Total != 1 || list.Items[0].Owed != "1000.00" || list.Owed != "1000.00" {
		t.Fatalf("list: %+v", list)
	}

	// A 500 delivery on credit → one open invoice next to the 1000 old debt.
	te.do(http.MethodPost, "/api/v1/till/drawer/open", OpenDrawerRequest{Breakdown: map[string]float64{"500": 1}}, ck)
	defer te.do(http.MethodPost, "/api/v1/till/drawer/close", CloseDrawerRequest{}, ck)
	rec = te.do(http.MethodPost, fmt.Sprintf("/api/v1/till/suppliers/%d/receive", sup.ID), ReceiveRequest{DiscountType: "fixed", DeferDrawerKick: true,
		Lines: []StockLine{{ProductID: f.plainID, Quantity: "10", CostPrice: "50", DiscountType: "fixed"}}}, ck)
	if rec.Code != http.StatusCreated {
		t.Fatalf("receive: %d %s", rec.Code, rec.Body)
	}
	d := get(sup.ID)
	if d.Owed != "1500.00" || len(d.Invoices) != 1 || d.Invoices[0].Balance != "500.00" {
		t.Fatalf("detail: %+v", d)
	}

	// Cash needs a source; then exact: 200 on the invoice + 100 old debt + 50 advance.
	if rec := te.do(http.MethodPost, path(sup.ID, "/payment"), AdminSupplierPayRequest{Amount: "100", Method: "cash"}, ck); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("cash without source: %d", rec.Code)
	}
	rec = te.do(http.MethodPost, path(sup.ID, "/payment"), AdminSupplierPayRequest{Method: "cash", Source: src, Advance: "50", OldDebt: "100",
		Allocations: []PayAlloc{{PurchaseID: d.Invoices[0].ID, Amount: "200"}}}, ck)
	if rec.Code != http.StatusCreated || decode[ReceiptRef](t, rec).ReceiptID == nil {
		t.Fatalf("exact pay: %d %s", rec.Code, rec.Body)
	}
	d = get(sup.ID)
	if d.Owed != "1150.00" || d.OldDebt != "900.00" || d.Invoices[0].Balance != "300.00" || len(d.Money) != 1 || d.Money[0].Amount != "350.00" {
		t.Fatalf("after exact pay: %+v", d)
	}
	var bal string
	te.db.Get(&bal, `SELECT COALESCE(SUM(balance_delta),0)::numeric(14,2)::text FROM locker_ledger WHERE locker_id=$1`, safe) //nolint:errcheck
	if bal != "-350.00" {
		t.Fatalf("safe balance %s", bal)
	}
	// Waterfall, invoices first, by an untracked online transfer: 1250 clears everything, 100 over.
	if rec := te.do(http.MethodPost, path(sup.ID, "/payment"), AdminSupplierPayRequest{Amount: "1250", Mode: "pay", Method: "online"}, ck); rec.Code != http.StatusCreated || decode[ReceiptRef](t, rec).ReceiptID != nil {
		t.Fatalf("online pay: %d %s", rec.Code, rec.Body)
	}
	if d = get(sup.ID); d.Owed != "-100.00" || len(d.Invoices) != 0 {
		t.Fatalf("after waterfall: %+v", d)
	}
	// They owe us 100: a refund of more is refused, 60 into the safe is fine.
	if rec := te.do(http.MethodPost, path(sup.ID, "/refund"), AdminMoneyRequest{Amount: "150", Method: "cash", Source: src}, ck); rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("refund over credit: %d", rec.Code)
	}
	if rec := te.do(http.MethodPost, path(sup.ID, "/refund"), AdminMoneyRequest{Amount: "60", Method: "cash", Source: src}, ck); rec.Code != http.StatusCreated {
		t.Fatalf("refund: %d %s", rec.Code, rec.Body)
	}
	st := decode[Statement](t, te.do(http.MethodGet, path(sup.ID, "/statement"), nil, ck))
	if last := st.Lines[len(st.Lines)-1]; st.Balance != "-40.00" || last.Kind != "Refund received" || last.Balance != "-40.00" {
		t.Fatalf("statement: %+v", st)
	}

	// Opening correction touches only the old debt; audited.
	if rec := te.do(http.MethodPost, path(sup.ID, "/opening"), OpeningRequest{Opening: "200", Note: "found old bill"}, ck); rec.Code != http.StatusNoContent {
		t.Fatalf("opening: %d %s", rec.Code, rec.Body)
	}
	if d = get(sup.ID); d.OldDebt != "200.00" || d.Owed != "160.00" {
		t.Fatalf("after opening: %+v", d)
	}

	// Reassign products to another supplier, then disable / reactivate.
	other := decode[AdminSupplierRow](t, te.do(http.MethodPost, "/api/v1/admin/suppliers", SupplierRequest{Name: name + " B"}, ck))
	te.db.Exec(`UPDATE products SET preferred_supplier_id=$1 WHERE id=$2`, sup.ID, f.plainID) //nolint:errcheck
	if n := decode[CountResult](t, te.do(http.MethodPost, path(sup.ID, "/reassign"), ReassignRequest{ToID: other.ID}, ck)); n.Count != 1 {
		t.Fatalf("reassign: %+v", n)
	}
	if rec := te.do(http.MethodDelete, path(other.ID, ""), nil, ck); rec.Code != http.StatusNoContent {
		t.Fatalf("disable: %d", rec.Code)
	}
	if l := decode[AdminSupplierList](t, te.do(http.MethodGet, "/api/v1/admin/suppliers?q="+fmt.Sprint(sfx), nil, ck)); l.Total != 1 {
		t.Fatalf("disabled still listed: %+v", l)
	}
	if l := decode[AdminSupplierList](t, te.do(http.MethodGet, "/api/v1/admin/suppliers?disabled=1&q="+fmt.Sprint(sfx), nil, ck)); l.Total != 2 {
		t.Fatalf("show disabled: %+v", l)
	}
	if rec := te.do(http.MethodPost, path(other.ID, "/activate"), nil, ck); rec.Code != http.StatusNoContent {
		t.Fatalf("activate: %d", rec.Code)
	}
}
