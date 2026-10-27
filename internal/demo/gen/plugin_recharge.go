package gen

import (
	"fmt"
	"strconv"
	"time"

	"github.com/shopspring/decimal"
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
		Charge  int64 `json:"charge"`
	} `json:"products"`
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
	if day.Weekday() == time.Monday {
		if err := g.refillFloats(day, d); err != nil {
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
	_, err := g.tillSale(g.cashier(), 0, items, total)
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
