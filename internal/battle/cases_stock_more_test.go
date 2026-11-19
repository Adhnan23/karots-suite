package battle

import (
	"fmt"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

// lotIDs is a product's lots in the order they were made.
func (s *shop) lotIDs(t *testing.T, id int64) []int64 {
	t.Helper()
	var ids []int64
	s.must(t, s.db.Select(&ids, `SELECT id FROM stock_batches WHERE product_id = $1 ORDER BY id`, id))
	return ids
}

// lastMove is a product's newest movement: its type, quantity and worth.
func (s *shop) lastMove(t *testing.T, id int64) (typ string, qty, worth decimal.Decimal) {
	t.Helper()
	var m struct {
		Type string          `db:"type"`
		Qty  decimal.Decimal `db:"quantity"`
		Cost decimal.Decimal `db:"cost"`
	}
	s.must(t, s.db.Get(&m, `SELECT type, quantity, cost FROM stock_movements WHERE product_id = $1 ORDER BY id DESC LIMIT 1`, id))
	return m.Type, m.Qty, m.Cost
}

// value is what a product's lots are worth at cost.
func (s *shop) value(t *testing.T, id int64) decimal.Decimal {
	t.Helper()
	var v decimal.Decimal
	s.must(t, s.db.Get(&v, `SELECT COALESCE(SUM(qty_remaining * cost_price), 0) FROM stock_batches WHERE product_id = $1`, id))
	return v
}

func near(a, b decimal.Decimal, cents string) bool {
	return a.Sub(b).Abs().LessThanOrEqual(decimal.RequireFromString(cents))
}

func (s *shop) supplier(t *testing.T, name, phone string) int64 {
	var sup struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/suppliers", map[string]string{"name": name, "phone": phone}, &sup))
	return sup.ID
}

// STK-3: a removal beyond what there is is refused and changes nothing.
func TestSTK3NeverBelowZero(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	p := s.stocked(t, "Bulbs", s.unitPcs, 0, nil)
	s.addLot(t, p, "3", "100", "")
	box := s.stocked(t, "Bulb box", s.unitPcs, 0, nil)
	s.addLot(t, box, "1", "1000", "")
	var cv struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/conversions", map[string]any{"fromProductId": box, "toProductId": p, "ratio": "10"}, &cv))
	s.mustRefuse(t, "write off 4 of 3", s.admin.Do("POST", "/api/v1/admin/stock/damage", map[string]any{"productId": p, "quantity": "4", "reason": "damage"}, nil))
	s.mustRefuse(t, "open 2 boxes of 1", s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/conversions/%d/run", cv.ID), map[string]string{"quantity": "2"}, nil))
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/freeze", map[string]any{"productIds": []int64{p}}, nil))
	s.mustRefuse(t, "put out 4 of 3", s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/stock/%d/to-shelf", p), map[string]string{"quantity": "4"}, nil))
	if s.onHand(t, p) != "3.000" || s.onHand(t, box) != "1.000" {
		t.Errorf("on hand %s / %s after refusals, want 3 / 1", s.onHand(t, p), s.onHand(t, box))
	}
	s.audit(t)
}

// STK-4: a count sets on hand exactly; its movement is the difference, worth the
// units at their lot cost.
func TestSTK4CountSetsOnHand(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	p := s.stocked(t, "Batteries", s.unitPcs, 0, nil)
	s.addLot(t, p, "5", "30", "2027-01-31")
	s.addLot(t, p, "5", "50", "2027-06-30")
	count := func(n string) {
		s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/adjust", map[string]any{"productId": p, "newQuantity": n, "note": "count"}, nil))
	}
	count("7") // 3 short: off the first lot (FEFO) at 30
	if typ, q, w := s.lastMove(t, p); typ != "adjust" || !q.Equal(decimal.NewFromInt(-3)) || !w.Equal(decimal.NewFromInt(90)) {
		t.Errorf("count down: %s %s worth %s, want adjust -3 worth 90", typ, q, w)
	}
	count("9") // 2 found: a new lot at the product's cost (10)
	if _, q, w := s.lastMove(t, p); !q.Equal(decimal.NewFromInt(2)) || !w.Equal(decimal.NewFromInt(20)) {
		t.Errorf("count up: %s worth %s, want +2 worth 20", q, w)
	}
	count("0")
	if s.onHand(t, p) != "0.000" {
		t.Errorf("on hand %s, want 0", s.onHand(t, p))
	}
	s.audit(t)
}

// STK-5: a lot counted ends holding its count; no other lot moves.
func TestSTK5LotCount(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	p := s.stocked(t, "Gloves", s.unitPcs, 0, nil)
	s.addLot(t, p, "5", "30", "2027-01-31")
	s.addLot(t, p, "5", "50", "2027-06-30")
	ids := s.lotIDs(t, p)
	take := map[string]any{"lines": []map[string]any{{"productId": p,
		"batches": []map[string]any{{"batchId": ids[1], "counted": "3"}}}}}
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/take", take, nil))
	if s.lotLeft(t, ids[0]) != "5.000" || s.lotLeft(t, ids[1]) != "3.000" || s.onHand(t, p) != "8.000" {
		t.Errorf("lots %s / %s, on hand %s; want 5 / 3, 8", s.lotLeft(t, ids[0]), s.lotLeft(t, ids[1]), s.onHand(t, p))
	}
	// Up again, still only that lot.
	take["lines"].([]map[string]any)[0]["batches"] = []map[string]any{{"batchId": ids[1], "counted": "6"}}
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/take", take, nil))
	if s.lotLeft(t, ids[0]) != "5.000" || s.lotLeft(t, ids[1]) != "6.000" || s.onHand(t, p) != "11.000" {
		t.Errorf("then lots %s / %s, on hand %s; want 5 / 6, 11", s.lotLeft(t, ids[0]), s.lotLeft(t, ids[1]), s.onHand(t, p))
	}
	s.audit(t)
}

// STK-6: a named short lot is refused (never made up from another); a top-up keeps
// the lot's cost, price and expiry; a new lot with a blank cost takes the product's.
func TestSTK6NamedLots(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	p := s.stocked(t, "Ink", s.unitPcs, 0, nil)
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/stock/lots/%d/add", p),
		map[string]string{"qty": "2", "cost": "70", "sellingPrice": "99", "expiry": "2027-05-31"}, nil))
	s.addLot(t, p, "10", "", "")
	ids := s.lotIDs(t, p)
	s.mustRefuse(t, "write off 3 from a lot of 2", s.admin.Do("POST", "/api/v1/admin/stock/damage",
		map[string]any{"productId": p, "quantity": "3", "reason": "damage", "batchId": ids[0]}, nil))
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/adjust", map[string]any{"productId": p, "newQuantity": "15", "batchId": ids[0]}, nil))
	var top struct {
		Qty, Cost, Price, Expiry string
	}
	s.must(t, s.db.Get(&top, `SELECT qty_remaining::numeric(9,0)::text AS qty, cost_price::numeric(9,2)::text AS cost,
		selling_price::numeric(9,2)::text AS price, expiry_date::text AS expiry FROM stock_batches WHERE id = $1`, ids[0]))
	if top != (struct{ Qty, Cost, Price, Expiry string }{"5", "70.00", "99.00", "2027-05-31"}) {
		t.Errorf("topped-up lot %+v, want 5 at 70, price 99, 2027-05-31", top)
	}
	var blank string
	s.must(t, s.db.Get(&blank, `SELECT cost_price::numeric(9,2)::text FROM stock_batches WHERE id = $1`, ids[1]))
	if blank != "10.00" {
		t.Errorf("blank-cost lot at %s, want the product's 10.00", blank)
	}
	s.audit(t)
}

// STK-8: each write-off reason is its own movement type; any other is refused; a
// frozen item that was out for sale leaves on hand as it was, its loss booked.
func TestSTK8WriteOffReasons(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	p := s.stocked(t, "Cups", s.unitPcs, 0, nil)
	s.addLot(t, p, "10", "25", "")
	for _, r := range []string{"damage", "own_use", "staff"} {
		s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/damage", map[string]any{"productId": p, "quantity": "1", "reason": r}, nil))
		if typ, q, w := s.lastMove(t, p); typ != r || !q.Equal(decimal.NewFromInt(-1)) || !w.Equal(decimal.NewFromInt(25)) {
			t.Errorf("%s: booked %s %s worth %s", r, typ, q, w)
		}
	}
	var unnamed int
	s.must(t, s.db.Get(&unnamed, `SELECT count(*) FROM stock_movements WHERE product_id = $1 AND type = 'staff' AND reference_type IS NOT NULL`, p))
	if unnamed != 0 {
		t.Errorf("a write-off that named no lot points at a lot")
	}
	s.mustRefuse(t, "reason 'lost'", s.admin.Do("POST", "/api/v1/admin/stock/damage", map[string]any{"productId": p, "quantity": "1", "reason": "lost"}, nil))
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/freeze", map[string]any{"productIds": []int64{p}}, nil))
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/damage", map[string]any{"productId": p, "quantity": "2", "reason": "damage", "fromShelf": true}, nil))
	if s.onHand(t, p) != "7.000" {
		t.Errorf("frozen shelf write-off moved on hand to %s, want 7", s.onHand(t, p))
	}
	if typ, q, w := s.lastMove(t, p); typ != "damage" || !q.Equal(decimal.NewFromInt(-2)) || !w.IsPositive() {
		t.Errorf("shelf write-off booked %s %s worth %s, want damage -2 with its worth", typ, q, w)
	}
	s.audit(t)
}

// STK-9: giving a lot back — the count wins, the lot ends at 0, a difference moves
// between lots at a blended cost (no value appears or vanishes).
func TestSTK9GiveBack(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	sup := s.supplier(t, "Give Supplier", "0112000004")
	giveBack := func(lot int64, counted string) decimal.Decimal {
		var r struct{ LossValue string }
		s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/suppliers/%d/claims/lots/%d/give-back", sup, lot),
			map[string]string{"counted": counted}, &r))
		return dec(t, r.LossValue)
	}
	// Counted more than the lot holds: the next lot covers it.
	p := s.stocked(t, "Cream", s.unitPcs, sup, nil)
	s.addLot(t, p, "5", "10", "2027-01-31")
	s.addLot(t, p, "5", "20", "2027-06-30")
	ids := s.lotIDs(t, p)
	before := s.value(t, p)
	loss := giveBack(ids[0], "7")
	if s.lotLeft(t, ids[0]) != "0.000" || s.lotLeft(t, ids[1]) != "3.000" || s.onHand(t, p) != "3.000" {
		t.Errorf("more: lots %s / %s, on hand %s; want 0 / 3, 3", s.lotLeft(t, ids[0]), s.lotLeft(t, ids[1]), s.onHand(t, p))
	}
	if !near(before, s.value(t, p).Add(loss), "0.01") || !near(loss, decimal.NewFromInt(90), "0.01") {
		t.Errorf("more: value %s before, %s + loss %s after (want loss 90)", before, s.value(t, p), loss)
	}
	// Counted less: the rest joins the next lot.
	q := s.stocked(t, "Lotion", s.unitPcs, sup, nil)
	s.addLot(t, q, "4", "10", "2027-01-31")
	s.addLot(t, q, "6", "20", "2027-06-30")
	ids = s.lotIDs(t, q)
	before = s.value(t, q)
	loss = giveBack(ids[0], "3")
	if s.lotLeft(t, ids[1]) != "7.000" || s.onHand(t, q) != "7.000" || !near(before, s.value(t, q).Add(loss), "0.01") {
		t.Errorf("less: lot %s, on hand %s, value %s before vs %s + %s", s.lotLeft(t, ids[1]), s.onHand(t, q), before, s.value(t, q), loss)
	}
	// Nothing on the shelf, no other lot: the lot goes, as a count.
	r := s.stocked(t, "Balm", s.unitPcs, sup, nil)
	s.addLot(t, r, "2", "10", "2027-01-31")
	if loss = giveBack(s.lotIDs(t, r)[0], "0"); !loss.IsZero() || s.onHand(t, r) != "0.000" {
		t.Errorf("none: loss %s, on hand %s; want 0, 0", loss, s.onHand(t, r))
	}
	s.audit(t)
}

