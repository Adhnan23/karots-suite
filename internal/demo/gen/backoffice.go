package gen

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// The back room: what goes wrong and gets put right — breakage and who pays for
// it, faulty units under warranty, and goods sent back to a supplier.

// later is something the shop finishes on a later day (a rep's visit).
type later struct {
	day time.Time
	do  func(day time.Time) error
}

// delivery is a supplier's drop the shop may send goods back from.
type delivery struct {
	id  int64
	sup int
	day time.Time
}

// mishap is a breakage note; supplier = it came that way, so the rep credits it.
type mishap struct {
	note     string
	supplier bool
}

var mishaps = map[string][]mishap{
	"Dairy":          {{"Cups swollen — fridge tripped overnight", false}, {"Curd pot cracked in the crate", true}, {"Past its date", false}},
	"Bakery":         {{"Day-old bread", false}, {"Packets crushed in the delivery crate", true}},
	"Beverages":      {{"Bottle leaking at the cap", true}, {"Dropped while stacking", false}},
	"Vegetables":     {{"Gone soft", false}, {"Sprouted", false}},
	"Rice & Grains":  {{"Packet torn — rat damage", false}, {"Weevils in the packet", true}},
	"Groceries":      {{"Packet torn — rat damage", false}, {"Tin dented, swollen", true}},
	"Snacks":         {{"Packets crushed", false}, {"Past its date", false}},
	"Cold & Allergy": {{"Syrup bottle cracked", false}, {"Seal broken in the carton", true}},
	"Stomach":        {{"Syrup bottle cracked", false}, {"Seal broken in the carton", true}},
	"Pain & Fever":   {{"Strip torn", false}, {"Expired — missed at the monthly check", false}},
	"Cases & Glass":  {{"Cracked in the display", false}, {"Came scratched in the box", true}},
	"Cables":         {{"Cut while testing", false}, {"Dead out of the box", true}},
	"Chargers":       {{"Dead out of the box", true}, {"Dropped on the floor", false}},
	"Cement & Sand":  {{"Bag split while unloading", false}, {"Bag gone hard — damp", false}, {"Bag torn on the lorry", true}},
	"Paint":          {{"Tin dented, lid won't seal", true}, {"Tin knocked over", false}},
	"Electrical":     {{"Bulb broke on the shelf", false}, {"Dead out of the box", true}},
	"Spices":         {{"Packet torn — rat damage", false}, {"Gone damp, lumpy", false}},
	"Tea & Coffee":   {{"Packet torn — rat damage", false}, {"Jar lid cracked", true}},
	"Household":      {{"Bottle leaking", true}, {"Pack wet — roof leak", false}},
	"Personal Care":  {{"Bottle leaking", true}, {"Seal broken — customer opened", false}},
	"Antibiotics":    {{"Strip torn", false}, {"Box crushed in the carton", true}},
	"Heart & BP":     {{"Strip torn", false}, {"Expired — missed at the monthly check", false}},
	"Diabetes":       {{"Strip torn", false}, {"Box crushed in the carton", true}},
	"Vitamins":       {{"Tablets discoloured — damp", false}, {"Past its date", false}},
	"First Aid":      {{"Pack wet — roof leak", false}, {"Seal broken in the carton", true}},
	"Mother & Baby":  {{"Pack torn", false}, {"Tin dented", true}},
	"Skin Care":      {{"Tube leaking", true}, {"Tester used by customers", false}},
	"Audio":          {{"Dead out of the box", true}, {"Used for display — worn", false}},
	"Power":          {{"Won't charge out of the box", true}},
	"Storage":        {{"Dead out of the box", true}},
	"Accessories":    {{"Broke in the display", false}, {"Came broken in the box", true}},
	"Plumbing":       {{"Pipe cracked at the end", false}, {"Came cracked", true}},
	"Fasteners":      {{"Box rusted — roof leak", false}},
	"Steel":          {{"Rusted in the yard", false}},
	"Roofing":        {{"Sheet bent while unloading", false}, {"Came bent on the lorry", true}},
	"Tools":          {{"Handle broke — used in the shop", false}, {"Came broken in the box", true}},
	"Garden":         {{"Hose split in the sun", false}},
}

