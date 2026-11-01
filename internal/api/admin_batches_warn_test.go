package api

import (
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"
)

// A lot warns by its supplier's days: the delivery's supplier, else the
// product's preferred one; a supplier left blank, or no supplier, follows the shop.
func TestExpiryWarningDays(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "admin")
	var shop int
	te.db.Get(&shop, `SELECT expiry_warn_days FROM settings WHERE id=1`)                         //nolint:errcheck
	t.Cleanup(func() { te.db.Exec(`UPDATE settings SET expiry_warn_days=$1 WHERE id=1`, shop) }) //nolint:errcheck
	te.db.MustExec(`UPDATE settings SET expiry_warn_days=20 WHERE id=1`)

	sfx := time.Now().UnixNano()
	sixty := 60
	long := decode[AdminSupplierRow](t, te.do(http.MethodPost, "/api/v1/admin/suppliers", SupplierRequest{Name: fmt.Sprintf("T Warn 60 %d", sfx), ExpiryWarnDays: &sixty}, ck))
	blank := decode[AdminSupplierRow](t, te.do(http.MethodPost, "/api/v1/admin/suppliers", SupplierRequest{Name: fmt.Sprintf("T Warn blank %d", sfx)}, ck))
	if long.ExpiryWarnDays == nil || *long.ExpiryWarnDays != 60 || blank.ExpiryWarnDays != nil {
		t.Fatalf("saved days: %+v %+v", long, blank)
	}

	lot := func(product, days int) int64 {
		var id int64
		if err := te.db.Get(&id, `INSERT INTO stock_batches (product_id, qty_received, qty_remaining, cost_price, expiry_date, source)
			VALUES ($1, 1, 1, 50, CURRENT_DATE + $2::int, 'adjust') RETURNING id`, product, days); err != nil {
			t.Fatal(err)
		}
		return id
	}
	te.db.MustExec(`UPDATE products SET preferred_supplier_id=$1 WHERE id=$2`, long.ID, f.plainID)
	te.db.MustExec(`UPDATE products SET preferred_supplier_id=$1 WHERE id=$2`, blank.ID, f.scarceID)
	inLong, outLong := lot(int(f.plainID), 45), lot(int(f.plainID), 90)
	outBlank, inBlank := lot(int(f.scarceID), 45), lot(int(f.scarceID), 15)
	outNone, inNone := lot(int(f.lotID), 45), lot(int(f.lotID), 15)

	// Delivered by the 60-day supplier: that beats the product's preferred (blank) one.
	var buy, item int64
	te.db.Get(&buy, `INSERT INTO purchases (supplier_id) VALUES ($1) RETURNING id`, long.ID) //nolint:errcheck
	te.db.Get(&item, `INSERT INTO purchase_items (purchase_id, product_id, quantity, cost_price, selling_price, subtotal)
		VALUES ($1, $2, 1, 50, 100, 50) RETURNING id`, buy, f.scarceID) //nolint:errcheck
	delivered := lot(int(f.scarceID), 50)
	te.db.MustExec(`UPDATE stock_batches SET purchase_item_id=$1 WHERE id=$2`, item, delivered)

	v := decode[BatchesView](t, te.do(http.MethodGet, "/api/v1/admin/stock/batches", nil, ck))
	group := func(id int64) BatchGroup {
		i := slices.IndexFunc(v.Groups, func(g BatchGroup) bool { return g.ProductID == id })
		if i < 0 {
			t.Fatalf("no group for %d", id)
		}
		return v.Groups[i]
	}
	row := func(g BatchGroup, id int64) LotRow {
		i := slices.IndexFunc(g.Lots, func(l LotRow) bool { return l.ID == id })
		if i < 0 {
			t.Fatalf("no lot %d in %+v", id, g)
		}
		return g.Lots[i]
	}
	for _, c := range []struct {
		name     string
		product  int64
		lot      int64
		days     int
		supplier string
		expiring int
	}{
		{"own days, inside", f.plainID, inLong, 60, long.Name, 1},
		{"own days, outside", f.plainID, outLong, 60, long.Name, 1},
		{"blank supplier, outside", f.scarceID, outBlank, 20, blank.Name, 2},
		{"blank supplier, inside", f.scarceID, inBlank, 20, blank.Name, 2},
		{"delivery beats preferred", f.scarceID, delivered, 60, long.Name, 2},
		{"no supplier, outside", f.lotID, outNone, 20, "", 1},
		{"no supplier, inside", f.lotID, inNone, 20, "", 1},
	} {
		g := group(c.product)
		l := row(g, c.lot)
		if l.WarnDays != c.days || l.Supplier != c.supplier || g.ExpiringSoon != c.expiring {
			t.Errorf("%s: days %d supplier %q group expiring %d", c.name, l.WarnDays, l.Supplier, g.ExpiringSoon)
		}
	}

	// The dashboard counts by the same rule.
	d := decode[Dashboard](t, te.do(http.MethodGet, "/api/v1/admin/dashboard", nil, ck))
	if d.Attention.Expiring != v.ExpiringSoon || d.Attention.Expired != v.Expired {
		t.Errorf("dashboard %d/%d, batches %d/%d", d.Attention.Expiring, d.Attention.Expired, v.ExpiringSoon, v.Expired)
	}

	// Blank again follows the shop; out of range is refused.
	name := long.Name
	if rec := te.do(http.MethodPut, fmt.Sprintf("/api/v1/admin/suppliers/%d", long.ID), SupplierRequest{Name: name}, ck); rec.Code >= 300 {
		t.Fatalf("clear: %d %s", rec.Code, rec.Body)
	}
	v = decode[BatchesView](t, te.do(http.MethodGet, "/api/v1/admin/stock/batches", nil, ck))
	if l := row(group(f.plainID), inLong); l.WarnDays != 20 {
		t.Errorf("cleared supplier follows the shop: %d", l.WarnDays)
	}
	bad := 5000
	if rec := te.do(http.MethodPut, fmt.Sprintf("/api/v1/admin/suppliers/%d", long.ID), SupplierRequest{Name: name, ExpiryWarnDays: &bad}, ck); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("too many days: %d", rec.Code)
	}
}