// STK-10: a recall marks a lot; stock doesn't move; undo clears both marks.
func TestSTK10Recall(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	sup := s.supplier(t, "Recall Supplier", "0112000005")
	p := s.stocked(t, "Tonic", s.unitPcs, sup, nil)
	s.addLot(t, p, "5", "10", "2027-01-31")
	lot := s.lotIDs(t, p)[0]
	url := fmt.Sprintf("/api/v1/admin/suppliers/%d/claims/lots/%d/recall", sup, lot)
	s.must(t, s.admin.Do("POST", url, map[string]bool{"stopSelling": true}, nil))
	var m struct {
		On    bool `db:"on"`
		Stops bool `db:"recall_stops"`
		Moves int  `db:"moves"`
	}
	read := func() {
		s.must(t, s.db.Get(&m, `SELECT recalled_on IS NOT NULL AS on, recall_stops,
			(SELECT count(*) FROM stock_movements WHERE product_id = $2) AS moves FROM stock_batches WHERE id = $1`, lot, p))
	}
	read()
	if !m.On || !m.Stops || m.Moves != 1 || s.onHand(t, p) != "5.000" {
		t.Errorf("recalled: %+v, on hand %s", m, s.onHand(t, p))
	}
	s.must(t, s.admin.Do("DELETE", url, nil, nil))
	read()
	if m.On || m.Stops {
		t.Errorf("undone: %+v", m)
	}
	s.audit(t)
}

