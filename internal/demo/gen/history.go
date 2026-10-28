package gen

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"karots-suite/internal/features/cashregister"
	"karots-suite/internal/features/purchases"
	"karots-suite/internal/features/sales"
)

// historyDays is how far back the shop goes: a little over a year, so every
// report has last year to compare against.
const historyDays = 400

// dayFactor shapes a day's trade: the shop grew over the year; Saturdays and
// Poya days are busier, Sundays quieter; Avurudu (mid-April) and December peak.
func dayFactor(d time.Time, daysAgo int) float64 {
	f := 0.6 + 0.4*(1-min(float64(daysAgo), historyDays)/historyDays)
	switch d.Weekday() {
	case time.Saturday:
		f *= 1.25
	case time.Sunday:
		f *= 0.8
	}
	if isPoya(d) {
		f *= 1.15
	}
	if d.Month() == time.April && d.Day() >= 8 && d.Day() <= 13 {
		f *= 1.7
	}
	if d.Month() == time.December {
		f *= 1.3
	}
	return f
}

// isPoya approximates the full-moon day from a known full moon (29.53-day cycle).
func isPoya(d time.Time) bool {
	ref := time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)
	cycle := math.Mod(d.Sub(ref).Hours()/24, 29.530588)
	if cycle < 0 {
		cycle += 29.530588
	}
	return cycle < 1 || cycle > 28.53
}

// saleHour spreads a day's sales: a morning rush, lunch, and the evening peak.
func saleHour(r *rand.Rand) float64 {
	switch x := r.Float64(); {
	case x < 0.25:
		return 8 + r.Float64()*3 // 08–11
	case x < 0.55:
		return 11.5 + r.Float64()*3 // lunch
	default:
		return 16 + r.Float64()*4.5 // evening
	}
}

// gen carries one run's state.
type gen struct {
	d     Deps
	s     *shop
	r     *rand.Rand
	tm    *timeMachine
	stock map[int64]float64 // what the generator thinks is on the shelf
	days  int               // how far back this run's history starts

	admin *Client       // the owner, for the back office
	owed  map[int][]due // deliveries not yet paid, by supplier

	tills    map[int64]*Client // signed-in till clients, by cashier
	open     []person          // who is on a till today
	jobs     []*repairJob      // repairs still moving
	docs     []docService      // Print & Copy services
	lastTill *Client           // the till that rang the last plugin sale
	combos   []combo           // bundles on offer
	patients []rxPatient       // pharmacy patients and their prescription items
	ctx      context.Context

	daySales   int // today's till sales so far, for Deps.OnDay
	dayTakings decimal.Decimal
}

func at(day time.Time, hour float64) time.Time {
	return day.Add(time.Duration(hour * float64(time.Hour)))
}

// history builds every day from days ago to yesterday, a week per batch.
func (g *gen) history(now time.Time, days int, progress func(int, string)) error {
	today := localDay(now)
	g.days = days
	for start := days; start > 0; start -= 7 {
		if err := g.tm.begin(g.ctx); err != nil {
			return err
		}
		for ago := start; ago > start-7 && ago > 0; ago-- {
			if err := g.day(today.AddDate(0, 0, -ago), ago, 20.5); err != nil {
				return err
			}
		}
		if err := g.tm.shift(g.ctx); err != nil {
			return err
		}
		progress(10+70*(days-start)/days, "Trading "+today.AddDate(0, 0, -max(start-6, 1)).Format("Mon 2 Jan 2006"))
	}
	return nil
}

func localDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// day is one trading day: deliveries, two tills, the sales, closing up.
// closeAt < 0 leaves the tills open (today).
func (g *gen) day(day time.Time, ago int, until float64) error {
	// Monday's delivery comes at 07:30 — today, only if that has happened yet
	opening := ago > 0 && ago == g.days // the empty shop's first delivery
	if (day.Weekday() == time.Monday && until > 7.6) || opening {
		if err := g.restock(day, opening); err != nil {
			return err
		}
	}
	g.daySales, g.dayTakings = 0, decimal.Zero
	tills := g.s.cashiers[:2]
	if ago%3 == 0 && len(g.s.cashiers) > 2 {
		tills = []person{g.s.cashiers[0], g.s.cashiers[2]}
	}
	if ago == 0 { // today the staff are on; the shown cashier is the owner's to demo
		tills = g.s.cashiers[1:min(3, len(g.s.cashiers))]
	}
	// Before 09:00 today has barely begun: the tills opened at midnight and a
	// handful of sales came in since — the dashboard never opens on an empty day.
	early := until < 9
	opens := 7.9
	if early {
		opens = 0
	}
	g.open = tills
	for _, c := range tills {
		g.tm.at(at(day, opens))
		if _, err := g.d.Svc.CashRegister.Open(g.ctx, c.id, cashregister.OpenInput{OpeningCash: "5000"}); err != nil {
			return fmt.Errorf("open till for %s: %w", c.login.Name, err)
		}
	}
	if until > 8.05 {
		if err := g.openBags(day, 8.0, tills[0]); err != nil {
			return err
		}
	}
	n := poisson(g.r, float64(g.s.p.Volume)*dayFactor(day, ago))
	hours := make([]float64, 0, n)
	for i := 0; i < n; i++ {
		if early {
			if i < 6 {
				hours = append(hours, until*(0.1+0.8*g.r.Float64()))
			}
			continue
		}
		if h := saleHour(g.r); h < until {
			hours = append(hours, h)
		}
	}
	slices.Sort(hours)
	lunch := false
	for _, h := range hours {
		if h > 13 && !lunch { // a busy morning empties a bin
			lunch = true
			if err := g.openBags(day, 13.0, tills[len(tills)-1]); err != nil {
				return err
			}
		}
		g.tm.at(at(day, h))
		if err := g.sale(tills[g.r.IntN(len(tills))]); err != nil {
			return err
		}
	}
	if err := g.pluginsDaily(day, until); err != nil {
		return err
	}
	if ago == 0 {
		return nil // today: the tills stay open, whatever the hour
	}
	for _, c := range tills {
		sum, err := g.d.Svc.CashRegister.Summary(g.ctx, c.id)
		if err != nil {
			return err
		}
		counted := sum.Expected
		if g.r.IntN(12) == 0 { // now and then a till is a little short
			counted = counted.Sub(decimal.NewFromInt(int64(10 * (1 + g.r.IntN(30)))))
		}
		g.tm.at(at(day, 20.6))
		if _, err := g.d.Svc.CashRegister.Close(g.ctx, c.id, cashregister.CloseInput{ClosingCash: counted.String(), DestLockerID: g.s.safe}); err != nil {
			return fmt.Errorf("close till for %s: %w", c.login.Name, err)
		}
	}
	if err := g.backOffice(day); err != nil {
		return err
	}
	if g.d.OnDay != nil {
		g.d.OnDay(Day{Date: day.Format(time.DateOnly), Sales: g.daySales, Takings: g.dayTakings.StringFixed(2), N: g.days - ago + 1, Of: g.days})
	}
	return nil
}

// restock orders about two weeks of each product that's running low, from its
// supplier; the first delivery fills the empty shop.
func (g *gen) restock(day time.Time, opening bool) error {
	bySup := map[int][]purchases.ItemInput{}
	for i, it := range g.s.items {
		if _, loose := g.s.bagFor(i); loose || it.service {
			continue // services hold no stock; loose rice comes out of the bags
		}
		week, have := g.weekly(it), g.stock[it.id]
		for _, c := range g.s.convs {
			if c.from == i { // a bag: enough for its loose bin too
				to := g.s.items[c.to]
				week += g.weekly(to) / c.ratio
				have += g.stock[to.id] / c.ratio
			}
		}
		want := week * 2.2
		if opening {
			want = week * 3
		}
		if !opening && have > week*1.2 {
			continue
		}
		qty := math.Ceil(want - have)
		if qty <= 0 {
			continue
		}
		bySup[it.supplier] = append(bySup[it.supplier], purchases.ItemInput{
			ProductID: it.id, Quantity: fmt.Sprint(qty), CostPrice: it.cost.String(), SellingPrice: it.price.String(),
		})
		g.stock[it.id] += qty
	}
	for sup, items := range bySup {
		inv := fmt.Sprintf("INV-%d%03d", day.YearDay(), sup)
		g.tm.at(at(day, 7.5))
		pd, err := purchases.NewService(g.d.DB).Create(g.ctx, purchases.CreateInput{
			SupplierID: g.s.suppliers[sup], InvoiceNo: &inv, Items: items,
		}, g.s.owner.id)
		if err != nil {
			return fmt.Errorf("delivery: %w", err)
		}
		g.owed[sup] = append(g.owed[sup], due{day, pd.Purchase.Total})
	}
	return nil
}

