package gen

import (
	"fmt"
	"strconv"
	"time"

	"github.com/shopspring/decimal"

	"karots-suite/internal/features/lockers"
)

// Recharge: four carriers with a reload SIM each (Dialog also an eZ Cash wallet),
// their packages, a stream of reloads and data packs at the till, bill payments
// through the wallet, and the floats topped up from the dealer every Monday.

type rechargeTill struct {
	Carriers []struct {
		ID        int64  `json:"id"`
		Name      string `json:"name"`
		ProductID int64  `json:"productId"`
	} `json:"carriers"`
	Floats []struct {
		ID        int64  `json:"id"`
		CarrierID int64  `json:"carrierId"`
		Carrier   string `json:"carrier"`
		Label     string `json:"label"`
		Reload    bool   `json:"reload"`
		Money     bool   `json:"money"`
		Balance   string `json:"balance"`
		Target    string `json:"target"`
	} `json:"floats"`
	Packages []struct {
		ID        int64  `json:"id"`
		CarrierID int64  `json:"carrierId"`
		Name      string `json:"name"`
		Price     string `json:"price"`
	} `json:"packages"`
	Products struct {
		Billpay int64 `json:"billpay"`
		Cashout int64 `json:"cashout"`
		Charge  int64 `json:"charge"`
	} `json:"products"`
	Banks []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"banks"`
}

var carriers = []struct {
	name, sim string
	packs     [][2]string // name, price
}{
	{"Dialog", "0771", [][2]string{{"Data 1GB", "199"}, {"Data 5GB", "649"}, {"Anytime 30 days", "1299"}}},
	{"Mobitel", "0711", [][2]string{{"Data 2GB", "249"}, {"Unlimited YouTube", "549"}}},
	{"Airtel", "0751", [][2]string{{"Data 3GB", "299"}}},
	{"Hutch", "0781", [][2]string{{"Data 4GB", "349"}}},
}

func (g *gen) rechargeData() (rechargeTill, error) {
	var d rechargeTill
	err := g.admin.Do("GET", "/api/v1/x/recharge/till", nil, &d)
	return d, err
}

func rechargeSetup(g *gen) error {
	// the shop's own banking app: bills paid from it, and cash outs into it
	if _, err := lockers.NewService(g.d.DB).Create(g.ctx, lockers.CreateInput{Name: "Sampath Bank — shop app", Kind: lockers.KindBank,
		OpeningBalance: "180000", CashierAccess: "on"}, g.s.owner.id); err != nil {
		return fmt.Errorf("bank app: %w", err)
	}
	dealer := g.s.suppliers[len(g.s.suppliers)-1] // "SIM & Reload Agency" in the phone shop
	for _, c := range carriers {
		var out struct {
			ID int64 `json:"id"`
		}
		if err := g.admin.Do("POST", "/api/v1/x/recharge/carriers", map[string]string{"name": c.name}, &out); err != nil {
			return err
		}
		base := fmt.Sprintf("/api/v1/x/recharge/carriers/%d", out.ID)
		if err := g.admin.Do("PUT", base+"/supplier", map[string]int64{"supplierId": dealer}, nil); err != nil {
			return err
		}
		if err := g.admin.Do("PUT", base+"/rates", []map[string]string{
			{"kind": "reload", "percent": "4", "fixed": "0", "charge": "0"},
			{"kind": "billpay", "percent": "0", "fixed": "10", "charge": "50"},
		}, nil); err != nil {
			return err
		}
		if err := g.admin.Do("POST", "/api/v1/x/recharge/devices", map[string]any{"carrierId": out.ID, "label": "Reload SIM",
			"number": c.sim + "234567", "reload": true, "money": false, "target": "25000", "opening": "30000"}, nil); err != nil {
			return err
		}
		if c.name == "Dialog" {
			if err := g.admin.Do("POST", "/api/v1/x/recharge/devices", map[string]any{"carrierId": out.ID, "label": "eZ Cash",
				"number": "0772345678", "reload": false, "money": true, "target": "50000", "opening": "40000"}, nil); err != nil {
				return err
			}
		}
		for i, p := range c.packs {
			price := decimal.RequireFromString(p[1])
			if err := g.admin.Do("POST", "/api/v1/x/recharge/packages", map[string]any{"carrierId": out.ID, "name": p[0], "price": p[1],
				"floatCost": price.Mul(decimal.NewFromFloat(0.97)).Round(0).String(), "sort": i}, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

func rechargeDaily(g *gen, day time.Time, until float64) error {
	d, err := g.rechargeData()
	if err != nil {
		return err
	}
	if day.Weekday() == time.Monday && until > 10.6 { // the dealer comes mid-morning
		if err := g.refillFloats(day, d); err != nil {
			return err
		}
	}
	if day.Weekday() == time.Saturday && until > 19.8 { // the week's float check
		if err := g.countFloats(day); err != nil {
			return err
		}
	}
	n := 5 + g.r.IntN(10)
	if g.on("pharmacy") { // the Everything shop sells fewer
		n /= 2
	}
	for i := 0; i < n; i++ {
		h := 8 + g.r.Float64()*12
		if h >= until {
			continue
		}
		g.tm.at(at(day, h))
		if err := g.rechargeSale(d); err != nil {
			return err
		}
	}
	return nil
}

// rechargeSale is a reload, a data pack or a bill paid through the wallet.
func (g *gen) rechargeSale(d rechargeTill) error {
	tag := func(t map[string]any) map[string]any { return map[string]any{"recharge": t} }
	c := g.cashier()
	fl := d.Floats[g.r.IntN(len(d.Floats))]
	var productID int64
	for _, c := range d.Carriers {
		if c.ID == fl.CarrierID {
			productID = c.ProductID
		}
	}
	number := fmt.Sprintf("07%d%07d", 1+g.r.IntN(8), g.r.IntN(10000000))
	line := map[string]any{"productId": productID, "quantity": "1", "discount": "0", "discountType": "fixed"}
	items := []map[string]any{line}
	var total decimal.Decimal
	switch {
	case len(d.Banks) > 0 && g.r.IntN(12) == 0: // through the shop's banking app
		bank := d.Banks[0]
		tagBank := func(kind string, extra map[string]any) map[string]any {
			t := map[string]any{"kind": kind, "bank": bank.ID, "label": bank.Name}
			for k, v := range extra {
				t[k] = v
			}
			return tag(t)
		}
		if g.r.IntN(2) == 0 { // a bill paid in the app, the customer pays in cash
			amt := decimal.NewFromInt(int64(2000 + g.r.IntN(80)*100))
			bill := []string{"Electricity (CEB)", "Water (NWSDB)", "SLT broadband", "Leasing instalment"}[g.r.IntN(4)]
			ref := fmt.Sprintf("%08d", g.r.IntN(100000000))
			line["productId"], line["priceOverride"], line["description"] = d.Products.Billpay, amt.String(), bill+" · "+ref
			line["plugin"] = tagBank("billpay", map[string]any{"billType": bill, "ref": ref})
			items = append(items, map[string]any{"productId": d.Products.Charge, "quantity": "1", "priceOverride": "50",
				"description": "Service charge · " + bill, "discount": "0", "discountType": "fixed", "plugin": tag(map[string]any{"kind": "charge"})})
			total = amt.Add(decimal.NewFromInt(50))
			break
		}
		// cash out: the customer sends money to the shop's account and takes it in
		// cash — only when this drawer can cover it
		amt := decimal.NewFromInt(int64(2000 + g.r.IntN(37)*500))
		sum, err := g.d.Svc.CashRegister.Summary(g.ctx, c.id)
		if err != nil || sum.Expected.LessThan(amt.Add(decimal.NewFromInt(3000))) {
			return nil
		}
		ref := fmt.Sprintf("TRF%07d", g.r.IntN(10000000))
		line["productId"], line["payout"], line["priceOverride"], line["description"] = d.Products.Cashout, true, amt.String(), "Cash out · "+ref
		line["plugin"] = tagBank("cashout", map[string]any{"ref": ref})
		items = append(items, map[string]any{"productId": d.Products.Charge, "quantity": "1", "priceOverride": "100",
			"description": "Service charge · Cash out", "discount": "0", "discountType": "fixed", "plugin": tag(map[string]any{"kind": "charge"})})
		total = decimal.NewFromInt(100).Sub(amt)
	case fl.Money: // a CEB / water bill paid from the wallet, plus the service charge
		amt := decimal.NewFromInt(int64(1500 + g.r.IntN(60)*100))
		bill := []string{"Electricity (CEB)", "Water (NWSDB)", "SLT broadband"}[g.r.IntN(3)]
		ref := fmt.Sprintf("04%08d", g.r.IntN(100000000))
		line["productId"] = d.Products.Billpay
		line["priceOverride"], line["description"] = amt.String(), bill+" · "+ref
		line["plugin"] = tag(map[string]any{"kind": "billpay", "device": fl.ID, "label": fl.Carrier + " · " + fl.Label, "billType": bill, "ref": ref})
		items = append(items, map[string]any{"productId": d.Products.Charge, "quantity": "1", "priceOverride": "50",
			"description": "Service charge · " + bill, "discount": "0", "discountType": "fixed", "plugin": tag(map[string]any{"kind": "charge"})})
		total = amt.Add(decimal.NewFromInt(50))
	case g.r.IntN(3) == 0 && len(d.Packages) > 0: // a data pack on this carrier
		for _, p := range d.Packages {
			if p.CarrierID == fl.CarrierID {
				total = dec(p.Price, "0")
				line["priceOverride"], line["description"] = p.Price, fl.Carrier+" "+p.Name+" · "+number
				line["plugin"] = tag(map[string]any{"kind": "package", "device": fl.ID, "package": p.ID, "label": fl.Carrier + " · " + fl.Label, "number": number})
				break
			}
		}
	}
	if total.IsZero() { // a plain reload
		amt := []int64{50, 100, 100, 200, 300, 500, 1000}[g.r.IntN(7)]
		total = decimal.NewFromInt(amt)
		line["priceOverride"], line["description"] = total.String(), fl.Carrier+" reload · "+number
		line["plugin"] = tag(map[string]any{"kind": "reload", "device": fl.ID, "label": fl.Carrier + " · " + fl.Label, "number": number})
	}
	_, err := g.tillSale(c, 0, items, total)
	return err
}

// refillFloats tops each float back up to its target from the dealer, paid from the safe.
func (g *gen) refillFloats(day time.Time, d rechargeTill) error {
	dealer := g.s.suppliers[len(g.s.suppliers)-1]
	for _, f := range d.Floats {
		need := dec(f.Target, "0").Sub(dec(f.Balance, "0")).Round(-3)
		if need.LessThan(decimal.NewFromInt(2000)) {
			continue
		}
		_, src, ok := g.payFrom(need)
		if !ok {
			continue // tops up next week
		}
		g.tm.at(at(day, 9.5+g.r.Float64()))
		if err := g.admin.Do("POST", "/api/v1/x/recharge/refill", map[string]any{"deviceId": f.ID, "supplierId": dealer,
			"amount": need.String(), "bonus": "0", "credit": false, "source": src,
			"reference": "DLR-" + strconv.Itoa(day.YearDay())}, nil); err != nil {
			return fmt.Errorf("refill %s: %w", f.Label, err)
		}
	}
	return nil
}

// countFloats is the owner checking each SIM's balance on the phone on
// Saturday night: mostly right, now and then a reload that didn't go through.
func (g *gen) countFloats(day time.Time) error {
	d, err := g.rechargeData()
	if err != nil {
		return err
	}
	var counts []map[string]any
	for _, f := range d.Floats {
		bal := dec(f.Balance, "0")
		if bal.LessThan(decimal.NewFromInt(500)) {
			continue // all but empty: it gets topped up, not counted
		}
		switch g.r.IntN(6) {
		case 0:
			bal = bal.Sub(decimal.NewFromInt(int64(50 * (1 + g.r.IntN(4))))) // a reload sent twice
		case 1:
			bal = bal.Add(decimal.NewFromInt(int64(10 * (1 + g.r.IntN(5))))) // a bonus from the carrier
		}
		counts = append(counts, map[string]any{"deviceId": f.ID, "counted": bal.StringFixed(2)})
	}
	if len(counts) == 0 {
		return nil
	}
	g.tm.at(at(day, 19.7))
	return g.admin.Do("POST", "/api/v1/x/recharge/counts", map[string]any{"counts": counts}, nil)
}
