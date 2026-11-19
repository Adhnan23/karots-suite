package battle

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"
)

// upload posts a file as a form (a sheet import), as the screen does.
func (c *client) upload(path, name string, data []byte, out any) error {
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	f, err := w.CreateFormFile("file", name)
	if err != nil {
		return err
	}
	f.Write(data) //nolint:errcheck
	w.Close()     //nolint:errcheck
	req := httptest.NewRequest("POST", path, &b)
	req.RemoteAddr = "127.0.0.1:1"
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("X-Ks-Device", c.device)
	for _, k := range c.cookie {
		req.AddCookie(k)
	}
	rec := httptest.NewRecorder()
	c.h.ServeHTTP(rec, req)
	if rec.Code < 200 || rec.Code > 299 {
		return fmt.Errorf("POST %s: %d %s", path, rec.Code, bytes.TrimSpace(rec.Body.Bytes()))
	}
	return nil
}

// refused asserts the call failed and changed nothing the auditor can see.
func (s *shop) mustRefuse(t *testing.T, what string, err error) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: went through, want refused", what)
	}
}

func (s *shop) onHand(t *testing.T, id int64) string {
	t.Helper()
	var q string
	s.must(t, s.db.Get(&q, `SELECT quantity::numeric(14,3)::text FROM stock WHERE product_id = $1`, id))
	return q
}

func (s *shop) lotLeft(t *testing.T, id int64) string {
	t.Helper()
	var q string
	s.must(t, s.db.Get(&q, `SELECT qty_remaining::numeric(14,3)::text FROM stock_batches WHERE id = $1`, id))
	return q
}

// STK-17: a pack holds a whole number (above 1) of a whole unit.
func TestSTK17PackHoldsWholeUnits(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	p := s.stocked(t, "Tablets", s.unitPcs, 0, nil)
	s.mustRefuse(t, "a pack of 2.5", s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/products/%d/packs", p),
		map[string]string{"name": "Half", "size": "2.5", "sellingPrice": "50"}, nil))
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/products/%d/packs", p),
		map[string]string{"name": "Card", "size": "10", "sellingPrice": "180"}, nil))
	s.audit(t)
}