// sale rings up 1–5 lines picked by how well things sell; most pay cash, some
// card, a few go on a customer's account.
func (g *gen) sale(c person) error {
	lines := 1 + g.r.IntN(3)
	if g.r.IntN(5) == 0 {
		lines += 2
	}
	var items []sales.ItemInput
	total := decimal.Zero
	seen := map[int64]bool{}
	for tries := 0; len(items) < lines && tries < 20; tries++ {
		it := g.pick()
		if seen[it.id] {
			continue
		}
		if it.service {
			seen[it.id] = true
			q := 1.0
			if strings.HasSuffix(it.name, "(cup)") || strings.HasSuffix(it.name, "(per joint)") {
				q = float64(1 + g.r.IntN(3)) // tea for the two of them; three joints
			}
			if slices.ContainsFunc(it.parts, func(pt partUse) bool { return g.stock[pt.id] < pt.perUnit*q+1 }) {
				continue // no glass left to fit
			}
			items = append(items, sales.ItemInput{ProductID: it.id, Quantity: fmt.Sprint(q)})
			total = total.Add(it.price.Mul(decimal.NewFromFloat(q)))
			for _, pt := range it.parts {
				g.stock[pt.id] -= pt.perUnit * q
			}
			continue
		}
		if g.stock[it.id] < 1 { // sold out until the next delivery
			continue
		}
		seen[it.id] = true
		q := 1.0
		switch {
		case it.warranty > 0: // one at a time, each with its own serial
		case it.unit == "m":
			q = float64(2 + g.r.IntN(24)) // metres of wire for a room
		case it.unit == "kg" && g.r.IntN(3) == 0:
			q = 0.5 * float64(1+g.r.IntN(4))
		case len(it.packs) > 0 && g.r.IntN(8) == 0:
			q, _ = it.packs[0].Size.Float64() // a whole tray, a box of cards
		case g.r.IntN(6) == 0:
			q = float64(2 + g.r.IntN(3))
		}
		if it.unit == "kg" || it.unit == "m" {
			q = min(q, math.Floor(g.stock[it.id]*10)/10)
		} else {
			q = min(q, math.Floor(g.stock[it.id]))
		}
		if q <= 0 {
			continue
		}
		in := sales.ItemInput{ProductID: it.id, Quantity: fmt.Sprint(q)}
		if it.warranty > 0 {
			sn, err := g.serial(it)
			if err != nil {
				return err
			}
			in.Serials = []string{sn}
		}
		items = append(items, in)
		gross := it.price.Mul(decimal.NewFromFloat(q))
		if len(it.packs) > 0 {
			gross, _ = sales.PackGross(decimal.NewFromFloat(q), it.price, it.packs, false)
		}
		total = total.Add(gross)
		g.stock[it.id] -= q
	}
	if len(items) == 0 {
		return nil // nothing on the shelf this customer wanted
	}
	in := sales.CreateInput{SaleType: "retail", Items: items}
	switch x := g.r.IntN(100); {
	case x < 4 && len(g.s.customers) > 0 && g.creditRoom(g.s.customers[x%len(g.s.customers)], total):
		cid := g.s.customers[x%len(g.s.customers)]
		in.CustomerID = &cid
		in.Payments = []sales.PaymentInput{{Method: sales.MethodOnAccount, Amount: total.String()}}
	case x < 24:
		in.Payments = []sales.PaymentInput{{Method: "card", Amount: total.String(), AccountID: g.s.card}}
	default:
		in.Payments = []sales.PaymentInput{{Method: "cash", Amount: tendered(g.r, total).String()}}
	}
	det, err := g.d.Svc.Sales.Create(g.ctx, in, c.id)
	if err != nil {
		return fmt.Errorf("sale: %w", err)
	}
	g.daySales++
	g.dayTakings = g.dayTakings.Add(det.Sale.Total)
	return nil
}

