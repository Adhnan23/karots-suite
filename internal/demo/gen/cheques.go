package gen

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// The shop's cheques, written at today's open through the admin API: customers'
// cheques in hand, cleared, bounced; the shop's own cheques to suppliers waiting
// at the bank, cleared, cancelled — so the Cheques page and the dashboard's
// reminders have something in every state. Today trades at the real clock, so
// a date k days from today is just that (the time machine moves it by 0).

var chequeBanks = []string{"Bank of Ceylon", "Commercial Bank", "Sampath Bank", "People's Bank", "Hatton National Bank"}

type chequePlan struct {
	in      bool // received from a customer, else given to a supplier
	dated   int  // days from today
	outcome string
	on      int // clear: days from today
	amount  int64
}

var chequeBook = []chequePlan{
	{in: true, dated: 0, amount: 12500},
	{in: true, dated: -4, amount: 8750},
	{in: true, dated: 12, amount: 25000},
	{in: true, dated: -14, outcome: "cleared", on: -11, amount: 18000},
	{in: true, dated: -9, outcome: "cleared", on: -6, amount: 6400},
	{in: true, dated: -5, outcome: "bounced", amount: 15500},
	{dated: 2, amount: 32000},
	{dated: 10, amount: 21500},
	{dated: -10, outcome: "cleared", on: -7, amount: 27000},
	{dated: -3, outcome: "cancelled", amount: 9800},
}

func (g *gen) cheques(day time.Time, hour float64) error {
	if len(g.s.customers) == 0 || len(g.s.suppliers) == 0 {
		return nil
	}
	g.tm.at(at(day, hour))
	ymd := func(k int) string { return day.AddDate(0, 0, k).Format(time.DateOnly) }
	for i, c := range chequeBook {
		no := fmt.Sprintf("%06d", 100000+g.r.IntN(900000))
		amt := decimal.NewFromInt(c.amount).StringFixed(2)
		line := map[string]any{"number": no, "amount": amt, "date": ymd(c.dated)}
		path, dir := fmt.Sprintf("/api/v1/admin/suppliers/%d/payment", g.s.suppliers[i%len(g.s.suppliers)]), "out"
		if c.in {
			line["bank"] = chequeBanks[i%len(chequeBanks)]
			path, dir = fmt.Sprintf("/api/v1/admin/customers/%d/payment", g.s.customers[i%len(g.s.customers)]), "in"
		} else {
			line["lockerId"] = g.s.bank
		}
		if err := g.admin.Do("POST", path, map[string]any{"amount": amt, "method": "cheque", "cheques": []any{line}}, nil); err != nil {
			return fmt.Errorf("cheque %s: %w", no, err)
		}
		if c.outcome == "" {
			continue
		}
		var id int64
		if err := g.d.DB.GetContext(g.ctx, &id, `SELECT id FROM cheques WHERE direction = $1 AND number = $2 ORDER BY id DESC LIMIT 1`, dir, no); err != nil {
			return err
		}
		var err error
		if c.outcome == "cleared" {
			err = g.admin.Do("POST", fmt.Sprintf("/api/v1/admin/cheques/%d/clear", id), map[string]any{"date": ymd(c.on), "lockerId": g.s.bank}, nil)
		} else {
			err = g.admin.Do("POST", fmt.Sprintf("/api/v1/admin/cheques/%d/return", id), map[string]string{"status": c.outcome}, nil)
		}
		if err != nil {
			return fmt.Errorf("cheque %s %s: %w", no, c.outcome, err)
		}
	}
	return nil
}
