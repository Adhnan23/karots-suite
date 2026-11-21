// Package battle is the plugin's battle tests in a binary of their own: one
// shop per process (plugin hooks are process-wide).
package battle

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"karots-suite/internal/ledgercheck"
	"karots-suite/internal/ledgercheck/ltest"
	"karots-suite/plugins/recharge"

	"github.com/shopspring/decimal"
)

// Reload & Bills' battle tests (docs/rules/plugins/recharge.md): one throwaway
// shop on the real server with this plugin compiled in.

func TestMain(m *testing.M) { os.Exit(ltest.Main(m)) }

var seq atomic.Int64

var tillOnce sync.Once

func openTill(t *testing.T, s *ltest.Shop) { tillOnce.Do(func() { s.OpenTill(t, s.Cashier) }) }

// net is a carrier with one device (Rs 5,000 on it) and its rates: reload 3%,
// bill Rs 5 a bill, deposit 1%, cash out 0.5%, wallet payments 1%.
type net struct {
	Carrier, Product, Device int64
	Own                      map[string]int64
}

func newNet(t *testing.T, s *ltest.Shop) net {
	t.Helper()
	var n net
	var c, d struct{ ID int64 }
	s.Must(t, s.Admin.Do("POST", "/api/v1/x/recharge/carriers", map[string]string{"name": fmt.Sprintf("Carrier %d", seq.Add(1))}, &c))
	s.Must(t, s.Admin.Do("PUT", fmt.Sprintf("/api/v1/x/recharge/carriers/%d/rates", c.ID), []map[string]string{
		{"kind": "reload", "percent": "3"}, {"kind": "billpay", "fixed": "5"}, {"kind": "deposit", "percent": "1"},
		{"kind": "cashout", "percent": "0.5"}, {"kind": "wallet_in", "percent": "1"}}, nil))
	s.Must(t, s.Admin.Do("POST", "/api/v1/x/recharge/devices", map[string]any{"carrierId": c.ID, "label": "Shop phone", "reload": true,
		"money": true, "opening": "5000"}, &d))
	var td recharge.TillData
	s.Must(t, s.Cashier.Do("GET", "/api/v1/x/recharge/till", nil, &td))
	n.Carrier, n.Device, n.Own = c.ID, d.ID, td.Products
	for _, x := range td.Carriers {
		if x.ID == c.ID {
			n.Product = x.ProductID
		}
	}
	return n
}

func pkg(t *testing.T, s *ltest.Shop, carrier int64, price, floatCost string) recharge.Package {
	t.Helper()
	var p recharge.Package
	s.Must(t, s.Admin.Do("POST", "/api/v1/x/recharge/packages", map[string]any{"carrierId": carrier, "name": fmt.Sprintf("Data %d", seq.Add(1)),
		"price": price, "floatCost": floatCost}, &p))
	return p
}

func balance(t *testing.T, s *ltest.Shop, device int64) decimal.Decimal {
	t.Helper()
	var b decimal.Decimal
	s.Must(t, s.DB.Get(&b, `SELECT COALESCE(SUM(float_delta), 0) FROM recharge_transactions WHERE device_id = $1`, device))
	return b
}

func tag(kv ...any) map[string]any {
	m := map[string]any{}
	for i := 0; i < len(kv); i += 2 {
		m[kv[i].(string)] = kv[i+1]
	}
	return map[string]any{"recharge": m}
}

// svc is a service line (a reload, bill, deposit or package) of qty × price.
func svc(product int64, qty, price string, plugin map[string]any) map[string]any {
	return map[string]any{"productId": product, "quantity": qty, "priceOverride": price, "plugin": plugin}
}

type sold struct {
	ID    int64
	Lines []struct{ ID int64 }
}

func sell(t *testing.T, s *ltest.Shop, items []map[string]any, payments ...map[string]any) (sold, error) {
	t.Helper()
	if len(payments) == 0 {
		payments = []map[string]any{{"method": "cash", "amount": "100000"}}
	}
	var r sold
	if err := s.Cashier.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
		"items": items, "payments": payments}, &r); err != nil {
		return r, err
	}
	s.Must(t, s.Cashier.Do("GET", fmt.Sprintf("/api/v1/till/sales/%d", r.ID), nil, &r))
	return r, nil
}

