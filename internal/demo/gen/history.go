package gen

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
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

	admin    *Client         // the owner, for the back office
	owed     map[int][]due   // deliveries not yet paid, by supplier
	safeCash decimal.Decimal // what the generator thinks is in the safe

	tills map[int64]*Client // signed-in till clients, by cashier
	open  []person          // who is on a till today
	jobs  []*repairJob      // repairs still moving
	ctx   context.Context
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
		progress(10+70*(days-start)/days, "A year of trade")
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
	if day.Weekday() == time.Monday || ago == g.days {
		if err := g.restock(day, ago == g.days); err != nil {
			return err
		}
	}
	tills := g.s.cashiers[:2]
	if ago%3 == 0 && len(g.s.cashiers) > 2 {
		tills = []person{g.s.cashiers[0], g.s.cashiers[2]}
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
	for _, h := range hours {
		g.tm.at(at(day, h))
		if err := g.sale(tills[g.r.IntN(len(tills))]); err != nil {
			return err
		}
	}
	if err := g.pluginsDaily(day, until); err != nil {
		return err
	}
	if until < 20.5 {
		return nil // today: the tills stay open
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
		g.safeCash = g.safeCash.Add(counted)
	}
	return g.backOffice(day)
}

// restock orders about two weeks of each product that's running low, from its
// supplier; the first delivery fills the empty shop.
func (g *gen) restock(day time.Time, opening bool) error {
	bySup := map[int][]purchases.ItemInput{}
	for _, it := range g.s.items {
		week := g.weekly(it)
		want := week * 2.2
		if opening {
			want = week * 3
		}
		have := g.stock[it.id]
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
		if seen[it.id] || g.stock[it.id] < 1 { // sold out until the next delivery
			continue
		}
		seen[it.id] = true
		q := 1.0
		if it.unit == "kg" && g.r.IntN(3) == 0 {
			q = 0.5 * float64(1+g.r.IntN(4))
		} else if g.r.IntN(6) == 0 {
			q = float64(2 + g.r.IntN(3))
		}
		q = min(q, g.stock[it.id])
		items = append(items, sales.ItemInput{ProductID: it.id, Quantity: fmt.Sprint(q)})
		total = total.Add(it.price.Mul(decimal.NewFromFloat(q)))
		g.stock[it.id] -= q
	}
	if len(items) == 0 {
		return nil // nothing on the shelf this customer wanted
	}
	in := sales.CreateInput{SaleType: "retail", Items: items}
	switch x := g.r.IntN(100); {
	case x < 4 && len(g.s.customers) > 0:
		cid := g.s.customers[g.r.IntN(len(g.s.customers))]
		in.CustomerID = &cid
		in.AllowOverLimit = true
		in.Payments = []sales.PaymentInput{{Method: sales.MethodOnAccount, Amount: total.String()}}
	case x < 24:
		in.Payments = []sales.PaymentInput{{Method: "card", Amount: total.String(), AccountID: g.s.card}}
	default:
		in.Payments = []sales.PaymentInput{{Method: "cash", Amount: tendered(g.r, total).String()}}
	}
	if _, err := g.d.Svc.Sales.Create(g.ctx, in, c.id); err != nil {
		return fmt.Errorf("sale: %w", err)
	}
	return nil
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
