package battle

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// STK-11 (make loose version): the loose product, its conversion and the first
// run in one go; a bad request leaves nothing behind. The conversion can then be
// edited, turned off and brought back, and its run is in the history.
func TestSTK11MakeLoose(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	bag := s.stocked(t, "Rice sack", s.unitPcs, 0, nil)
	s.addLot(t, bag, "2", "2500", "2027-03-31")
	loose := func(name string, unit int64, ratio, now string) error {
		return s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/products/%d/loose", bag), map[string]any{
			"name": name, "unitId": unit, "ratio": ratio, "sellingPrice": "120", "barcode": "LOOSE-1", "convertNow": now}, nil)
	}
	s.mustRefuse(t, "no name", loose(" ", s.unitKg, "25", "1"))
	s.mustRefuse(t, "the sack's own unit", loose("Rice", s.unitPcs, "25", "1"))
	s.mustRefuse(t, "ratio 0", loose("Rice", s.unitKg, "0", "1"))
	s.mustRefuse(t, "more sacks than there are", loose("Rice", s.unitKg, "25", "3"))
	var n int
	s.must(t, s.db.Get(&n, `SELECT count(*) FROM products WHERE name = 'Rice'`))
	if n != 0 {
		t.Fatalf("a refused loose version left %d products behind", n)
	}
	s.must(t, loose("Rice", s.unitKg, "25", "1"))
	var p struct {
		ID      int64  `db:"id"`
		Barcode string `db:"barcode"`
		Cost    string `db:"cost"`
		Price   string `db:"price"`
	}
	// The product's own cost is the sack's ÷ 25 (10 → 0.40); the lot carries what the sack really cost.
	s.must(t, s.db.Get(&p, `SELECT p.id, COALESCE(p.barcode, '') AS barcode, b.cost_price::numeric(9,2)::text AS cost, p.selling_price::numeric(9,2)::text AS price
		FROM products p JOIN stock_batches b ON b.product_id = p.id WHERE p.name = 'Rice'`))
	if s.onHand(t, p.ID) != "25.000" || s.onHand(t, bag) != "1.000" || p.Barcode != "LOOSE-1" || p.Cost != "100.00" || p.Price != "120.00" {
		t.Errorf("loose version %+v holds %s, sacks %s; want 25 kg in a lot at 100, barcode LOOSE-1, 1 sack", p, s.onHand(t, p.ID), s.onHand(t, bag))
	}
	var cv int64
	s.must(t, s.db.Get(&cv, `SELECT id FROM product_conversions WHERE from_product_id = $1`, bag))
	list := func(q string) string {
		var rows []struct{ ID int64 }
		s.must(t, s.admin.Do("GET", "/api/v1/admin/conversions"+q, nil, &rows))
		var ids []string
		for _, r := range rows {
			ids = append(ids, fmt.Sprint(r.ID))
		}
		return strings.Join(ids, ",")
	}
	other := s.stocked(t, "Oil drum", s.unitPcs, 0, nil)
	oil := s.stocked(t, "Oil loose", s.unitKg, 0, nil)
	s.must(t, s.admin.Do("POST", "/api/v1/admin/conversions", map[string]any{"fromProductId": other, "toProductId": oil, "ratio": "200"}, nil))
	if got := list("?q=Rice%20sack"); got != fmt.Sprint(cv) {
		t.Errorf("searching conversions for the sack lists %q, want only %d", got, cv)
	}
	s.must(t, s.admin.Do("PUT", fmt.Sprintf("/api/v1/admin/conversions/%d", cv), map[string]any{"ratio": "24", "note": "smaller sacks"}, nil))
	var ratio string
	s.must(t, s.db.Get(&ratio, `SELECT ratio::numeric(9,0)::text FROM product_conversions WHERE id = $1`, cv))
	s.must(t, s.admin.Do("DELETE", fmt.Sprintf("/api/v1/admin/conversions/%d", cv), nil, nil))
	off, all := list("?q=Rice%20sack"), list("?disabled=1&q=Rice%20sack")
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/conversions/%d/activate", cv), nil, nil))
	if ratio != "24" || off != "" || all != fmt.Sprint(cv) || list("?q=Rice%20sack") != fmt.Sprint(cv) {
		t.Errorf("edit / off / back: ratio %s, listed %q off, %q with disabled, %q back", ratio, off, all, list("?q=Rice%20sack"))
	}
	var runs struct{ Total int }
	s.must(t, s.admin.Do("GET", "/api/v1/admin/conversions/runs", nil, &runs))
	if runs.Total != 1 {
		t.Errorf("run history holds %d runs, want the first one", runs.Total)
	}
	s.audit(t)
}

// STK-20: an expiry cleared is none; a lot that isn't there is refused.
func TestSTK20ExpiryEdits(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	p := s.stocked(t, "Torch", s.unitPcs, 0, nil)
	s.addLot(t, p, "3", "10", "")
	ids := s.lotIDs(t, p)
	exp := fmt.Sprintf("/api/v1/admin/stock/batches/%d/expiry", ids[0])
	s.must(t, s.admin.Do("POST", exp, map[string]string{"expiry": "2027-04-30"}, nil))
	s.must(t, s.admin.Do("POST", exp, map[string]string{"expiry": ""}, nil))
	var none bool
	s.must(t, s.db.Get(&none, `SELECT expiry_date IS NULL FROM stock_batches WHERE id = $1`, ids[0]))
	if !none {
		t.Errorf("a cleared expiry is still set")
	}
	s.mustRefuse(t, "expiry of a lot that isn't there", s.admin.Do("POST", "/api/v1/admin/stock/batches/999999/expiry", map[string]string{"expiry": "2027-01-31"}, nil))
	s.audit(t)
}