func eq(t *testing.T, rule, what string, got decimal.Decimal, want string) {
	t.Helper()
	if !got.Equal(decimal.RequireFromString(want)) {
		t.Errorf("%s: %s %s, want %s", rule, what, got, want)
	}
}

func TestLedgerChecksFire(t *testing.T) {
	s := ltest.Shared(t).Shop
	openTill(t, s)
	n := newNet(t, s)
	r, err := sell(t, s, []map[string]any{svc(n.Product, "1", "100", tag("kind", "reload", "device", n.Device))})
	s.Must(t, err)
	var cnt []recharge.CountResult
	s.Must(t, s.Admin.Do("POST", "/api/v1/x/recharge/counts", map[string]any{"counts": []map[string]any{{"deviceId": n.Device, "counted": "9000"}}}, &cnt))
	s.Must(t, s.Cashier.Do("POST", fmt.Sprintf("/api/v1/till/sales/%d/return", r.ID), map[string]any{
		"lines": []map[string]any{{"saleItemId": r.Lines[0].ID, "quantity": "1"}}}, nil))
	dup := s.Product(t, fmt.Sprintf("Fire dup %d", seq.Add(1)), "10", "0", nil)
	move := fmt.Sprintf(`(SELECT id FROM recharge_transactions WHERE sale_item_id = %d AND parent_id IS NULL)`, r.Lines[0].ID)
	for _, x := range []struct{ rule, bad string }{
		{"RCH-3", fmt.Sprintf(`INSERT INTO recharge_transactions (carrier_id, device_id, type, amount, cash_delta, float_delta, sale_id, sale_item_id)
			VALUES (%d, %d, 'reload', 100, 0, -100, %d, %d)`, n.Carrier, n.Device, r.ID, r.Lines[0].ID)},
		{"RCH-3", fmt.Sprintf(`UPDATE recharge_transactions SET amount = 95, float_delta = -95 WHERE id = %s`, move)},
		{"RCH-3", fmt.Sprintf(`UPDATE recharge_transactions SET sale_item_id = NULL WHERE parent_id = %s AND type = 'commission' AND float_delta > 0`, move)},
		{"RCH-6", fmt.Sprintf(`UPDATE recharge_transactions SET float_delta = 150 WHERE parent_id = %s AND type = 'reversal'`, move)},
		{"RCH-6", fmt.Sprintf(`UPDATE recharge_transactions SET float_delta = -9 WHERE parent_id = %s AND type = 'commission' AND float_delta < 0`, move)},
		{"RCH-8", fmt.Sprintf(`UPDATE recharge_counts SET expected = 1 WHERE device_id = %d`, n.Device)},
		{"RCH-8", fmt.Sprintf(`UPDATE recharge_counts SET counted = expected + 1, expected = expected + 1 WHERE device_id = %d`, n.Device)},
		{"RCH-11", fmt.Sprintf(`UPDATE products SET merged_into_id = %d WHERE id = %d`, dup, n.Product)},
	} {
		tx := s.DB.MustBegin()
		tx.MustExec(x.bad)
		vs, err := ledgercheck.Run(context.Background(), tx)
		tx.Rollback() //nolint:errcheck
		if err != nil {
			t.Fatal(err)
		}
		fired := false
		for _, v := range vs {
			fired = fired || v.Rule == x.rule
		}
		if !fired {
			t.Errorf("%s did not fire on: %s", x.rule, x.bad)
		}
	}
}

