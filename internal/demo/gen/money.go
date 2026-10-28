package gen

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"time"

	"github.com/shopspring/decimal"

	"karots-suite/internal/features/lockers"
)

// The back office, through the admin API as the owner: suppliers paid when
// their credit runs out, credit customers paying in, the month's bills, a few
// returns and breakages. Without it a year of dues would pile up.

type due struct {
	day    time.Time
	amount decimal.Decimal
}

func (g *gen) safeSrc() string { return "locker:" + strconv.FormatInt(g.s.safe, 10) }
func (g *gen) bankSrc() string { return "locker:" + strconv.FormatInt(g.s.bank, 10) }

// balance is what an account really holds now (the ledger, not a guess).
func (g *gen) balance(id int64) decimal.Decimal {
	l, err := lockers.NewService(g.d.DB).Get(g.ctx, id)
	if err != nil {
		return decimal.Zero
	}
	return l.Balance
}

// payFrom picks where money for amt comes from: the safe while it keeps a float,
// else the bank; ok false when neither has it (the owner waits a week).
func (g *gen) payFrom(amt decimal.Decimal) (method, src string, ok bool) {
	if g.balance(g.s.safe).GreaterThanOrEqual(amt.Add(decimal.NewFromInt(20000))) {
		return "cash", g.safeSrc(), true
	}
	if g.balance(g.s.bank).GreaterThanOrEqual(amt) {
		return "online", g.bankSrc(), true
	}
	return "", "", false
}

// backOffice runs after a day's trade.
func (g *gen) backOffice(day time.Time) error {
	if day.Weekday() == time.Monday {
		if err := g.paySuppliers(day); err != nil {
			return err
		}
	}
	if day.Weekday() == time.Tuesday {
		if err := g.bankTheCash(day); err != nil {
			return err
		}
	}
	if day.Day() == 2 {
		if err := g.customersPay(day); err != nil {
			return err
		}
	}
	if err := g.bills(day); err != nil {
		return err
	}
	if g.r.IntN(4) == 0 {
		if err := g.aReturn(day); err != nil {
			return err
		}
	}
	return g.backRoom(day)
}

// paySuppliers settles each supplier's deliveries older than 30 days — now and
// then the whole statement when the rep calls — cash from the safe while it
// holds enough, otherwise from the bank.
func (g *gen) paySuppliers(day time.Time) error {
	for _, sup := range slices.Sorted(maps.Keys(g.owed)) { // a fixed order: the same seed builds the same shop
		list := g.owed[sup]
		whole := g.r.IntN(6) == 0 // the rep's here: the whole statement, this week's drops too
		total := decimal.Zero
		keep := list[:0]
		for _, d := range list {
			if day.Sub(d.day) >= 30*24*time.Hour || whole {
				total = total.Add(d.amount)
			} else {
				keep = append(keep, d)
			}
		}
		if total.IsZero() {
			g.owed[sup] = keep
			continue
		}
		method, src, ok := g.payFrom(total)
		if !ok {
			continue // not this week: the deliveries stay owed
		}
		g.owed[sup] = keep
		g.tm.at(at(day, 10+g.r.Float64()))
		if err := g.admin.Do("POST", fmt.Sprintf("/api/v1/admin/suppliers/%d/payment", g.s.suppliers[sup]),
			map[string]string{"amount": total.StringFixed(2), "method": method, "source": src, "note": "Monthly settlement"}, nil); err != nil {
			return fmt.Errorf("pay supplier: %w", err)
		}
		if whole && g.r.IntN(2) == 0 { // paid up: on his next call he takes back what didn't sell
			s := sup
			g.later = append(g.later, later{day.AddDate(0, 0, 1+g.r.IntN(3)), func(d time.Time) error { return g.sendBack(d, s) }})
		}
	}
	return nil
}

// bankTheCash deposits what the safe holds beyond a float for the week, as a shop
// does — the bank pays the rent, the salaries and the dealers.
func (g *gen) bankTheCash(day time.Time) error {
	keep := decimal.NewFromInt(50000)
	amt := g.balance(g.s.safe).Sub(keep).Truncate(-3)
	if !amt.IsPositive() {
		return nil
	}
	g.tm.at(at(day, 10.5))
	if err := g.admin.Do("POST", "/api/v1/admin/lockers/transfer", map[string]any{"fromId": g.s.safe, "toId": g.s.bank,
		"amount": amt.StringFixed(2), "note": "Weekly bank deposit"}, nil); err != nil {
		return fmt.Errorf("bank deposit: %w", err)
	}
	return nil
}