// STK-18: a whole-unit product never takes a fraction — counted, added, written
// off or converted into; a decimal one does.
func TestSTK18WholeUnitsNeverFractions(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	pcs := s.stocked(t, "Soap", s.unitPcs, 0, nil)
	kg := s.stocked(t, "Dhal", s.unitKg, 0, nil)
	s.addLot(t, pcs, "10", "40", "")
	s.addLot(t, kg, "10", "300", "")
	s.mustRefuse(t, "count 2.5 pcs", s.admin.Do("POST", "/api/v1/admin/stock/adjust", map[string]any{"productId": pcs, "newQuantity": "2.5"}, nil))
	s.mustRefuse(t, "add 1.5 pcs", s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/stock/lots/%d/add", pcs), map[string]string{"qty": "1.5"}, nil))
	s.mustRefuse(t, "write off 0.5 pcs", s.admin.Do("POST", "/api/v1/admin/stock/damage", map[string]any{"productId": pcs, "quantity": "0.5", "reason": "damage"}, nil))
	box := s.stocked(t, "Soap box", s.unitPcs, 0, nil)
	s.addLot(t, box, "2", "450", "")
	var cv struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/conversions", map[string]any{"fromProductId": box, "toProductId": pcs, "ratio": "12"}, &cv))
	s.mustRefuse(t, "11.5 soaps out of a box", s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/conversions/%d/run", cv.ID),
		map[string]string{"quantity": "1", "toQty": "11.5"}, nil))
	s.mustRefuse(t, "half a box", s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/conversions/%d/run", cv.ID), map[string]string{"quantity": "0.5"}, nil))
	if got := s.onHand(t, pcs); got != "10.000" {
		t.Errorf("soap on hand %s after refusals, want 10", got)
	}
	// Decimal units take fractions everywhere.
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/adjust", map[string]any{"productId": kg, "newQuantity": "9.75"}, nil))
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/damage", map[string]any{"productId": kg, "quantity": "0.25", "reason": "damage"}, nil))
	if got := s.onHand(t, kg); got != "9.500" {
		t.Errorf("dhal on hand %s, want 9.5", got)
	}
	s.audit(t)
}

// STK-18 (user, Q3): a unit can't lose its decimals, nor a product move to a whole
// unit, while it holds a fraction; a unit whose products have packs can't take decimals.
func TestSTK18UnitChangesWhileFractions(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	var gram, box struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/units", map[string]any{"name": "Battle gram", "abbreviation": "bgm", "allowDecimal": true}, &gram))
	s.must(t, s.admin.Do("POST", "/api/v1/admin/units", map[string]any{"name": "Battle box", "abbreviation": "bbx"}, &box))
	spice := s.stocked(t, "Spice", gram.ID, 0, nil)
	s.addLot(t, spice, "2.5", "4", "")
	s.mustRefuse(t, "untick decimals on g while 2.5 g held", s.admin.Do("PUT", fmt.Sprintf("/api/v1/admin/units/%d", gram.ID),
		map[string]any{"name": "Battle gram", "abbreviation": "bgm", "allowDecimal": false}, nil))
	move := map[string]any{"name": "Spice", "categoryId": s.cat, "unitId": box.ID, "costPrice": "10", "sellingPrice": "20",
		"wholesalePrice": "20", "reorderAuto": false}
	s.mustRefuse(t, "move 2.5 g to boxes", s.admin.Do("PUT", fmt.Sprintf("/api/v1/admin/products/%d", spice), move, nil))
	boxed := s.stocked(t, "Boxed", box.ID, 0, map[string]any{"packs": []map[string]string{{"name": "Crate", "size": "6", "sellingPrice": "100"}}})
	_ = boxed
	s.mustRefuse(t, "tick decimals on a unit with packs", s.admin.Do("PUT", fmt.Sprintf("/api/v1/admin/units/%d", box.ID),
		map[string]any{"name": "Battle box", "abbreviation": "bbx", "allowDecimal": true}, nil))
	// Counted whole, the change goes through.
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/adjust", map[string]any{"productId": spice, "newQuantity": "2"}, nil))
	s.must(t, s.admin.Do("PUT", fmt.Sprintf("/api/v1/admin/products/%d", spice), move, nil))
	s.audit(t)
}

// STK-7, STK-24: a sheet's opening stock is one lot and one adjust movement worth
// the units at the sheet's cost.
func TestSTK24SheetOpeningWorth(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	sheet := "Product,Category Path,Cost,PRICE,Opening\nSheet Rice,Battle,40,55,5\n"
	s.must(t, s.admin.upload("/api/v1/admin/products/import", "p.csv", []byte(sheet), nil))
	var m struct {
		Qty, Cost string
		Lots      int
	}
	s.must(t, s.db.Get(&m, `SELECT m.quantity::numeric(14,2)::text AS qty, m.cost::numeric(14,2)::text AS cost,
		(SELECT count(*) FROM stock_batches b WHERE b.product_id = p.id) AS lots
		FROM products p JOIN stock_movements m ON m.product_id = p.id WHERE p.name = 'Sheet Rice'`))
	if m.Qty != "5.00" || m.Cost != "200.00" || m.Lots != 1 {
		t.Errorf("opening movement %+v, want 5 worth 200 in one lot", m)
	}
	s.audit(t)
}

// STK-21 (user, Q1): with no lot named, good lots go first; expired and recalled
// (stop selling) lots only when nothing else is left.
func TestSTK21GoodLotsFirst(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	var sup struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/suppliers", map[string]string{"name": "Lot Supplier", "phone": "0112000002"}, &sup))
	p := s.stocked(t, "Syrup", s.unitPcs, sup.ID, nil)
	s.addLot(t, p, "5", "50", "2026-01-31") // expired
	s.addLot(t, p, "5", "50", "2027-02-28") // recalled below
	s.addLot(t, p, "5", "50", "2027-06-30") // good
	s.addLot(t, p, "5", "50", "")           // good, no expiry
	var ids []int64
	s.must(t, s.db.Select(&ids, `SELECT id FROM stock_batches WHERE product_id = $1 ORDER BY id`, p))
	expired, recalled, good, undated := ids[0], ids[1], ids[2], ids[3]
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/suppliers/%d/claims/lots/%d/recall", sup.ID, recalled), map[string]bool{"stopSelling": true}, nil))
	s.open(t, s.till1)
	s.sell(t, s.till1, "0", []line{{"productId": p, "quantity": "7"}}, cash("140"))
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/damage", map[string]any{"productId": p, "quantity": "2", "reason": "staff"}, nil))
	for id, want := range map[int64]string{good: "0.000", undated: "1.000", expired: "5.000", recalled: "5.000"} {
		if got := s.lotLeft(t, id); got != want {
			t.Errorf("lot %d left %s, want %s (good lots first)", id, got, want)
		}
	}
	// Nothing good left: expired, then recalled, as FEFO.
	s.sell(t, s.till1, "0", []line{{"productId": p, "quantity": "7"}}, cash("140"))
	for id, want := range map[int64]string{undated: "0.000", expired: "0.000", recalled: "4.000"} {
		if got := s.lotLeft(t, id); got != want {
			t.Errorf("then lot %d left %s, want %s", id, got, want)
		}
	}
	s.audit(t)
}

// STK-15 (user, Q2): a recalled lot can't be merged.
func TestSTK15NoMergingARecalledLot(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	var sup struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/suppliers", map[string]string{"name": "Merge Supplier", "phone": "0112000003"}, &sup))
	p := s.stocked(t, "Drops", s.unitPcs, sup.ID, nil)
	s.addLot(t, p, "4", "30", "2027-01-31")
	s.addLot(t, p, "6", "40", "2027-03-31")
	var ids []int64
	s.must(t, s.db.Select(&ids, `SELECT id FROM stock_batches WHERE product_id = $1 ORDER BY id`, p))
	recall := fmt.Sprintf("/api/v1/admin/suppliers/%d/claims/lots/%d/recall", sup.ID, ids[1])
	s.must(t, s.admin.Do("POST", recall, map[string]bool{"stopSelling": false}, nil))
	merge := fmt.Sprintf("/api/v1/admin/stock/lots/%d/merge", p)
	err := s.admin.Do("POST", merge, map[string]any{"batchIds": ids}, nil)
	s.mustRefuse(t, "merge a recalled lot", err)
	if err != nil && !strings.Contains(err.Error(), "recall") {
		t.Errorf("refusal should say why: %v", err)
	}
	s.must(t, s.admin.Do("DELETE", recall, nil, nil))
	// The prompt sends the expiry it shows (the earliest); a blank cost is the value-weighted one.
	s.must(t, s.admin.Do("POST", merge, map[string]any{"batchIds": ids, "expiry": "2027-01-31"}, nil))
	// (4×30 + 6×40) / 10 = 36.
	var kept struct {
		Qty, Cost, Expiry string
	}
	s.must(t, s.db.Get(&kept, `SELECT qty_remaining::numeric(14,0)::text AS qty, cost_price::numeric(14,2)::text AS cost, expiry_date::text AS expiry
		FROM stock_batches WHERE id = $1`, ids[0]))
	if kept.Qty != "10" || kept.Cost != "36.00" || kept.Expiry != "2027-01-31" {
		t.Errorf("merged lot %+v, want 10 at 36.00 expiring 2027-01-31", kept)
	}
	s.audit(t)
}