// RCH-3, RCH-4, RCH-9: every kind of line books its float and commission in the
// sale; a low balance never refuses.
func TestRCH3_EveryKindBooks(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	openTill(t, s)
	n := newNet(t, s)
	p := pkg(t, s, n.Carrier, "100", "88")
	_, err := sell(t, s, []map[string]any{
		svc(n.Product, "1", "100", tag("kind", "reload", "device", n.Device, "number", "0771234567")),
		svc(n.Product, "1", "100", tag("kind", "package", "device", n.Device, "package", p.ID)),
		svc(n.Own["billpay"], "1", "500", tag("kind", "billpay", "device", n.Device, "billType", "CEB", "ref", "12345")),
		svc(n.Own["deposit"], "1", "1000", tag("kind", "deposit", "device", n.Device)),
		svc(n.Own["charge"], "1", "30", tag("kind", "charge")),
		{"productId": n.Own["cashout"], "quantity": "1", "priceOverride": "200", "payout": true, "plugin": tag("kind", "cashout", "device", n.Device)},
	})
	s.Must(t, err)
	// 5000 − 100 + 3 − 88 + 12 − 500 + 5 − 1000 + 10 + 200 + 1
	eq(t, "RCH-3", "float", balance(t, s, n.Device), "3543")
	if _, err := sell(t, s, []map[string]any{svc(n.Product, "1", "9000", tag("kind", "reload", "device", n.Device))}); err != nil {
		t.Errorf("RCH-3: a reload past the float was refused: %v", err)
	}
	eq(t, "RCH-4", "float", balance(t, s, n.Device), "-5187")
	s.Audit(t)
}

// RCH-3 (user, 2026-11-21): a discounted reload still moves the float by its face
// value; a package of two uses twice its float.
func TestRCH3_FaceValue(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	openTill(t, s)
	n := newNet(t, s)
	p := pkg(t, s, n.Carrier, "100", "88")
	l := svc(n.Product, "1", "100", tag("kind", "reload", "device", n.Device))
	l["discount"], l["discountType"] = "5", "fixed"
	_, err := sell(t, s, []map[string]any{l, svc(n.Product, "2", "100", tag("kind", "package", "device", n.Device, "package", p.ID))})
	s.Must(t, err)
	// 5000 − 100 + 3 − 176 + 24
	eq(t, "RCH-3", "float", balance(t, s, n.Device), "4751")
	s.Audit(t)
}

// RCH-5: a bill paid from a bank moves the bank by its face value — also once the
// bank (or the device) is turned off for cashiers since (user, 2026-11-21).
func TestRCH5_BankLeg(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	openTill(t, s)
	n := newNet(t, s)
	var bank struct{ ID int64 }
	s.Must(t, s.Admin.Do("POST", "/api/v1/admin/lockers", map[string]any{"name": fmt.Sprintf("Bank %d", seq.Add(1)), "kind": "bank",
		"openingBalance": "10000", "cashierAccess": true}, &bank))
	bill := func() error {
		_, err := sell(t, s, []map[string]any{svc(n.Own["billpay"], "1", "300", tag("kind", "billpay", "bank", bank.ID, "ref", "A1"))})
		return err
	}
	s.Must(t, bill())
	s.Must(t, s.DB.Get(new(int), `UPDATE lockers SET cashier_access = false WHERE id = $1 RETURNING 1`, bank.ID))
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/recharge/devices/%d/active", n.Device), map[string]bool{"on": false}, nil))
	if err := bill(); err != nil {
		t.Errorf("RCH-5: a bill on a bank turned off since was refused: %v", err)
	}
	if _, err := sell(t, s, []map[string]any{svc(n.Product, "1", "50", tag("kind", "reload", "device", n.Device))}); err != nil {
		t.Errorf("RCH-5: a reload on a device turned off since was refused: %v", err)
	}
	if _, err := sell(t, s, []map[string]any{svc(n.Own["billpay"], "1", "300", tag("kind", "billpay", "bank", 999999))}); err == nil {
		t.Error("RCH-3: a bill on a bank that doesn't exist was booked")
	}
	var bal decimal.Decimal
	s.Must(t, s.DB.Get(&bal, `SELECT COALESCE(SUM(balance_delta), 0) FROM locker_ledger WHERE locker_id = $1`, bank.ID))
	eq(t, "RCH-5", "bank", bal, "9400")
	s.Audit(t)
}

