package battle

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"karots-suite/internal/ledgercheck/ltest"
	"karots-suite/internal/plugin"
	"karots-suite/plugins/recharge"

	"github.com/shopspring/decimal"
)

// RCH-4: the balance the Float page, the till, money places and readiness show is
// the device's float rows.
func TestRCH4_BalanceEverywhere(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	openTill(t, s)
	n := newNet(t, s)
	s.Must(t, s.Admin.Do("PUT", fmt.Sprintf("/api/v1/x/recharge/devices/%d", n.Device), map[string]any{"carrierId": n.Carrier, "label": "Shop phone",
		"reload": true, "money": true, "target": "8000"}, nil))
	_, err := sell(t, s, []map[string]any{svc(n.Product, "1", "1000", tag("kind", "reload", "device", n.Device))})
	s.Must(t, err)
	want := balance(t, s, n.Device) // 5000 − 1000 + 30
	eq(t, "RCH-4", "float", want, "4030")
	var name string
	s.Must(t, s.DB.Get(&name, `SELECT c.name || ' — ' || d.label FROM recharge_devices d JOIN recharge_carriers c ON c.id = d.carrier_id WHERE d.id = $1`, n.Device))
	var page recharge.FloatPage
	s.Must(t, s.Admin.Do("GET", "/api/v1/x/recharge/float", nil, &page))
	var td recharge.TillData
	s.Must(t, s.Cashier.Do("GET", "/api/v1/x/recharge/till", nil, &td))
	for what, fs := range map[string][]recharge.Float{"float page": page.Floats, "till": td.Floats} {
		found := false
		for _, f := range fs {
			if f.ID == n.Device {
				found = true
				eq(t, "RCH-4", what, f.Balance, want.String())
			}
		}
		if !found {
			t.Errorf("RCH-4: the %s doesn't show the device", what)
		}
	}
	ctx := context.Background()
	place := false
	for _, g := range plugin.CollectMoneyLocations(ctx) {
		for _, it := range g.Items {
			if it.Name == name {
				place = true
				eq(t, "RCH-4", "money place", it.Balance, want.String())
			}
		}
	}
	ready := false
	for _, sec := range plugin.CollectReadiness(ctx) {
		for _, r := range sec.Rows {
			if r.Name == name {
				ready = true
				eq(t, "RCH-4", "readiness need", r.Need, "3970")
			}
		}
	}
	if !place || !ready {
		t.Errorf("RCH-4: in money places %v, in readiness %v; want both", place, ready)
	}
}

// RCH-10: a cashier who deals with suppliers buys float from their own drawer or
// a cashier locker — never from another's drawer or a locker closed to them.
func TestRCH10_CounterRefill(t *testing.T) { // flips the cashier's supplier flag: runs alone
	s := ltest.Shared(t).Shop
	openTill(t, s)
	n := newNet(t, s)
	var cashier int64
	s.Must(t, s.DB.Get(&cashier, `UPDATE users SET can_handle_suppliers = true WHERE phone = '0770000002' RETURNING id`))
	defer s.DB.Exec(`UPDATE users SET can_handle_suppliers = false WHERE id = $1`, cashier) //nolint:errcheck
	var sup, open, shut struct{ ID int64 }
	s.Must(t, s.Admin.Do("POST", "/api/v1/admin/suppliers", map[string]string{"name": fmt.Sprintf("Counter dealer %d", seq.Add(1)), "phone": fmt.Sprintf("011%07d", seq.Add(1))}, &sup))
	s.Must(t, s.Admin.Do("POST", "/api/v1/admin/lockers", map[string]any{"name": fmt.Sprintf("Cashier safe %d", seq.Add(1)), "kind": "safe", "openingBalance": "5000", "cashierAccess": true}, &open))
	s.Must(t, s.Admin.Do("POST", "/api/v1/admin/lockers", map[string]any{"name": fmt.Sprintf("Owner safe %d", seq.Add(1)), "kind": "safe", "openingBalance": "5000"}, &shut))
	var srcs []recharge.Source
	s.Must(t, s.Cashier.Do("GET", "/api/v1/x/recharge/refill/sources", nil, &srcs))
	var got []string
	for _, x := range srcs {
		got = append(got, x.Value)
	}
	if l := strings.Join(got, " "); !strings.Contains(l, fmt.Sprintf("till:%d", cashier)) || !strings.Contains(l, fmt.Sprintf("locker:%d", open.ID)) ||
		strings.Contains(l, fmt.Sprintf("locker:%d", shut.ID)) {
		t.Errorf("RCH-10: sources %s; want my drawer and the cashier safe, not the owner's", l)
	}
	refill := func(src string) error {
		return s.Cashier.Do("POST", "/api/v1/x/recharge/refill/counter", map[string]any{"deviceId": n.Device, "supplierId": sup.ID, "amount": "500", "source": src}, nil)
	}
	s.Must(t, refill(fmt.Sprintf("till:%d", cashier)))
	s.Must(t, refill(fmt.Sprintf("locker:%d", open.ID)))
	s.Must(t, s.Cashier.Do("POST", "/api/v1/x/recharge/refill/counter", map[string]any{"deviceId": n.Device, "supplierId": sup.ID, "amount": "300", "credit": true}, nil))
	for _, bad := range []string{fmt.Sprintf("locker:%d", shut.ID), "till:1"} {
		if err := refill(bad); err == nil {
			t.Errorf("RCH-10: a cashier bought float from %s", bad)
		}
	}
	eq(t, "RCH-10", "float", balance(t, s, n.Device), "6300")
	s.Audit(t)
}