// mishapFor is what happened to it: the product's own story first, then its kind's.
func (g *gen) mishapFor(it item) mishap {
	for _, r := range []struct {
		word string
		m    mishap
	}{
		{"Eggs", mishap{"Tray slipped — eggs cracked", false}},
		{"Soap", mishap{"Wrapper torn, soap chipped", false}},
		{"Bread", mishap{"Day-old bread", false}},
		{"Yoghurt", mishap{"Cups swollen — fridge tripped overnight", false}},
		{"Cement", mishap{"Bag split while unloading", false}},
	} {
		if strings.Contains(it.name, r.word) {
			return r.m
		}
	}
	if it.unit == "kg" { // sold loose from a bin
		ms := []mishap{{"Swept up — spilled from the bin", false}, {"Bin got damp", false}}
		return ms[g.r.IntN(len(ms))]
	}
	ms := mishaps[it.cat]
	if ms == nil {
		ms = anyMishap
	}
	return ms[g.r.IntN(len(ms))]
}

var anyMishap = []mishap{{"Damaged while unloading", false}, {"Pack damaged — can't sell", false}, {"Came damaged", true}}

// backRoom runs the day's back-room business after closing.
func (g *gen) backRoom(day time.Time) error {
	keep := g.later[:0]
	var due []later
	for _, l := range g.later {
		if l.day.After(day) {
			keep = append(keep, l)
		} else {
			due = append(due, l)
		}
	}
	g.later = keep
	for _, l := range due {
		if err := l.do(day); err != nil {
			return err
		}
	}
	if g.r.IntN(5) == 0 {
		if err := g.breakage(day); err != nil {
			return err
		}
	}
	if g.r.IntN(9) == 0 {
		if err := g.warrantyClaim(day); err != nil {
			return err
		}
	}
	if g.r.IntN(16) == 0 {
		return g.sendBack(day)
	}
	return nil
}

func (g *gen) soon(day time.Time, minDays, maxDays int, do func(time.Time) error) {
	g.later = append(g.later, later{day.AddDate(0, 0, minDays+g.r.IntN(maxDays-minDays+1)), do})
}

// breakage writes off something dropped or spoiled, with what really happened;
// when it came that way the supplier's rep credits it on a later visit.
func (g *gen) breakage(day time.Time) error {
	it := g.pick()
	if it.service || it.warranty > 0 || g.stock[it.id] < 3 {
		return nil
	}
	m := g.mishapFor(it)
	q := 1.0
	if strings.HasPrefix(it.name, "Eggs") {
		q = float64(2 + g.r.IntN(5)) // a tray slipped
	}
	g.stock[it.id] -= q
	g.tm.at(at(day, 15+3*g.r.Float64()))
	if err := g.admin.Do("POST", "/api/v1/admin/stock/damage", map[string]any{
		"productId": it.id, "quantity": fmt.Sprint(q), "reason": "damage", "note": m.note,
	}, nil); err != nil {
		return err
	}
	if !m.supplier {
		return nil
	}
	var mv int64
	if err := g.d.DB.GetContext(g.ctx, &mv, `SELECT max(id) FROM stock_movements WHERE product_id = $1 AND type = 'damage'`, it.id); err != nil {
		return err
	}
	value := it.cost.Mul(decimal.NewFromFloat(q))
	g.soon(day, 4, 14, func(d time.Time) error {
		g.tm.at(at(d, 11+2*g.r.Float64()))
		return g.admin.Do("POST", "/api/v1/admin/recovery", map[string]any{"sourceType": "damage", "sourceId": mv, "outcome": "paid",
			"supplierId": g.s.suppliers[it.supplier], "amount": value.StringFixed(2), "note": "Rep took it back — credited on the bill"}, nil)
	})
	return nil
}

// warrantyClaim: a customer brings a faulty unit back under warranty; the
// counter swaps it from stock, and the faulty one goes back to the supplier,
// who replaces it (or, now and then, credits it) a week or two later.
func (g *gen) warrantyClaim(day time.Time) error {
	var u struct {
		ID      int64 `db:"id"`
		Product int64 `db:"product_id"`
	}
	err := g.d.DB.GetContext(g.ctx, &u, `
		SELECT id, product_id FROM warranty_units
		WHERE status = 'active' AND created_at < $1 AND warranty_until > $2
		ORDER BY id DESC OFFSET $3 LIMIT 1`, g.tm.start, day, g.r.IntN(15))
	if err != nil {
		return nil // nothing sold under warranty long enough ago
	}
	var it item
	for _, x := range g.s.items {
		if x.id == u.Product {
			it = x
		}
	}
	if it.warranty == 0 || g.stock[it.id] < 1 || len(it.faults) == 0 {
		return nil
	}
	sn, err := g.serial(it)
	if err != nil {
		return err
	}
	cl, err := g.tillOf(g.cashier())
	if err != nil {
		return err
	}
	g.tm.at(at(day, 10+8*g.r.Float64()))
	if err := cl.Do("POST", "/api/v1/till/warranty/replace", map[string]any{"unitId": u.ID, "newSerial": sn,
		"reason": it.faults[g.r.IntN(len(it.faults))]}, nil); err != nil {
		return fmt.Errorf("warranty claim: %w", err)
	}
	g.stock[it.id]--
	g.soon(day, 7, 20, func(d time.Time) error {
		g.tm.at(at(d, 11+2*g.r.Float64()))
		body := map[string]any{"sourceType": "warranty", "sourceId": u.ID, "outcome": "replacement", "supplierId": g.s.suppliers[it.supplier],
			"note": "Sent to the agent — new unit back", "extraPerUnit": "0"}
		if g.r.IntN(4) == 0 {
			body["outcome"], body["amount"], body["note"] = "paid", it.cost.StringFixed(2), "Agent credited it — no stock to swap"
		} else {
			g.stock[it.id]++
		}
		return g.admin.Do("POST", "/api/v1/admin/recovery", body, nil)
	})
	return nil
}

