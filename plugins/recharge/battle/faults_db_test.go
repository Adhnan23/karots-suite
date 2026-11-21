package battle

import (
	"fmt"
	"testing"

	"karots-suite/internal/ledgercheck/ltest"
)

// RCH-1, RCH-2, RCH-3, RCH-7, RCH-8, RCH-10, PLG-3, PLG-5: bad requests are
// refused with a clear 4xx and change nothing.
func TestPLG5_Refusals(t *testing.T) { // compares every table: runs alone
	s := ltest.Shared(t).Shop
	openTill(t, s)
	n := newNet(t, s)
	other := newNet(t, s)
	var carrierName string
	s.Must(t, s.DB.Get(&carrierName, `SELECT name FROM recharge_carriers WHERE id = $1`, other.Carrier))
	var sup struct{ ID int64 }
	s.Must(t, s.Admin.Do("POST", "/api/v1/admin/suppliers", map[string]string{"name": fmt.Sprintf("Refuse dealer %d", seq.Add(1)), "phone": fmt.Sprintf("011%07d", seq.Add(1))}, &sup))
	var cashier int64
	s.Must(t, s.DB.Get(&cashier, `SELECT id FROM users WHERE phone = '0770000002'`))
	sale := func(items ...map[string]any) func() error {
		return func() error { _, err := sell(t, s, items); return err }
	}
	before := ltest.State(t, s.DB)
	for _, x := range []struct {
		name string
		do   func() error
	}{
		{"carrier no name", func() error {
			return s.Admin.Do("POST", "/api/v1/x/recharge/carriers", map[string]string{"name": " "}, nil)
		}},
		{"carrier twice", func() error {
			return s.Admin.Do("POST", "/api/v1/x/recharge/carriers", map[string]string{"name": carrierName}, nil)
		}},
		{"rename to a taken name", func() error {
			return s.Admin.Do("PUT", fmt.Sprintf("/api/v1/x/recharge/carriers/%d", n.Carrier), map[string]string{"name": carrierName}, nil)
		}},
		{"rename no carrier", func() error {
			return s.Admin.Do("PUT", "/api/v1/x/recharge/carriers/999999", map[string]string{"name": "X"}, nil)
		}},
		{"rate over 100%", func() error {
			return s.Admin.Do("PUT", fmt.Sprintf("/api/v1/x/recharge/carriers/%d/rates", n.Carrier), []map[string]string{{"kind": "reload", "percent": "101"}}, nil)
		}},
		{"rate unknown kind", func() error {
			return s.Admin.Do("PUT", fmt.Sprintf("/api/v1/x/recharge/carriers/%d/rates", n.Carrier), []map[string]string{{"kind": "magic", "percent": "1"}}, nil)
		}},
		{"rate negative", func() error {
			return s.Admin.Do("PUT", fmt.Sprintf("/api/v1/x/recharge/carriers/%d/rates", n.Carrier), []map[string]string{{"kind": "reload", "fixed": "-1"}}, nil)
		}},
		{"device no carrier", func() error {
			return s.Admin.Do("POST", "/api/v1/x/recharge/devices", map[string]any{"carrierId": 999999, "label": "X", "reload": true}, nil)
		}},
		{"device for nothing", func() error {
			return s.Admin.Do("POST", "/api/v1/x/recharge/devices", map[string]any{"carrierId": n.Carrier, "label": "X"}, nil)
		}},
		{"device negative opening", func() error {
			return s.Admin.Do("POST", "/api/v1/x/recharge/devices", map[string]any{"carrierId": n.Carrier, "label": "X", "reload": true, "opening": "-5"}, nil)
		}},
		{"edit no device", func() error {
			return s.Admin.Do("PUT", "/api/v1/x/recharge/devices/999999", map[string]any{"carrierId": n.Carrier, "label": "X", "reload": true}, nil)
		}},
		{"package float above price", func() error {
			return s.Admin.Do("POST", "/api/v1/x/recharge/packages", map[string]any{"carrierId": n.Carrier, "name": "Big", "price": "100", "floatCost": "120"}, nil)
		}},
		{"package no price", func() error {
			return s.Admin.Do("POST", "/api/v1/x/recharge/packages", map[string]any{"carrierId": n.Carrier, "name": "Free", "price": "0"}, nil)
		}},
		{"quick package no commission", func() error {
			return s.Cashier.Do("POST", "/api/v1/x/recharge/packages/quick", map[string]any{"carrierId": n.Carrier, "name": "Q", "price": "50"}, nil)
		}},
		{"edit no package", func() error {
			return s.Admin.Do("PUT", "/api/v1/x/recharge/packages/999999", map[string]any{"carrierId": n.Carrier, "name": "Q", "price": "50"}, nil)
		}},
		{"line no device", sale(svc(n.Product, "1", "100", tag("kind", "reload", "device", 999999)))},
		{"line unknown kind", sale(svc(n.Product, "1", "100", tag("kind", "magic", "device", n.Device)))},
		{"line no details", sale(svc(n.Product, "1", "100", map[string]any{"recharge": "x"}))},
		{"package no package", sale(svc(n.Product, "1", "100", tag("kind", "package", "device", n.Device, "package", 999999)))},
		{"count below 0", func() error {
			return s.Admin.Do("POST", "/api/v1/x/recharge/counts", map[string]any{"counts": []map[string]any{{"deviceId": n.Device, "counted": "-1"}}}, nil)
		}},
		{"count no device", func() error {
			return s.Admin.Do("POST", "/api/v1/x/recharge/counts", map[string]any{"counts": []map[string]any{{"deviceId": 999999, "counted": "1"}}}, nil)
		}},
		{"refill nothing", func() error {
			return s.Admin.Do("POST", "/api/v1/x/recharge/refill", map[string]any{"deviceId": n.Device, "supplierId": sup.ID, "amount": "0", "credit": true}, nil)
		}},
		{"refill no dealer", func() error {
			return s.Admin.Do("POST", "/api/v1/x/recharge/refill", map[string]any{"deviceId": n.Device, "amount": "100", "credit": true}, nil)
		}},
		{"refill no device", func() error {
			return s.Admin.Do("POST", "/api/v1/x/recharge/refill", map[string]any{"deviceId": 999999, "supplierId": sup.ID, "amount": "100", "credit": true}, nil)
		}},
		{"refill from nowhere", func() error {
			return s.Admin.Do("POST", "/api/v1/x/recharge/refill", map[string]any{"deviceId": n.Device, "supplierId": sup.ID, "amount": "100", "source": "bank"}, nil)
		}},
		{"cashier refills", func() error {
			return s.Cashier.Do("POST", "/api/v1/x/recharge/refill/counter", map[string]any{"deviceId": n.Device, "supplierId": sup.ID, "amount": "100",
				"source": fmt.Sprintf("till:%d", cashier)}, nil)
		}},
		{"cashier sets rates", func() error {
			return s.Cashier.Do("PUT", fmt.Sprintf("/api/v1/x/recharge/carriers/%d/rates", n.Carrier), []map[string]string{{"kind": "reload", "percent": "1"}}, nil)
		}},
		{"sale lines bad sale", func() error { return s.Cashier.Do("GET", "/api/v1/x/recharge/sale-lines?sale=abc", nil, nil) }},
	} {
		if err := x.do(); err == nil {
			t.Errorf("PLG-5 %s: accepted", x.name)
		} else if se, ok := err.(*ltest.StatusError); !ok || se.Code >= 500 {
			t.Errorf("PLG-5 %s: %v, want a clear 4xx", x.name, err)
		}
	}
	if ch := ltest.Changed(before, ltest.State(t, s.DB)); len(ch) > 0 {
		t.Errorf("PLG-5: refused requests changed %v", ch)
	}
}