// RCH-1, RCH-2: a carrier turned off and on again keeps its name only while no
// other enabled carrier took it; a package saved in admin (or reviewed) is no
// longer flagged; a package's name is unique per carrier.
func TestRCH1_CarrierAndPackages(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	a := newNet(t, s)
	var name string
	s.Must(t, s.DB.Get(&name, `SELECT name FROM recharge_carriers WHERE id = $1`, a.Carrier))
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/recharge/carriers/%d/active", a.Carrier), map[string]bool{"on": false}, nil))
	var twin struct{ ID int64 }
	s.Must(t, s.Admin.Do("POST", "/api/v1/x/recharge/carriers", map[string]string{"name": strings.ToUpper(name)}, &twin))
	if err := s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/recharge/carriers/%d/active", a.Carrier), map[string]bool{"on": true}, nil); err == nil {
		t.Error("RCH-1: turned a carrier back on under a name another enabled carrier has")
	}
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/recharge/carriers/%d/active", twin.ID), map[string]bool{"on": false}, nil))
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/recharge/carriers/%d/active", a.Carrier), map[string]bool{"on": true}, nil))
	var setup struct {
		Devices []recharge.Device `json:"devices"`
	}
	s.Must(t, s.Admin.Do("GET", "/api/v1/x/recharge/setup", nil, &setup))
	listed := false
	for _, d := range setup.Devices {
		listed = listed || d.ID == a.Device
	}
	if !listed {
		t.Error("RCH-1: setup doesn't list the device")
	}
	var quick struct{ Package recharge.Package }
	s.Must(t, s.Cashier.Do("POST", "/api/v1/x/recharge/packages/quick", map[string]any{"carrierId": a.Carrier, "name": "Night 1GB", "price": "99", "commission": "9"}, &quick))
	other := pkg(t, s, a.Carrier, "50", "45")
	if quick.Package.AddedBy == "" || !quick.Package.FloatCost.Equal(decimal.NewFromInt(90)) {
		t.Errorf("RCH-2: a till package %+v; want flagged with its cashier, float 90", quick.Package)
	}
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/recharge/packages/%d/review", quick.Package.ID), nil, nil))
	if err := s.Admin.Do("PUT", fmt.Sprintf("/api/v1/x/recharge/packages/%d", other.ID), map[string]any{"carrierId": a.Carrier, "name": "night 1gb", "price": "50", "floatCost": "45"}, nil); err == nil {
		t.Error("RCH-2: renamed a package onto another's name")
	}
	s.Must(t, s.Admin.Do("PUT", fmt.Sprintf("/api/v1/x/recharge/packages/%d", other.ID), map[string]any{"carrierId": a.Carrier, "name": "Day 500MB", "price": "60", "commission": "6"}, nil))
	var flagged int
	s.Must(t, s.DB.Get(&flagged, `SELECT count(*) FROM recharge_packages WHERE carrier_id = $1 AND reviewed_at IS NULL`, a.Carrier))
	var fc decimal.Decimal
	s.Must(t, s.DB.Get(&fc, `SELECT float_cost FROM recharge_packages WHERE id = $1`, other.ID))
	if flagged != 0 || !fc.Equal(decimal.NewFromInt(54)) {
		t.Errorf("RCH-2: %d still flagged, edited float %s; want none and 54", flagged, fc)
	}
}

// RCH-6: the Returns field lists a sale's recharge lines with what left; an
// amount given back is booked, never more than left.
func TestRCH6_AmountBack(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	openTill(t, s)
	n := newNet(t, s)
	r, err := sell(t, s, []map[string]any{svc(n.Product, "1", "200", tag("kind", "reload", "device", n.Device)),
		svc(n.Own["deposit"], "1", "100", tag("kind", "deposit", "device", n.Device))})
	s.Must(t, err)
	var lines []recharge.SaleLine
	s.Must(t, s.Cashier.Do("GET", fmt.Sprintf("/api/v1/x/recharge/sale-lines?sale=%d", r.ID), nil, &lines))
	if len(lines) != 2 || !lines[0].Left.Equal(decimal.NewFromInt(200)) {
		t.Fatalf("RCH-6: sale lines %+v; want the reload (200) and the deposit", lines)
	}
	ret := func(item int64, back string) {
		s.Must(t, s.Cashier.Do("POST", fmt.Sprintf("/api/v1/till/sales/%d/return", r.ID), map[string]any{"lines": []map[string]any{
			{"saleItemId": item, "quantity": "1", "plugin": map[string]any{"recharge": map[string]string{"back": back}}}}}, nil))
	}
	ret(r.Lines[0].ID, "150")  // 150 of 200 back: commission 6 → 4.50 taken back
	ret(r.Lines[1].ID, "9999") // more than left: all 100 back, commission 1 taken back
	// 5000 − 200 + 6 − 100 + 1 + 150 − 4.50 + 100 − 1
	eq(t, "RCH-6", "float", balance(t, s, n.Device), "4951.50")
	s.Audit(t)
}

// Activity lists float moves and bank bills by who did them.
func TestRCH_Activity(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	openTill(t, s)
	n := newNet(t, s)
	num := fmt.Sprintf("07%08d", seq.Add(1))
	_, err := sell(t, s, []map[string]any{svc(n.Product, "1", "75", tag("kind", "reload", "device", n.Device, "number", num))})
	s.Must(t, err)
	var act struct {
		Rows []struct {
			Source, Action, Detail string
		} `json:"rows"`
	}
	s.Must(t, s.Admin.Do("GET", "/api/v1/admin/activity?q="+num, nil, &act))
	found := false
	for _, r := range act.Rows {
		found = found || (r.Source == "recharge" && r.Action == "float reload" && strings.Contains(r.Detail, num))
	}
	if !found {
		t.Errorf("activity %+v; want the reload to %s", act.Rows, num)
	}
}