// STK-11: a run takes the bags and adds what was weighed out, each bag its own lot
// carrying its expiry and value; not enough bags is refused.
func TestSTK11ConversionRun(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	bag := s.stocked(t, "Sugar bag", s.unitPcs, 0, nil)
	loose := s.stocked(t, "Sugar loose", s.unitKg, 0, nil)
	s.addLot(t, bag, "1", "2500", "2027-01-31")
	s.addLot(t, bag, "1", "2600", "2027-06-30")
	var cv struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/conversions", map[string]any{"fromProductId": bag, "toProductId": loose, "ratio": "25"}, &cv))
	run := fmt.Sprintf("/api/v1/admin/conversions/%d/run", cv.ID)
	s.mustRefuse(t, "open 3 of 2 bags", s.admin.Do("POST", run, map[string]string{"quantity": "3"}, nil))
	before := s.value(t, bag)
	s.must(t, s.admin.Do("POST", run, map[string]string{"quantity": "2", "toQty": "49.2"}, nil))
	var lots []struct {
		Qty    decimal.Decimal `db:"qty_remaining"`
		Expiry string          `db:"expiry"`
	}
	s.must(t, s.db.Select(&lots, `SELECT qty_remaining, expiry_date::text AS expiry FROM stock_batches WHERE product_id = $1 ORDER BY id`, loose))
	if len(lots) != 2 || lots[0].Expiry != "2027-01-31" || lots[1].Expiry != "2027-06-30" || !lots[0].Qty.Add(lots[1].Qty).Equal(decimal.RequireFromString("49.2")) {
		t.Errorf("loose lots %+v, want two (one per bag, their expiry) adding to 49.2", lots)
	}
	if s.onHand(t, bag) != "0.000" || s.onHand(t, loose) != "49.200" || !near(before, s.value(t, loose), "0.02") {
		t.Errorf("bags %s, loose %s, value %s → %s", s.onHand(t, bag), s.onHand(t, loose), before, s.value(t, loose))
	}
	s.audit(t)
}

