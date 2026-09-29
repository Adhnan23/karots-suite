package api

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestCounterWarranty(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "cashier")
	sfx := time.Now().UnixNano()

	// Serial path: a sold unit with a year of cover is swapped for a new serial.
	oldSN := fmt.Sprintf("SN-OLD-%d", sfx)
	if _, err := te.db.Exec(`INSERT INTO warranty_units (product_id, serial_no, sold_at, warranty_months, warranty_until, source)
		VALUES ($1, $2, now(), 12, (now() + interval '1 year')::date, 'sale')`, f.plainID, oldSN); err != nil {
		t.Fatal(err)
	}
	u := decode[WarrantyUnitView](t, te.do(http.MethodGet, "/api/v1/till/warranty/serial/"+url.PathEscape(oldSN), nil, ck))
	if !u.UnderWarranty || u.Months != 12 || u.Left == "expired" || len(u.Claims) != 0 {
		t.Fatalf("lookup: %+v", u)
	}
	newSN := fmt.Sprintf("SN-NEW-%d", sfx)
	rec := te.do(http.MethodPost, "/api/v1/till/warranty/replace", WarrantyReplaceRequest{UnitID: u.ID, NewSerial: newSN, Reason: "dead on arrival"}, ck)
	if rec.Code != http.StatusCreated || decode[WarrantyClaimRef](t, rec).ClaimID == 0 {
		t.Fatalf("replace: %d %s", rec.Code, rec.Body)
	}
	if nu := decode[WarrantyUnitView](t, te.do(http.MethodGet, "/api/v1/till/warranty/serial/"+url.PathEscape(newSN), nil, ck)); nu.Until != u.Until {
		t.Fatalf("the new unit continues the old cover: %s vs %s", nu.Until, u.Until)
	}
	if rec := te.do(http.MethodPost, "/api/v1/till/warranty/replace", WarrantyReplaceRequest{UnitID: u.ID, NewSerial: newSN + "x"}, ck); rec.Code != http.StatusConflict {
		t.Fatalf("replacing a replaced unit: %d", rec.Code)
	}
	claims := decode[[]WarrantyClaimRow](t, te.do(http.MethodGet, "/api/v1/till/warranty-claims?q="+url.QueryEscape(oldSN), nil, ck))
	if len(claims) != 1 || claims[0].ReplacementSerial != newSN {
		t.Fatalf("claims: %+v", claims)
	}
	if rec := te.do(http.MethodGet, "/api/v1/till/warranty/serial/NOPE-"+fmt.Sprint(sfx), nil, ck); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown serial: %d", rec.Code)
	}

	// Receipt path: a warranted product with no serial, claimed by its receipt.
	te.db.Exec(`UPDATE products SET warranty_months = 6 WHERE id = $1`, f.lotID) //nolint:errcheck
	res := decode[SaleResult](t, te.do(http.MethodPost, "/api/v1/till/sales", sale(f.lotID, "2", PaymentRequest{Method: "cash", Amount: "200"}), ck))
	rv := decode[WarrantyReceiptView](t, te.do(http.MethodGet, "/api/v1/till/warranty/receipt?no="+url.QueryEscape(res.ReceiptNo), nil, ck))
	if len(rv.Lines) != 1 || !rv.Lines[0].UnderWarranty || rv.Lines[0].Remaining != "2" {
		t.Fatalf("receipt lookup: %+v", rv)
	}
	if rec := te.do(http.MethodPost, "/api/v1/till/warranty/receipt-replace", WarrantyReceiptReplaceRequest{SaleID: rv.SaleID, ProductID: f.lotID, Reason: "cracked"}, ck); rec.Code != http.StatusCreated {
		t.Fatalf("receipt replace: %d %s", rec.Code, rec.Body)
	}
	if rv := decode[WarrantyReceiptView](t, te.do(http.MethodGet, "/api/v1/till/warranty/receipt?no="+url.QueryEscape(res.ReceiptNo), nil, ck)); rv.Lines[0].Replaced != 1 || rv.Lines[0].Remaining != "1" {
		t.Fatalf("after receipt replace: %+v", rv.Lines)
	}
}
