package gen

import (
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// Repairs: phones and laptops come in every day; each goes in progress, gets its
// labour and maybe a part, is ready a few days later and is collected at the till
// (its bill on the cart, as the Repairs panel puts it). Recent jobs are still open.

type repairJob struct {
	id, customer int64
	ticket       string
	next         time.Time // when its next step is due
	step         string    // in_progress → ready → collect
	advance      decimal.Decimal
}

var (
	repairDevices = []string{"Samsung Galaxy A14", "Redmi Note 12", "iPhone 11", "Huawei Y9", "Nokia G21", "Oppo A57", "Vivo Y21", "Lenovo IdeaPad 3", "HP 250 G8", "Samsung Tab A8"}
	repairFaults  = map[string][]string{
		"Screen":   {"Cracked screen", "Touch not working", "Lines on display"},
		"Battery":  {"Drains fast", "Swollen battery", "Won't hold charge"},
		"Charging": {"Not charging", "Loose charging port"},
		"Software": {"Stuck on logo", "Forgot pattern lock", "Slow, needs reset"},
		"Water":    {"Fell in water", "No sound after rain"},
	}
	repairTypes = []string{"Screen", "Battery", "Charging", "Software", "Water"}
	repairers   = []string{"Sahan (in-house)", "Nuwan Mobile Lab", "Sahan (in-house)"}
	labourFor   = map[string][2]int{"Screen": {2500, 6500}, "Battery": {1500, 3000}, "Charging": {1500, 3500}, "Software": {1000, 2500}, "Water": {3000, 7500}}
)

func repairsSetup(g *gen) error {
	return g.admin.Do("PUT", "/api/v1/x/repairs/config", map[string]any{"warrantyMode": "days", "defaultWarrantyDays": 30}, nil)
}

func repairsDaily(g *gen, day time.Time, until float64) error {
	n := 1 + g.r.IntN(3)
	if !g.on("recharge") { // the Everything shop takes fewer
		n = g.r.IntN(2)
	}
	for i := 0; i < n; i++ {
		h := 9 + g.r.Float64()*9
		if h >= until {
			continue
		}
		if err := g.repairIn(day, h); err != nil {
			return err
		}
	}
	keep := g.jobs[:0]
	for _, j := range g.jobs {
		if j.next.After(at(day, until)) {
			keep = append(keep, j)
			continue
		}
		done, err := g.repairStep(day, j)
		if err != nil {
			return err
		}
		if !done {
			keep = append(keep, j)
		}
	}
	g.jobs = keep
	return nil
}

func (g *gen) repairIn(day time.Time, h float64) error {
	typ := repairTypes[g.r.IntN(len(repairTypes))]
	faults := repairFaults[typ]
	cust := g.customer()
	g.tm.at(at(day, h))
	var out struct {
		ID       int64  `json:"id"`
		TicketNo string `json:"ticketNo"`
	}
	err := g.admin.Do("POST", "/api/v1/x/repairs/jobs", map[string]any{
		"customerId": cust, "repairType": typ, "deviceModel": repairDevices[g.r.IntN(len(repairDevices))],
		"repairedBy": repairers[g.r.IntN(len(repairers))], "fault": faults[g.r.IntN(len(faults))], "notes": "",
		"promisedDate": day.AddDate(0, 0, 2+g.r.IntN(3)).Format("2006-01-02"), "urgent": g.r.IntN(8) == 0,
	}, &out)
	if err != nil {
		return fmt.Errorf("new job: %w", err)
	}
	j := &repairJob{id: out.ID, customer: cust, ticket: out.TicketNo, next: at(day, h+2+g.r.Float64()*20), step: "in_progress"}
	g.jobs = append(g.jobs, j)
	if (typ == "Screen" || typ == "Water") && g.r.IntN(2) == 0 { // a part to order: they leave an advance
		return g.repairAdvance(j)
	}
	return nil
}

// repairAdvance takes money towards the repair at the counter (a customer
// payment, as the credit tab takes it) and ties it to the job.
func (g *gen) repairAdvance(j *repairJob) error {
	cl, err := g.tillOf(g.cashier())
	if err != nil {
		return err
	}
	amt := decimal.NewFromInt(int64(1000 * (1 + g.r.IntN(3))))
	var pay struct {
		PaymentID int64 `json:"paymentId"`
	}
	if err := cl.Do("POST", fmt.Sprintf("/api/v1/till/customers/%d/payment", j.customer), map[string]any{
		"amount": amt.String(), "method": "cash", "note": "Advance for repair " + j.ticket}, &pay); err != nil {
		return fmt.Errorf("repair advance: %w", err)
	}
	if err := g.admin.Do("POST", fmt.Sprintf("/api/v1/x/repairs/jobs/%d/deposits", j.id), map[string]int64{"paymentId": pay.PaymentID}, nil); err != nil {
		return fmt.Errorf("link advance: %w", err)
	}
	j.advance = amt
	return nil
}

// repairStep moves a job on; true when it's been collected (or given up on).
func (g *gen) repairStep(day time.Time, j *repairJob) (bool, error) {
	h := max(9+g.r.Float64()*8, float64(j.next.Sub(day).Hours()))
	if h > 20 {
		h = 19.5
	}
	g.tm.at(at(day, h))
	path := fmt.Sprintf("/api/v1/x/repairs/jobs/%d", j.id)
	switch j.step {
	case "in_progress":
		if err := g.admin.Do("POST", path+"/status", map[string]string{"status": "in_progress"}, nil); err != nil {
			return false, err
		}
		j.step, j.next = "ready", at(day, h+24*(1+g.r.Float64()*3))
	case "ready":
		var job struct {
			RepairType string `json:"repairType"`
			RepairedBy string `json:"repairedBy"`
		}
		if err := g.admin.Do("GET", path, nil, &job); err != nil {
			return false, err
		}
		lo, hi := labourFor[job.RepairType][0], labourFor[job.RepairType][1]
		if hi == 0 {
			lo, hi = 1500, 4000
		}
		amt := (lo + g.r.IntN(hi-lo+1)) / 100 * 100
		if err := g.admin.Do("POST", path+"/charges", map[string]string{"label": "Labour — " + job.RepairType, "amount": fmt.Sprint(amt)}, nil); err != nil {
			return false, err
		}
		if !strings.Contains(job.RepairedBy, "in-house") { // the outside lab is paid its share from the safe
			if err := g.admin.Do("POST", path+"/payouts", map[string]string{"amount": fmt.Sprint(amt * 55 / 100 / 50 * 50),
				"source": g.safeSrc(), "note": "Paid " + job.RepairedBy}, nil); err != nil {
				return false, fmt.Errorf("lab payout: %w", err)
			}
		}
		if part, ok := g.repairPart(job.RepairType); ok && g.stock[part.id] >= 1 {
			g.stock[part.id]--
			if err := g.admin.Do("POST", path+"/parts", map[string]any{"productId": part.id, "qty": "1"}, nil); err != nil {
				return false, err
			}
		}
		if err := g.admin.Do("POST", path+"/status", map[string]string{"status": "ready"}, nil); err != nil {
			return false, err
		}
		j.step, j.next = "collect", at(day, h+24*(g.r.Float64()*4))
	case "collect":
		return true, g.collect(path, j)
	}
	return false, nil
}

func (g *gen) repairPart(typ string) (item, bool) {
	switch typ {
	case "Battery":
		return g.itemNamed("Phone Battery")
	case "Charging":
		return g.itemNamed("Charging Port")
	}
	return item{}, false
}

// collect puts the job's bill on the cart and pays for it, as the Repairs panel does.
func (g *gen) collect(path string, j *repairJob) error {
	var bill struct {
		CustomerID *int64 `json:"customerId"`
		Lines      []struct {
			Kind, Name, Qty, Price, Discount, DiscountType string
			ProductID                                      int64 `json:"productId"`
		} `json:"lines"`
	}
	if err := g.admin.Do("GET", path+"/bill", nil, &bill); err != nil {
		return err
	}
	tag := map[string]any{"repairs": map[string]any{"job": j.id, "ticket": j.ticket}}
	var items []map[string]any
	total := decimal.Zero
	for _, l := range bill.Lines {
		price, qty := dec(l.Price, "0"), dec(l.Qty, "1")
		if l.Kind != "labour" { // a part sells at its shelf price
			price = g.priceOf(l.ProductID)
		}
		it := map[string]any{"productId": l.ProductID, "quantity": qty.String(), "discount": "0", "discountType": "fixed", "plugin": tag}
		if l.Kind == "labour" {
			it["priceOverride"], it["description"] = price.String(), l.Name
		}
		items = append(items, it)
		total = total.Add(price.Mul(qty))
	}
	if len(items) == 0 {
		return nil
	}
	cust := int64(0)
	if bill.CustomerID != nil {
		cust = *bill.CustomerID
	}
	if j.advance.IsZero() || cust == 0 {
		_, err := g.tillSale(g.cashier(), cust, items, total)
		return err
	}
	// the advance they left comes off on their account; the rest in cash
	cl, err := g.tillOf(g.cashier())
	if err != nil {
		return err
	}
	g.lastTill = cl
	var room decimal.Decimal // an advance first pays off what they already owed
	if err := g.d.DB.GetContext(g.ctx, &room, `SELECT credit_limit - outstanding_balance FROM customers WHERE id = $1`, cust); err != nil {
		return err
	}
	adv := decimal.Min(j.advance, total, room)
	if !adv.IsPositive() {
		_, err := g.tillSale(g.cashier(), cust, items, total)
		return err
	}
	pays := []any{map[string]any{"method": "credit", "amount": adv.String()}}
	if rest := total.Sub(adv); rest.IsPositive() {
		pays = append(pays, map[string]any{"method": "cash", "amount": rest.String()}) // exact: change can't come off an account
	}
	return cl.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
		"items": items, "payments": pays, "customerId": cust}, nil)
}

// dec reads an API money / quantity string; "" is def.
func dec(s, def string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.RequireFromString(def)
	}
	return d
}