// STK-12: the guide's pencils — frozen stock doesn't move on a sale; put out for
// sale takes it out; unfreezing adds what's out; then sales move it again.
func TestSTK12FreezePencils(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	p := s.stocked(t, "Pencil", s.unitPcs, 0, nil)
	s.addLot(t, p, "200", "5", "")
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/freeze", map[string]any{"productIds": []int64{p}}, nil))
	s.open(t, s.till1)
	s.sell(t, s.till1, "0", []line{{"productId": p, "quantity": "5"}}, cash("100"))
	if s.onHand(t, p) != "200.000" {
		t.Errorf("frozen sale moved stock to %s", s.onHand(t, p))
	}
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/stock/%d/to-shelf", p), map[string]string{"quantity": "10"}, nil))
	if typ, q, _ := s.lastMove(t, p); typ != "to_shelf" || !q.Equal(decimal.NewFromInt(-10)) || s.onHand(t, p) != "190.000" {
		t.Errorf("put out: %s %s, on hand %s", typ, q, s.onHand(t, p))
	}
	newFrozen := func(name string) bool {
		var on bool
		s.must(t, s.db.Get(&on, `SELECT stock_frozen FROM products WHERE id = $1`, s.stocked(t, name, s.unitPcs, 0, nil)))
		return on
	}
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/freeze-shop", nil, nil))
	if !newFrozen("Crayon") {
		t.Errorf("a product made after Freeze the shop isn't frozen")
	}
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/unfreeze-shop", nil, nil))
	if newFrozen("Marker") {
		t.Errorf("a product made after Unfreeze the shop is frozen")
	}
	if s.onHand(t, p) != "190.000" {
		t.Errorf("freezing / unfreezing the shop moved stock to %s", s.onHand(t, p))
	}
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/freeze", map[string]any{"productIds": []int64{p}}, nil))
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/stock/%d/unfreeze", p), map[string]string{"shelfQty": "13"}, nil))
	if s.onHand(t, p) != "203.000" {
		t.Errorf("unfrozen with 13 out: on hand %s, want 203", s.onHand(t, p))
	}
	s.sell(t, s.till1, "0", []line{{"productId": p, "quantity": "3"}}, cash("60"))
	if s.onHand(t, p) != "200.000" {
		t.Errorf("after unfreezing a sale left %s, want 200", s.onHand(t, p))
	}
	s.audit(t)
}