// creditRoom: a regular only buys on account within their limit (the till would
// stop it otherwise; the cheat menu has a customer who went over).
func (g *gen) creditRoom(id int64, total decimal.Decimal) bool {
	var room decimal.Decimal
	err := g.d.DB.GetContext(g.ctx, &room, `SELECT credit_limit - outstanding_balance FROM customers WHERE id = $1`, id)
	return err == nil && room.GreaterThanOrEqual(total)
}

// tendered is what a customer hands over: exact, or rounded up to the next
// 100, 500 or 1000.
func tendered(r *rand.Rand, total decimal.Decimal) decimal.Decimal {
	steps := []int64{0, 100, 500, 1000}
	step := steps[r.IntN(len(steps))]
	if step == 0 {
		return total
	}
	n := decimal.NewFromInt(step)
	return total.Div(n).Ceil().Mul(n)
}

// weekly is how much of it sells in a typical week at this shop's volume: its
// share of the catalogue's sales × bills a week × ~2.4 lines × ~1.3 each.
func (g *gen) weekly(it item) float64 {
	var sum float64
	for _, x := range g.s.items {
		sum += x.perWeek
	}
	return math.Max(2, it.perWeek/sum*float64(g.s.p.Volume)*7*2.4*1.3)
}

// pick chooses a product, weighted by weekly sales.
func (g *gen) pick() item {
	var sum float64
	for _, it := range g.s.items {
		sum += it.perWeek
	}
	x := g.r.Float64() * sum
	for _, it := range g.s.items {
		if x -= it.perWeek; x <= 0 {
			return it
		}
	}
	return g.s.items[len(g.s.items)-1]
}

func poisson(r *rand.Rand, mean float64) int {
	// Normal approximation: means here are tens to hundreds.
	n := int(math.Round(mean + r.NormFloat64()*math.Sqrt(mean)))
	return max(n, 0)
}

// openBags is a cashier opening a bag at the counter's Convert tab when the
// loose bin runs low: a 25 kg bag of rice weighs out a little short.
func (g *gen) openBags(day time.Time, h float64, c person) error {
	cl, err := g.tillOf(c)
	if err != nil {
		return err
	}
	for _, cv := range g.s.convs {
		from, to := g.s.items[cv.from], g.s.items[cv.to]
		if g.stock[to.id] >= g.weekly(to)*0.35 || g.stock[from.id] < 1 {
			continue
		}
		bags := min(math.Ceil((g.weekly(to)*0.6-g.stock[to.id])/cv.ratio), math.Floor(g.stock[from.id]))
		if bags < 1 {
			continue
		}
		got := bags * cv.ratio
		body := map[string]string{"quantity": fmt.Sprint(bags)}
		if cv.loss > 0 {
			got = math.Round(got*(1-g.r.Float64()*cv.loss)*10) / 10
			body["toQty"] = fmt.Sprint(got)
		}
		g.tm.at(at(day, h+g.r.Float64()*0.2))
		if err := cl.Do("POST", fmt.Sprintf("/api/v1/till/conversions/%d/run", cv.id), body, nil); err != nil {
			return fmt.Errorf("open %s: %w", from.name, err)
		}
		g.stock[from.id] -= bags
		g.stock[to.id] += got
	}
	return nil
}

// serial is a unit's serial number: a phone's IMEI, else the maker's code.
// The count of units so far keeps every one unique across runs; × a prime
// scatters them so they don't read 1, 2, 3.
func (g *gen) serial(it item) (string, error) {
	var n int
	if err := g.d.DB.GetContext(g.ctx, &n, `SELECT count(*) FROM warranty_units`); err != nil {
		return "", err
	}
	n++ // every unit sold or swapped adds one, so this is never used twice
	if it.imei {
		return fmt.Sprintf("35%06d%07d", 100000+it.id*37%900000, n*7919%10000000), nil
	}
	abbr := ""
	for _, w := range strings.Fields(it.name) {
		if r := w[0]; r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			abbr += string(r)
		}
	}
	return fmt.Sprintf("%s%d%06d", abbr, 2+it.id%7, n*7919%1000000), nil
}