// STK-10, STK-12, STK-22, STK-21: the screens that read stock show the books — the
// Frozen list, a supplier's claims, the stock-take page's lots, the till's
// "which lot?" list and the batch report.
func TestStockScreensShowTheBooks(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	sup := s.supplier(t, "Screen Supplier", "0112000007")
	p := s.stocked(t, "Screen syrup", s.unitPcs, sup, nil)
	s.addLot(t, p, "4", "50", s.daysFromToday(-10)) // expired
	s.addLot(t, p, "6", "50", "2027-06-30")
	ids := s.lotIDs(t, p)
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/suppliers/%d/claims/lots/%d/recall", sup, ids[1]), map[string]bool{"stopSelling": true}, nil))
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/freeze", map[string]any{"productIds": []int64{p}}, nil))

	var frozen struct {
		Products []struct{ ID int64 }
	}
	s.must(t, s.admin.Do("GET", "/api/v1/admin/stock/frozen", nil, &frozen))
	var claims struct {
		Recalled, Expired []struct{ LotID int64 }
	}
	s.must(t, s.admin.Do("GET", fmt.Sprintf("/api/v1/admin/suppliers/%d/claims", sup), nil, &claims))
	var take struct {
		Items []struct {
			ID   int64
			Lots []struct{ ID int64 }
		}
	}
	s.must(t, s.admin.Do("GET", "/api/v1/admin/stock/take?q=Screen%20syrup", nil, &take))
	var prompt map[string][]struct {
		BatchID int64 `json:"batchId"`
	}
	s.must(t, s.till1.Do("GET", "/api/v1/till/price-options", nil, &prompt))
	var moves struct {
		Total int
		Items []struct{ Type string }
	}
	s.must(t, s.admin.Do("GET", fmt.Sprintf("/api/v1/admin/stock/movements?product=%d", p), nil, &moves))
	var sales struct{ Total int }
	s.must(t, s.admin.Do("GET", fmt.Sprintf("/api/v1/admin/stock/movements?product=%d&type=sale", p), nil, &sales))
	if moves.Total != 2 || len(moves.Items) != 2 || moves.Items[0].Type != "adjust" || sales.Total != 0 {
		t.Errorf("movements screen: %+v, %d sales; want the 2 lots added, no sales", moves, sales.Total)
	}
	var report struct{ TotalValue string }
	s.must(t, s.admin.Do("GET", "/api/v1/admin/stock/batches", nil, &report))

	if len(frozen.Products) != 1 || frozen.Products[0].ID != p {
		t.Errorf("Frozen list %+v, want the syrup", frozen.Products)
	}
	if len(claims.Recalled) != 1 || claims.Recalled[0].LotID != ids[1] || len(claims.Expired) != 1 || claims.Expired[0].LotID != ids[0] {
		t.Errorf("claims %+v, want lot %d recalled and lot %d expired", claims, ids[1], ids[0])
	}
	if len(take.Items) != 1 || len(take.Items[0].Lots) != 2 {
		t.Errorf("stock-take page %+v, want the syrup with its 2 lots", take.Items)
	}
	if len(prompt[fmt.Sprint(p)]) != 2 {
		t.Errorf("till's lot prompt for the syrup %+v, want both lots", prompt[fmt.Sprint(p)])
	}
	if report.TotalValue == "" || dec(t, report.TotalValue).IsZero() {
		t.Errorf("batch report value %q, want the shop's lots", report.TotalValue)
	}
	s.audit(t)
}

// daysFromToday is a date n days from today, YYYY-MM-DD.
func (s *shop) daysFromToday(n int) string {
	return time.Now().AddDate(0, 0, n).Format(time.DateOnly)
}

// STK-8, STK-21: Write off all expired takes every expired lot out as damage, one
// write-off each, and leaves good lots alone.
func TestSTK8WriteOffAllExpired(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	p := s.stocked(t, "Old milk", s.unitPcs, 0, nil)
	s.addLot(t, p, "3", "40", s.daysFromToday(-5))
	s.addLot(t, p, "4", "40", s.daysFromToday(60))
	ids := s.lotIDs(t, p)
	var r struct{ Lots int }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/expired/write-off-all", nil, &r))
	if r.Lots != 1 || s.lotLeft(t, ids[0]) != "0.000" || s.lotLeft(t, ids[1]) != "4.000" {
		t.Errorf("wrote off %d lots; expired lot left %s, good lot %s (want 1, 0, 4)", r.Lots, s.lotLeft(t, ids[0]), s.lotLeft(t, ids[1]))
	}
	if typ, q, w := s.lastMove(t, p); typ != "damage" || !q.Equal(decimal.NewFromInt(-3)) || !w.Equal(decimal.NewFromInt(120)) {
		t.Errorf("booked %s %s worth %s, want damage -3 worth 120", typ, q, w)
	}
	s.audit(t)
}