// STK-13: into a frozen product — out for sale adds nothing, into storage adds;
// both frozen never asks: out for sale.
func TestSTK13ConversionIntoFrozen(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	box := s.stocked(t, "Chalk box", s.unitPcs, 0, nil)
	stick := s.stocked(t, "Chalk", s.unitPcs, 0, nil)
	s.addLot(t, box, "3", "100", "")
	var cv struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/conversions", map[string]any{"fromProductId": box, "toProductId": stick, "ratio": "10"}, &cv))
	run := fmt.Sprintf("/api/v1/admin/conversions/%d/run", cv.ID)
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/freeze", map[string]any{"productIds": []int64{stick}}, nil))
	s.must(t, s.admin.Do("POST", run, map[string]any{"quantity": "1"}, nil))
	if typ, q, _ := s.lastMove(t, stick); typ != "conversion" || !q.IsZero() || s.onHand(t, stick) != "0.000" {
		t.Errorf("out for sale: %s %s, on hand %s; want a 0 movement", typ, q, s.onHand(t, stick))
	}
	s.must(t, s.admin.Do("POST", run, map[string]any{"quantity": "1", "intoStock": true}, nil))
	if s.onHand(t, stick) != "10.000" {
		t.Errorf("into storage: on hand %s, want 10", s.onHand(t, stick))
	}
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/freeze", map[string]any{"productIds": []int64{box}}, nil))
	s.must(t, s.admin.Do("POST", run, map[string]any{"quantity": "1", "intoStock": true}, nil))
	if s.onHand(t, stick) != "10.000" || s.onHand(t, box) != "0.000" {
		t.Errorf("both frozen: chalk %s (want 10, out for sale), boxes %s", s.onHand(t, stick), s.onHand(t, box))
	}
	s.audit(t)
}