// RCH-6: a line returned in parts gives back exactly what left and takes back
// all its commission; a wrong number gives none back and keeps the commission.
func TestRCH6_ReturnInParts(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	openTill(t, s)
	n := newNet(t, s)
	r, err := sell(t, s, []map[string]any{
		svc(n.Product, "3", "1.11", tag("kind", "reload", "device", n.Device)),
		svc(n.Product, "1", "200", tag("kind", "reload", "device", n.Device)),
	})
	s.Must(t, err)
	for range 3 {
		s.Must(t, s.Cashier.Do("POST", fmt.Sprintf("/api/v1/till/sales/%d/return", r.ID), map[string]any{
			"lines": []map[string]any{{"saleItemId": r.Lines[0].ID, "quantity": "1"}}}, nil))
	}
	s.Must(t, s.Cashier.Do("POST", fmt.Sprintf("/api/v1/till/sales/%d/return", r.ID), map[string]any{
		"lines": []map[string]any{{"saleItemId": r.Lines[1].ID, "quantity": "1", "plugin": map[string]any{"recharge": map[string]string{"back": "none"}}}}}, nil))
	// 5000 − 3.33 + 0.10 (all back) − 200 + 6 (gone, commission kept)
	eq(t, "RCH-6", "float", balance(t, s, n.Device), "4806")
	s.Audit(t)
}

// RCH-7: a sale paid by wallet puts the money on the device with its commission;
// no device is refused.
func TestRCH7_WalletTender(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	openTill(t, s)
	n := newNet(t, s)
	soap := s.Product(t, fmt.Sprintf("Soap %d", seq.Add(1)), "500", "10", nil)
	_, err := sell(t, s, []map[string]any{{"productId": soap, "quantity": "1"}},
		map[string]any{"method": "wallet", "amount": "500", "meta": map[string]string{"device": fmt.Sprint(n.Device), "ref": "TX1"}})
	s.Must(t, err)
	eq(t, "RCH-7", "float", balance(t, s, n.Device), "5505")
	if _, err := sell(t, s, []map[string]any{{"productId": soap, "quantity": "1"}}, map[string]any{"method": "wallet", "amount": "500"}); err == nil {
		t.Error("RCH-7: a wallet payment with no device was taken")
	}
	s.Audit(t)
}

// RCH-8: a count and a sale on the same device at once — the count's expected is
// exactly the float booked before it.
func TestRCH8_CountAtOnceWithSale(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	openTill(t, s)
	n := newNet(t, s)
	for range 15 {
		var wg sync.WaitGroup
		wg.Go(func() {
			_, _ = sell(t, s, []map[string]any{svc(n.Product, "1", "10", tag("kind", "reload", "device", n.Device))})
		})
		wg.Go(func() {
			_ = s.Cashier.Do("POST", "/api/v1/x/recharge/counts/drawer", map[string]any{"counts": []map[string]any{{"deviceId": n.Device, "counted": "99999"}}}, nil)
		})
		wg.Wait()
	}
	s.Audit(t)
}

// RCH-10: float bought on credit is a supplier bill; paid now, the money leaves
// the drawer; the float is up by the amount plus the bonus.
func TestRCH10_Refill(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	openTill(t, s)
	n := newNet(t, s)
	var sup struct{ ID int64 }
	s.Must(t, s.Admin.Do("POST", "/api/v1/admin/suppliers", map[string]string{"name": fmt.Sprintf("Dealer %d", seq.Add(1)), "phone": fmt.Sprintf("011%07d", seq.Add(1))}, &sup))
	var cashier int64
	s.Must(t, s.DB.Get(&cashier, `SELECT id FROM users WHERE phone = '0770000002'`))
	s.Must(t, s.Admin.Do("POST", "/api/v1/x/recharge/refill", map[string]any{"deviceId": n.Device, "supplierId": sup.ID, "amount": "2000",
		"bonus": "20", "credit": true, "reference": "INV-1"}, nil))
	s.Must(t, s.Admin.Do("POST", "/api/v1/x/recharge/refill", map[string]any{"deviceId": n.Device, "supplierId": sup.ID, "amount": "1000",
		"source": fmt.Sprintf("till:%d", cashier)}, nil))
	eq(t, "RCH-10", "float", balance(t, s, n.Device), "8020")
	var owed decimal.Decimal
	s.Must(t, s.DB.Get(&owed, `SELECT COALESCE(SUM(total - paid_amount), 0) FROM purchases WHERE supplier_id = $1`, sup.ID))
	eq(t, "RCH-10", "owed to the dealer", owed, "2000")
	s.Audit(t)
}

