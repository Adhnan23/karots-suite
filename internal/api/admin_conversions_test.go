package api

import (
	"net/http"
	"slices"
	"strconv"
	"testing"
)

func TestAdminConversions(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	const phone = "0770009591"
	clean := func() {
		te.db.Exec(`DELETE FROM conversion_runs WHERE created_by IN (SELECT id FROM users WHERE phone=$1)`, phone) //nolint:errcheck
		te.db.Exec(`DELETE FROM stock_movements WHERE user_id IN (SELECT id FROM users WHERE phone=$1)`, phone)    //nolint:errcheck
	}
	clean()
	ck := te.login(t, phone, "admin")
	t.Cleanup(clean)

	if rec := te.do(http.MethodPost, "/api/v1/admin/conversions", ConversionRequest{FromProductID: f.plainID, ToProductID: f.plainID, Ratio: "2"}, ck); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("self conversion: %d", rec.Code)
	}
	rec := te.do(http.MethodPost, "/api/v1/admin/conversions", ConversionRequest{FromProductID: f.plainID, ToProductID: f.scarceID, Ratio: "12", Note: "box of 12"}, ck)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	id := decode[Option](t, rec).ID
	t.Cleanup(func() { te.db.Exec(`DELETE FROM product_conversions WHERE id=$1`, id) }) //nolint:errcheck
	base := "/api/v1/admin/conversions/" + strconv.FormatInt(id, 10)

	if rec := te.do(http.MethodPost, base+"/run", RunConversionRequest{Quantity: "1"}, ck); rec.Code != http.StatusNoContent {
		t.Fatalf("run: %d %s", rec.Code, rec.Body)
	}
	var scarce float64
	te.db.Get(&scarce, `SELECT quantity FROM stock WHERE product_id=$1`, f.scarceID) //nolint:errcheck
	if scarce != 13 {
		t.Fatalf("1 box → 12 more: have %v", scarce)
	}
	// What was really counted out: 11 of the 12 (blank "Got" = 12, above).
	if rec := te.do(http.MethodPost, base+"/run", RunConversionRequest{Quantity: "1", ToQty: "11"}, ck); rec.Code != http.StatusNoContent {
		t.Fatalf("run with got: %d %s", rec.Code, rec.Body)
	}
	if rec := te.do(http.MethodPost, base+"/run", RunConversionRequest{Quantity: "1", ToQty: "-2"}, ck); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("negative got: %d", rec.Code)
	}
	runs := decode[ConversionRunList](t, te.do(http.MethodGet, "/api/v1/admin/conversions/runs?conversion="+strconv.FormatInt(id, 10)+"&preset=today", nil, ck))
	if runs.Total != 2 || runs.Items[0].ToQty != "11" || runs.Items[1].ToQty != "12" {
		t.Fatalf("runs: %+v", runs)
	}

	te.do(http.MethodPut, base, ConversionRequest{Ratio: "10"}, ck)
	te.do(http.MethodDelete, base, nil, ck)
	has := func(q string) (AdminConversion, bool) {
		rows := decode[[]AdminConversion](t, te.do(http.MethodGet, "/api/v1/admin/conversions"+q, nil, ck))
		i := slices.IndexFunc(rows, func(r AdminConversion) bool { return r.ID == id })
		if i < 0 {
			return AdminConversion{}, false
		}
		return rows[i], true
	}
	if _, ok := has(""); ok {
		t.Fatal("disabled conversion still listed")
	}
	if cv, ok := has("?disabled=1"); !ok || cv.IsActive || cv.Ratio != "10" {
		t.Fatalf("show disabled: %+v %v", cv, ok)
	}
}