// STK-14: folding a card + loose pair opens every card into the loose product,
// adds a pack of the ratio's size, retires the card; value unchanged.
func TestSTK14Fold(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	card := s.stocked(t, "Amoxil card", s.unitPcs, 0, nil)
	tab := s.stocked(t, "Amoxil", s.unitPcs, 0, nil)
	s.addLot(t, card, "3", "120", "2027-03-31")
	var cv struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/conversions", map[string]any{"fromProductId": card, "toProductId": tab, "ratio": "10"}, &cv))
	before := s.value(t, card)
	fold := fmt.Sprintf("/api/v1/admin/products/%d/fold", tab)
	s.mustRefuse(t, "a pack name of 31 letters", s.admin.Do("POST", fold, map[string]any{"conversionId": cv.ID, "name": strings.Repeat("c", 31)}, nil))
	s.must(t, s.admin.Do("POST", fold, map[string]any{"conversionId": cv.ID, "name": "Card"}, nil))
	var f struct {
		Size   string `db:"size"`
		Active bool   `db:"active"`
	}
	s.must(t, s.db.Get(&f, `SELECT (SELECT size::numeric(9,0)::text FROM product_packs WHERE product_id = $1) AS size,
		(SELECT is_active FROM products WHERE id = $2) AS active`, tab, card))
	if s.onHand(t, tab) != "30.000" || s.onHand(t, card) != "0.000" || f.Size != "10" || f.Active || !near(before, s.value(t, tab), "0.01") {
		t.Errorf("fold: tablets %s, cards %s, %+v, value %s → %s", s.onHand(t, tab), s.onHand(t, card), f, before, s.value(t, tab))
	}
	s.audit(t)
}

// STK-16: merging a product into another sums on hand; lots and movements follow.
func TestSTK16ProductMerge(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	keep := s.stocked(t, "Panadol", s.unitPcs, 0, nil)
	twin := s.stocked(t, "Panadol 500", s.unitPcs, 0, nil)
	s.addLot(t, keep, "4", "5", "")
	s.addLot(t, twin, "6", "6", "")
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/products/%d/merge/%d", twin, keep), nil, nil))
	if s.onHand(t, keep) != "10.000" || len(s.lotIDs(t, keep)) != 2 {
		t.Errorf("merged: on hand %s, %d lots; want 10 in 2", s.onHand(t, keep), len(s.lotIDs(t, keep)))
	}
	s.audit(t)
}

// STK-19: a warranty replacement takes one unit at its cost; none left is refused.
func TestSTK19WarrantyReplacement(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	ph := s.prod["serial"]
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/adjust", map[string]any{"productId": ph, "newQuantity": "2"}, nil))
	s.open(t, s.till1)
	s.sell(t, s.till1, "0", []line{{"productId": ph, "quantity": "1", "serials": []string{"IMEI-STK19"}}}, cash("15000"))
	var unit int64
	s.must(t, s.db.Get(&unit, `SELECT id FROM warranty_units WHERE serial_no = 'IMEI-STK19'`))
	s.must(t, s.till1.Do("POST", "/api/v1/till/warranty/replace", map[string]any{"unitId": unit, "newSerial": "IMEI-STK19-B", "reason": "dead"}, nil))
	if typ, q, w := s.lastMove(t, ph); typ != "warranty_replacement" || !q.Equal(decimal.NewFromInt(-1)) || !w.Equal(decimal.NewFromInt(1)) || s.onHand(t, ph) != "0.000" {
		t.Errorf("replacement: %s %s worth %s, on hand %s", typ, q, w, s.onHand(t, ph))
	}
	s.must(t, s.db.Get(&unit, `SELECT id FROM warranty_units WHERE serial_no = 'IMEI-STK19-B'`))
	s.mustRefuse(t, "replace with none in stock", s.till1.Do("POST", "/api/v1/till/warranty/replace", map[string]any{"unitId": unit, "newSerial": "IMEI-STK19-C", "reason": "dead"}, nil))
	s.audit(t)
}