// RCH-2, PLG-6: the same package added at the till six times at once is one package.
func TestRCH2_QuickPackageAtOnce(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	n := newNet(t, s)
	name := fmt.Sprintf("Rush %d", seq.Add(1))
	var bad atomic.Int64
	var wg sync.WaitGroup
	for range 6 {
		wg.Go(func() {
			if s.Cashier.Do("POST", "/api/v1/x/recharge/packages/quick", map[string]any{"carrierId": n.Carrier, "name": name, "price": "99", "commission": "9"}, nil) != nil {
				bad.Add(1)
			}
		})
	}
	wg.Wait()
	var count int
	s.Must(t, s.DB.Get(&count, `SELECT count(*) FROM recharge_packages WHERE carrier_id = $1 AND name = $2`, n.Carrier, name))
	if bad.Load() != 0 || count != 1 {
		t.Errorf("RCH-2: %d of 6 failed, %d packages; want none failed and one", bad.Load(), count)
	}
}

// RCH-11: a merge moves a carrier's product to the kept one.
func TestRCH11_ProductMerge(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	n := newNet(t, s)
	kept := s.Service(t, fmt.Sprintf("Kept reload %d", seq.Add(1)), "0")
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/admin/products/%d/merge/%d", n.Product, kept), map[string]string{"lotPrice": "follow"}, nil))
	var pid int64
	s.Must(t, s.DB.Get(&pid, `SELECT product_id FROM recharge_carriers WHERE id = $1`, n.Carrier))
	if pid != kept {
		t.Errorf("RCH-11: carrier on product %d, want %d", pid, kept)
	}
	s.Audit(t)
}

// RCH-9: earnings are commission and count differences (and a dealer's bonus);
// face value and the service charge never count. Runs alone: earnings are shop-wide.
func TestRCH9_Earnings(t *testing.T) {
	s := ltest.Shared(t).Shop
	openTill(t, s)
	n := newNet(t, s)
	var sup struct{ ID int64 }
	s.Must(t, s.Admin.Do("POST", "/api/v1/admin/suppliers", map[string]string{"name": fmt.Sprintf("Earn dealer %d", seq.Add(1)), "phone": fmt.Sprintf("011%07d", seq.Add(1))}, &sup))
	today := func() decimal.Decimal {
		var f recharge.FloatPage
		s.Must(t, s.Admin.Do("GET", "/api/v1/x/recharge/float", nil, &f))
		return f.Today
	}
	before := today()
	_, err := sell(t, s, []map[string]any{svc(n.Product, "1", "100", tag("kind", "reload", "device", n.Device)),
		svc(n.Own["charge"], "1", "30", tag("kind", "charge"))})
	s.Must(t, err)
	s.Must(t, s.Admin.Do("POST", "/api/v1/x/recharge/refill", map[string]any{"deviceId": n.Device, "supplierId": sup.ID, "amount": "1000", "bonus": "20", "credit": true}, nil))
	// float now 5000 − 100 + 3 + 1000 + 20 = 5923; the phone shows 10 less
	s.Must(t, s.Admin.Do("POST", "/api/v1/x/recharge/counts", map[string]any{"counts": []map[string]any{{"deviceId": n.Device, "counted": "5913"}}}, nil))
	eq(t, "RCH-9", "earnings today up by", today().Sub(before), "13")
	s.Audit(t)
}