// Fresh drops go back for what arrived wrong; older, paid ones because they
// didn't sell in time.
var sendBackFresh = []string{"Damaged in delivery", "Wrong item sent", "Over-supplied — not ordered"}
var sendBackOld = []string{"Short expiry — rep agreed to take back", "Slow mover — rep took it back"}

// sendBack returns part of a recent delivery to its supplier. Unpaid, it comes
// off the bill; already paid, the rep refunds it in cash on the next visit.
func (g *gen) sendBack(day time.Time) error {
	if len(g.deliveries) == 0 {
		return nil
	}
	dv := g.deliveries[g.r.IntN(len(g.deliveries))]
	paid := g.paidAlready(dv)
	reason := sendBackFresh[g.r.IntN(len(sendBackFresh))]
	if paid {
		reason = sendBackOld[g.r.IntN(len(sendBackOld))]
	}
	var ret struct {
		Lines []struct {
			ProductID int64  `json:"productId"`
			BatchID   int64  `json:"batchId"`
			Left      string `json:"left"`
			Cost      string `json:"cost"`
		} `json:"lines"`
	}
	if err := g.admin.Do("GET", fmt.Sprintf("/api/v1/admin/purchases/%d/returnable", dv.id), nil, &ret); err != nil {
		return err
	}
	var lines []map[string]any
	total := decimal.Zero
	for _, l := range ret.Lines {
		left, _ := strconv.ParseFloat(l.Left, 64)
		if left < 2 || g.stock[l.ProductID] < 3 || len(lines) == 2 || g.r.IntN(3) != 0 {
			continue
		}
		q := float64(1 + g.r.IntN(min(3, int(left)-1)))
		lines = append(lines, map[string]any{"productId": l.ProductID, "quantity": fmt.Sprint(q), "batchId": l.BatchID})
		total = total.Add(decimal.RequireFromString(l.Cost).Mul(decimal.NewFromFloat(q)))
		g.stock[l.ProductID] -= q
	}
	if len(lines) == 0 {
		return nil
	}
	g.tm.at(at(day, 9+3*g.r.Float64()))
	if err := g.admin.Do("POST", "/api/v1/admin/purchase-returns", map[string]any{"supplierId": g.s.suppliers[dv.sup], "purchaseId": dv.id,
		"reference": fmt.Sprintf("DN-%d%02d", day.YearDay(), dv.sup), "reason": reason, "lines": lines}, nil); err != nil {
		return fmt.Errorf("return to supplier: %w", err)
	}
	if !paid {
		g.offTheBill(dv, total)
		return nil
	}
	g.soon(day, 5, 12, func(d time.Time) error {
		cl, err := g.tillOf(g.cashier())
		if err != nil {
			return err
		}
		g.tm.at(at(d, 10+6*g.r.Float64()))
		return cl.Do("POST", fmt.Sprintf("/api/v1/till/suppliers/%d/refund", g.s.suppliers[dv.sup]), map[string]any{
			"amount": total.StringFixed(2), "method": "cash", "note": "Rep refunded the goods sent back"}, nil)
	})
	return nil
}

// paidAlready: the supplier's bills are settled after 30 days (paySuppliers).
func (g *gen) paidAlready(dv delivery) bool {
	for _, d := range g.owed[dv.sup] {
		if d.day.Equal(dv.day) {
			return false
		}
	}
	return true
}

// offTheBill takes a return off the delivery's unpaid bill.
func (g *gen) offTheBill(dv delivery, amount decimal.Decimal) {
	for i, d := range g.owed[dv.sup] {
		if d.day.Equal(dv.day) {
			g.owed[dv.sup][i].amount = decimal.Max(decimal.Zero, d.amount.Sub(amount))
			return
		}
	}
}