// STK-20: a lot's price, expiry and cost edits move no stock.
func TestSTK20LotEdits(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	p := s.stocked(t, "Glue", s.unitPcs, 0, nil)
	s.addLot(t, p, "5", "10", "")
	lot := s.lotIDs(t, p)[0]
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/stock/batches/%d/price", lot), map[string]string{"sellingPrice": "25"}, nil))
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/stock/batches/%d/expiry", lot), map[string]string{"expiry": "2027-08-31"}, nil))
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/take", map[string]any{"lines": []map[string]any{{"productId": p,
		"batches": []map[string]any{{"batchId": lot, "cost": "12"}}}}}, nil))
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/stock/batches/%d/price", lot), map[string]string{"sellingPrice": ""}, nil))
	var e struct {
		Moves int    `db:"moves"`
		Price string `db:"price"`
		Cost  string `db:"cost"`
	}
	s.must(t, s.db.Get(&e, `SELECT (SELECT count(*) FROM stock_movements WHERE product_id = $2) AS moves,
		selling_price::numeric(9,2)::text AS price, cost_price::numeric(9,2)::text AS cost FROM stock_batches WHERE id = $1`, lot, p))
	if e.Moves != 1 || e.Price != "0.00" || e.Cost != "12.00" || s.onHand(t, p) != "5.000" {
		t.Errorf("edits: %+v, on hand %s; want 1 movement, price follows the product, cost 12", e, s.onHand(t, p))
	}
	s.audit(t)
}

// STK-22: the stock value is Σ live lots × their cost.
func TestSTK22StockValue(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	var c struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/categories", map[string]string{"name": "Valued"}, &c))
	p := s.stocked(t, "Kettle", s.unitPcs, 0, map[string]any{"categoryId": c.ID})
	s.addLot(t, p, "5", "10", "")
	s.addLot(t, p, "3", "20", "")
	var r struct{ CostValue string }
	s.must(t, s.admin.Do("GET", fmt.Sprintf("/api/v1/admin/reports/inventory?category=%d", c.ID), nil, &r))
	if r.CostValue != "110.00" {
		t.Errorf("stock value %s, want 110.00", r.CostValue)
	}
	s.audit(t)
}

// STK-11: bags split three ways — the last lot takes what's left, so the loose
// lots add up to exactly what was weighed out.
func TestSTK11ThreeBagsAddUpExactly(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	bag := s.stocked(t, "Flour bag", s.unitPcs, 0, nil)
	loose := s.stocked(t, "Flour loose", s.unitKg, 0, nil)
	for _, e := range []string{"2027-01-31", "2027-02-28", "2027-03-31"} {
		s.addLot(t, bag, "1", "2500", e)
	}
	var cv struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/conversions", map[string]any{"fromProductId": bag, "toProductId": loose, "ratio": "25"}, &cv))
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/conversions/%d/run", cv.ID), map[string]string{"quantity": "3", "toQty": "74"}, nil))
	var sum decimal.Decimal
	s.must(t, s.db.Get(&sum, `SELECT SUM(qty_remaining) FROM stock_batches WHERE product_id = $1`, loose))
	if !sum.Equal(decimal.NewFromInt(74)) || len(s.lotIDs(t, loose)) != 3 {
		t.Errorf("three bags gave %d lots holding %s, want 3 holding 74", len(s.lotIDs(t, loose)), sum)
	}
	s.audit(t)
}