// PLG-4: a failure at any statement leaves nothing behind.
func TestPLG4_Sweep(t *testing.T) { // compares every table: runs alone
	s := ltest.Shared(t)
	openTill(t, s.Shop)
	n := newNet(t, s.Shop)
	p := pkg(t, s.Shop, n.Carrier, "100", "90")
	var bank, sup struct{ ID int64 }
	s.Must(t, s.Admin.Do("POST", "/api/v1/admin/lockers", map[string]any{"name": fmt.Sprintf("Sweep bank %d", seq.Add(1)), "kind": "bank",
		"openingBalance": "100000", "cashierAccess": true}, &bank))
	s.Must(t, s.Admin.Do("POST", "/api/v1/admin/suppliers", map[string]string{"name": fmt.Sprintf("Sweep dealer %d", seq.Add(1)), "phone": fmt.Sprintf("011%07d", seq.Add(1))}, &sup))
	var cashier int64
	s.Must(t, s.DB.Get(&cashier, `SELECT id FROM users WHERE phone = '0770000002'`))
	soap := s.Product(t, fmt.Sprintf("Sweep soap %d", seq.Add(1)), "100", "1000", nil)
	k := 0
	s.Sweep(t, "new carrier", nil, func() error {
		k++
		return s.Admin.Do("POST", "/api/v1/x/recharge/carriers", map[string]string{"name": fmt.Sprintf("Sweep carrier %d-%d", seq.Add(1), k)}, nil)
	})
	s.Sweep(t, "new device with float", nil, func() error {
		return s.Admin.Do("POST", "/api/v1/x/recharge/devices", map[string]any{"carrierId": n.Carrier, "label": "Sweep", "reload": true, "opening": "500"}, nil)
	})
	post := func(items []map[string]any, payments ...map[string]any) error {
		if len(payments) == 0 {
			payments = []map[string]any{{"method": "cash", "amount": "100000"}}
		}
		return s.Cashier.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
			"items": items, "payments": payments}, nil)
	}
	s.Sweep(t, "a sale of every kind", nil, func() error {
		return post([]map[string]any{
			svc(n.Product, "1", "100", tag("kind", "reload", "device", n.Device)),
			svc(n.Product, "1", "100", tag("kind", "package", "device", n.Device, "package", p.ID)),
			svc(n.Own["billpay"], "1", "300", tag("kind", "billpay", "bank", bank.ID)),
			svc(n.Own["deposit"], "1", "200", tag("kind", "deposit", "device", n.Device)),
			{"productId": n.Own["cashout"], "quantity": "1", "priceOverride": "50", "payout": true, "plugin": tag("kind", "cashout", "device", n.Device)},
		})
	})
	var r sold
	s.Sweep(t, "a return", func() {
		var err error
		r, err = sell(t, s.Shop, []map[string]any{svc(n.Product, "1", "100", tag("kind", "reload", "device", n.Device)),
			svc(n.Own["billpay"], "1", "300", tag("kind", "billpay", "bank", bank.ID))})
		s.Must(t, err)
	}, func() error {
		return s.Cashier.Do("POST", fmt.Sprintf("/api/v1/till/sales/%d/return", r.ID), map[string]any{"lines": []map[string]any{
			{"saleItemId": r.Lines[0].ID, "quantity": "1"}, {"saleItemId": r.Lines[1].ID, "quantity": "1"}}}, nil)
	})
	s.Sweep(t, "wallet tender", nil, func() error {
		return post([]map[string]any{{"productId": soap, "quantity": "1"}},
			map[string]any{"method": "wallet", "amount": "100", "meta": map[string]string{"device": fmt.Sprint(n.Device)}})
	})
	s.Sweep(t, "count", nil, func() error {
		return s.Cashier.Do("POST", "/api/v1/x/recharge/counts/drawer", map[string]any{"counts": []map[string]any{{"deviceId": n.Device, "counted": "1234"}}}, nil)
	})
	s.Sweep(t, "refill on credit", nil, func() error {
		return s.Admin.Do("POST", "/api/v1/x/recharge/refill", map[string]any{"deviceId": n.Device, "supplierId": sup.ID, "amount": "1000", "bonus": "10", "credit": true}, nil)
	})
	s.Sweep(t, "refill paid now", nil, func() error {
		return s.Admin.Do("POST", "/api/v1/x/recharge/refill", map[string]any{"deviceId": n.Device, "supplierId": sup.ID, "amount": "100",
			"source": fmt.Sprintf("till:%d", cashier)}, nil)
	})
	s.Sweep(t, "quick package", nil, func() error {
		return s.Cashier.Do("POST", "/api/v1/x/recharge/packages/quick", map[string]any{"carrierId": n.Carrier, "name": fmt.Sprintf("Sweep pk %d", seq.Add(1)), "price": "50", "commission": "5"}, nil)
	})
}

func TestPLG7_DataGroup(t *testing.T) {
	ltest.Shared(t)
	ltest.DataGroupHoldsTables(t, "recharge")
}
