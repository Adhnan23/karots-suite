package api

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// ?data=1 on every slip path answers the values its Classic slip prints.
func TestSlipData(t *testing.T) {
	te := newTestEnv(t)
	te.db.Exec(`UPDATE settings SET allow_untracked_cash = true, shop_name_si = 'කැරට්ස්'`) //nolint:errcheck
	t.Cleanup(func() { te.db.Exec(`UPDATE settings SET shop_name_si = NULL`) })             //nolint:errcheck
	f := te.seedTill(t)
	ck := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "cashier")
	te.do(http.MethodPost, "/api/v1/till/drawer/open", OpenDrawerRequest{Breakdown: map[string]float64{"500": 2}}, ck)
	defer te.do(http.MethodPost, "/api/v1/till/drawer/close", CloseDrawerRequest{}, ck)
	sfx := time.Now().UnixNano()

	data := func(path string) SlipData {
		t.Helper()
		rec := te.do(http.MethodPost, path, nil, ck)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Body)
		}
		return decode[SlipData](t, rec)
	}

	// Sale: a serial-tracked unit prints its SN line under the item.
	res := decode[SaleResult](t, te.do(http.MethodPost, "/api/v1/till/sales", sale(f.plainID, "2", PaymentRequest{Method: "cash", Amount: "500"}), ck))
	sn := fmt.Sprintf("SN-SLIP-%d", sfx)
	if _, err := te.db.Exec(`INSERT INTO warranty_units (product_id, serial_no, sale_id, sold_at, warranty_months, warranty_until, source)
		VALUES ($1, $2, $3, now(), 12, (now() + interval '1 year')::date, 'sale')`, f.plainID, sn, res.ID); err != nil {
		t.Fatal(err)
	}
	d := data(fmt.Sprintf("/api/v1/till/sales/%d/print?data=1", res.ID))
	if d.Kind != "sale" || d.Dots != 576 || d.Columns != 48 || d.Shop.Name == "" || d.Shop.NameLocal != "කැරට්ස්" || d.Sale == nil {
		t.Fatalf("sale: %+v", d)
	}
	if s := d.Sale; s.ReceiptNo != res.ReceiptNo || s.Total != "200.00" || s.Change != "300.00" || len(s.Items) != 1 ||
		s.Items[0].Quantity != "2" || len(s.Items[0].Serials) != 1 || s.Items[0].Serials[0][:4] != "SN: " || len(s.Payments) != 1 || s.Payments[0].Method != "Cash" {
		t.Fatalf("sale fields: %+v", d.Sale)
	}
	if d58 := data(fmt.Sprintf("/api/v1/till/sales/%d/print?data=1&size=58", res.ID)); d58.Dots != 384 || d58.Width != "58" || d58.Columns != 32 {
		t.Fatalf("size=58: %+v", d58)
	}

	// Refund of one unit.
	v := decode[SaleView](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/till/sales/%d", res.ID), nil, ck))
	te.do(http.MethodPost, fmt.Sprintf("/api/v1/till/sales/%d/return", res.ID), ReturnRequest{Reason: "wrong size", Lines: []ReturnLine{{SaleItemID: v.Lines[0].ID, Quantity: "1"}}}, ck)
	if d := data(fmt.Sprintf("/api/v1/till/sales/%d/refund-print?data=1", res.ID)); d.Kind != "refund" || d.Refund == nil ||
		d.Refund.ReceiptNo != res.ReceiptNo || d.Refund.Refund != "100.00" || d.Refund.Reason != "wrong size" || len(d.Refund.Items) != 1 {
		t.Fatalf("refund: %+v %+v", d, d.Refund)
	}

	// Credit payment → a debt slip and a CR- money receipt.
	var cid int64
	name := fmt.Sprintf("T slip %d", sfx)
	te.db.Get(&cid, `INSERT INTO customers (name, credit_limit, outstanding_balance) VALUES ($1, 5000, 500) RETURNING id`, name) //nolint:errcheck
	pay := decode[CreditPaymentResult](t, te.do(http.MethodPost, fmt.Sprintf("/api/v1/till/customers/%d/payment", cid), CreditPaymentRequest{Amount: "200", Method: "cash"}, ck))
	if d := data(fmt.Sprintf("/api/v1/till/customer-payments/%d/print?data=1", pay.PaymentID)); d.Kind != "debt" || d.Debt == nil ||
		d.Debt.ReceiptNo != pay.ReceiptNo || d.Debt.Amount != "200.00" || d.Debt.BalanceAfter != "300.00" || d.Debt.Method != "Cash" {
		t.Fatalf("debt: %+v %+v", d, d.Debt)
	}
	mr := decode[[]MoneyReceiptRow](t, te.do(http.MethodGet, "/api/v1/till/money-receipts?q="+url.QueryEscape(name), nil, ck))
	if len(mr) == 0 {
		t.Fatal("no money receipt for the collection")
	}
	if d := data(fmt.Sprintf("/api/v1/till/money-receipts/%d/print?data=1", mr[0].ID)); d.Kind != "money" || d.Money == nil ||
		d.Money.ReceiptNo != mr[0].ReceiptNo || d.Money.Amount != "200.00" || d.Money.Title != mr[0].KindLabel {
		t.Fatalf("money: %+v %+v", d, d.Money)
	}

	// Warranty replacement.
	oldSN := fmt.Sprintf("SN-OLD-%d", sfx)
	te.db.Exec(`INSERT INTO warranty_units (product_id, serial_no, sold_at, warranty_months, warranty_until, source)
		VALUES ($1, $2, now(), 12, (now() + interval '1 year')::date, 'sale')`, f.plainID, oldSN) //nolint:errcheck
	u := decode[WarrantyUnitView](t, te.do(http.MethodGet, "/api/v1/till/warranty/serial/"+url.PathEscape(oldSN), nil, ck))
	claim := decode[WarrantyClaimRef](t, te.do(http.MethodPost, "/api/v1/till/warranty/replace", WarrantyReplaceRequest{UnitID: u.ID, NewSerial: oldSN + "-N", Reason: "dead"}, ck))
	if d := data(fmt.Sprintf("/api/v1/till/warranty-claims/%d/print?data=1", claim.ClaimID)); d.Kind != "warranty" || d.Warranty == nil ||
		d.Warranty.OldSerial != oldSN || d.Warranty.NewSerial != oldSN+"-N" || d.Warranty.WarrantyUntil == "" {
		t.Fatalf("warranty: %+v %+v", d, d.Warranty)
	}

	// The settings test slip.
	admin := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "admin")
	rec := te.do(http.MethodPost, "/api/v1/admin/settings/printer/test?data=1", nil, admin)
	if d := decode[SlipData](t, rec); rec.Code != http.StatusOK || d.Kind != "test" || d.Test == nil || d.Test.Printed == "" {
		t.Fatalf("test: %d %s", rec.Code, rec.Body)
	}

	// The Settings preview: a made-up sale with the shop's real header and footer;
	// it never prints.
	te.db.Exec(`UPDATE settings SET receipt_footer = 'Sample footer'`)            //nolint:errcheck
	t.Cleanup(func() { te.db.Exec(`UPDATE settings SET receipt_footer = NULL`) }) //nolint:errcheck
	rec = te.do(http.MethodPost, "/api/v1/admin/settings/sample-slip?data=1", nil, admin)
	if d := decode[SlipData](t, rec); rec.Code != http.StatusOK || d.Kind != "sale" || d.Sale == nil || len(d.Sale.Items) != 2 || d.Shop.Footer != "Sample footer" {
		t.Fatalf("sample data: %d %s", rec.Code, rec.Body)
	}
	rec = te.do(http.MethodPost, "/api/v1/admin/settings/sample-slip?preview=1", nil, admin)
	if p := decode[SlipPreview](t, rec); rec.Code != http.StatusOK || !strings.Contains(fmt.Sprint(p.Lines), "Sample footer") {
		t.Fatalf("sample preview: %d %s", rec.Code, rec.Body)
	}
	if rec := te.do(http.MethodPost, "/api/v1/admin/settings/sample-slip", nil, admin); rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("the sample never prints: %d", rec.Code)
	}
}