// customersPay: on the 2nd, most credit customers pay most of what they owe.
func (g *gen) customersPay(day time.Time) error {
	for _, id := range g.s.customers {
		var owed decimal.Decimal
		if err := g.d.DB.GetContext(g.ctx, &owed, `SELECT outstanding_balance FROM customers WHERE id = $1`, id); err != nil {
			return err
		}
		if !owed.IsPositive() || g.r.IntN(5) == 0 { // one in five lets it ride
			continue
		}
		pay := owed.Mul(decimal.NewFromFloat(0.6 + 0.4*g.r.Float64())).Round(-2)
		if !pay.IsPositive() {
			continue
		}
		g.tm.at(at(day, 9+3*g.r.Float64()))
		if err := g.admin.Do("POST", fmt.Sprintf("/api/v1/admin/customers/%d/payment", id),
			map[string]string{"amount": pay.StringFixed(2), "method": "cash", "source": g.safeSrc()}, nil); err != nil {
			return fmt.Errorf("customer payment: %w", err)
		}
	}
	return nil
}

// bills are the month's running costs, scaled to the shop's size.
func (g *gen) bills(day time.Time) error {
	scale := float64(g.s.p.Volume) / 100
	type bill struct {
		cat, what string
		amount    float64
	}
	var due []bill
	switch day.Day() {
	case 1:
		due = append(due, bill{"Rent", "Shop rent", 85000 * scale})
	case 10:
		due = append(due, bill{"Electricity", "CEB bill", (16000 + 8000*g.r.Float64()) * scale})
	case 12:
		due = append(due, bill{"Water", "Water board", 2200 + 800*g.r.Float64()})
	case 28:
		for _, c := range g.s.cashiers {
			due = append(due, bill{"Salaries", "Salary — " + c.login.Name, 52000})
		}
	}
	if day.Weekday() == time.Friday {
		due = append(due, bill{"Transport", "Three-wheeler hire", 1500 + 2500*g.r.Float64()})
	}
	for _, b := range due {
		amt := decimal.NewFromFloat(b.amount).Round(0)
		_, src, ok := g.payFrom(amt)
		if !ok {
			continue // a lean week: this bill waits
		}
		g.tm.at(at(day, 11+4*g.r.Float64()))
		if err := g.admin.Do("POST", "/api/v1/admin/expenses",
			map[string]string{"category": b.cat, "amount": amt.String(), "description": b.what, "source": src}, nil); err != nil {
			return fmt.Errorf("expense %s: %w", b.cat, err)
		}
	}
	return nil
}

// aReturn takes back one line of one of today's sales — changed mind, wrong size.
func (g *gen) aReturn(day time.Time) error {
	var line struct {
		Sale int64           `db:"sale_id"`
		Item int64           `db:"id"`
		Qty  decimal.Decimal `db:"quantity"`
	}
	err := g.d.DB.GetContext(g.ctx, &line, `
		SELECT si.sale_id, si.id, si.quantity - si.returned_qty AS quantity FROM sale_items si JOIN sales s ON s.id = si.sale_id
		JOIN products p ON p.id = si.product_id
		WHERE s.created_at >= $1 AND s.status = 'completed' AND s.customer_id IS NULL AND si.quantity > si.returned_qty
		  AND NOT p.is_service AND NOT p.track_serial AND si.unit_price > 0 -- goods come back; a reload, a cash out or a phone (warranty) don't
		ORDER BY si.id DESC OFFSET $2 LIMIT 1`, g.tm.start, g.r.IntN(20))
	if err != nil {
		return nil // nothing sold yet this batch
	}
	g.tm.at(at(day, 18+2*g.r.Float64()))
	reasons := []string{"Changed mind", "Wrong size", "Damaged packet", "Bought twice"}
	return g.admin.Do("POST", fmt.Sprintf("/api/v1/admin/sales/%d/return", line.Sale), map[string]any{
		"reason": reasons[g.r.IntN(len(reasons))], "source": g.safeSrc(),
		"lines": []map[string]any{{"saleItemId": line.Item, "quantity": decimal.Min(line.Qty, decimal.NewFromInt(1)).String(), "disposition": "restock"}},
	}, nil)
}
